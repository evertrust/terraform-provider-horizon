package provider

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/evertrust/horizon-go/v2/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	sanTypeDNS   = "DNSNAME"
	sanTypeIP    = "IPADDRESS"
	sanTypeEmail = "RFC822NAME"
	sanTypeURI   = "URI"
)

// dnElementByOID maps the DN attribute OIDs Horizon knows to its element names.
var dnElementByOID = map[string]string{
	"2.5.4.3":                    "cn",
	"1.2.840.113549.1.9.1":       "e",
	"2.5.4.11":                   "ou",
	"2.5.4.8":                    "st",
	"2.5.4.7":                    "l",
	"2.5.4.10":                   "o",
	"2.5.4.6":                    "c",
	"0.9.2342.19200300.100.1.25": "dc",
	"0.9.2342.19200300.100.1.1":  "uid",
	"2.5.4.5":                    "serialNumber",
	"2.5.4.4":                    "surname",
	"2.5.4.42":                   "givenName",
	"1.2.840.113549.1.9.8":       "unstructuredAddress",
	"1.2.840.113549.1.9.2":       "unstructuredName",
	"2.5.4.97":                   "organizationIdentifier",
	"2.5.4.45":                   "uniqueIdentifier",
	"2.5.4.9":                    "street",
	"2.5.4.13":                   "description",
	"2.5.4.12":                   "t",
}

func usesChallenge(data certificateResourceModel) bool {
	return !data.Challenge.IsNull() || data.RequestChallenge.ValueBool()
}

func validateChallengeConfig(data certificateResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if !data.Challenge.IsNull() && data.RequestChallenge.ValueBool() {
		diags.AddAttributeError(
			path.Root("request_challenge"),
			"request_challenge conflicts with challenge",
			"Set challenge when a challenge was issued for you, or set request_challenge to let the provider request one.",
		)
	}

	if !usesChallenge(data) {
		return diags
	}

	if !data.Password.IsNull() {
		diags.AddAttributeError(
			path.Root("password"),
			"password cannot be set when enrolling with a challenge",
			"Horizon encrypts the PKCS#12 of a challenge enrollment with the challenge. The provider exposes the challenge as the password attribute.",
		)
	}

	if data.Csr.IsNull() && data.Pkcs12WriteOnly.ValueBool() {
		diags.AddAttributeError(
			path.Root("pkcs12_write_only"),
			"pkcs12_write_only would lose the private key of a challenge enrollment",
			"Horizon returns the PKCS#12 of a centralized challenge enrollment once and does not store the private key, so the key cannot be retrieved later.",
		)
	}

	if !data.Challenge.IsNull() {
		const detail = "Ownership and labels of a challenge enrollment come from the challenge request."
		if !data.Owner.IsNull() {
			diags.AddAttributeWarning(path.Root("owner"), "owner is not applied at enrollment when challenge is provided.", detail)
		}
		if !data.Team.IsNull() {
			diags.AddAttributeWarning(path.Root("team"), "team is not applied at enrollment when challenge is provided.", detail)
		}
		if !data.ContactEmail.IsNull() {
			diags.AddAttributeWarning(path.Root("contact_email"), "contact_email is not applied at enrollment when challenge is provided.", detail)
		}
		if len(data.Labels.Elements()) > 0 {
			diags.AddAttributeWarning(path.Root("labels"), "labels are not applied at enrollment when challenge is provided.", detail)
		}
	}

	return diags
}

func subjectElements(ctx context.Context, data certificateResourceModel) ([]models.IndexedDNElement, diag.Diagnostics) {
	subject := make([]certificateSubjectModel, 0, len(data.Subject.Elements()))
	diags := data.Subject.ElementsAs(ctx, &subject, false)
	elements := make([]models.IndexedDNElement, 0, len(subject))
	for _, dnElement := range subject {
		el := models.IndexedDNElement{Element: dnElement.Element.ValueString()}
		el.SetValue(dnElement.Value.ValueString())
		elements = append(elements, el)
	}
	return elements, diags
}

func sanElements(ctx context.Context, data certificateResourceModel) ([]models.ListSANElement, diag.Diagnostics) {
	sans := make([]certificateSanModel, 0, len(data.Sans.Elements()))
	diags := data.Sans.ElementsAs(ctx, &sans, false)
	elements := make([]models.ListSANElement, 0, len(sans))
	for _, sanElement := range sans {
		values := make([]string, 0, len(sanElement.Value))
		for _, value := range sanElement.Value {
			values = append(values, value.ValueString())
		}
		el := models.ListSANElement{Value: values}
		el.SetType(sanElement.Type.ValueString())
		elements = append(elements, el)
	}
	return elements, diags
}

func applyOwnership(ctx context.Context, template *models.WebRAEnrollRequestTemplate, data certificateResourceModel) diag.Diagnostics {
	labels := make([]certificateLabelModel, 0, len(data.Labels.Elements()))
	diags := data.Labels.ElementsAs(ctx, &labels, false)
	labelElements := make([]models.RequestLabelElement, 0, len(labels))
	for _, label := range labels {
		el := models.RequestLabelElement{Label: label.Label.ValueString()}
		el.SetValue(label.Value.ValueString())
		labelElements = append(labelElements, el)
	}
	template.SetLabels(labelElements)

	if !data.Owner.IsNull() {
		owner := models.NewCertificateOwnerElementWithDefaults()
		owner.SetValue(data.Owner.ValueString())
		template.SetOwner(*owner)
	}

	if !data.Team.IsNull() {
		team := models.NewCertificateTeamElementWithDefaults()
		team.SetValue(data.Team.ValueString())
		template.SetTeam(*team)
	}

	if !data.ContactEmail.IsNull() {
		contact := models.NewCertificateContactEmailElementWithDefaults()
		contact.SetValue(data.ContactEmail.ValueString())
		template.SetContactEmail(*contact)
	}
	return diags
}

func identityFromCSR(csrPEM string) ([]models.IndexedDNElement, []models.ListSANElement, diag.Diagnostics) {
	var diags diag.Diagnostics

	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil {
		diags.AddAttributeError(path.Root("csr"), "Invalid CSR", "csr is not PEM encoded.")
		return nil, nil, diags
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		diags.AddAttributeError(path.Root("csr"), "Invalid CSR", err.Error())
		return nil, nil, diags
	}

	var subject []models.IndexedDNElement
	var skipped []string
	indexes := map[string]int{}
	for _, name := range csr.Subject.Names {
		element, known := dnElementByOID[name.Type.String()]
		value, isString := name.Value.(string)
		if !known || !isString {
			skipped = append(skipped, name.Type.String())
			continue
		}
		indexes[element]++
		el := models.IndexedDNElement{Element: fmt.Sprintf("%s.%d", element, indexes[element])}
		el.SetValue(value)
		subject = append(subject, el)
	}
	if len(skipped) > 0 {
		diags.AddAttributeWarning(
			path.Root("csr"),
			"Some CSR subject attributes were not sent to Horizon",
			fmt.Sprintf("Horizon has no DN element for the attribute(s) %s. Set subject explicitly to control the certificate DN.", strings.Join(skipped, ", ")),
		)
	}

	var sans []models.ListSANElement
	addSan := func(sanType string, values []string) {
		if len(values) == 0 {
			return
		}
		el := models.ListSANElement{Value: values}
		el.SetType(sanType)
		sans = append(sans, el)
	}
	addSan(sanTypeDNS, csr.DNSNames)
	ips := make([]string, 0, len(csr.IPAddresses))
	for _, ip := range csr.IPAddresses {
		ips = append(ips, ip.String())
	}
	addSan(sanTypeIP, ips)
	addSan(sanTypeEmail, csr.EmailAddresses)
	uris := make([]string, 0, len(csr.URIs))
	for _, uri := range csr.URIs {
		uris = append(uris, uri.String())
	}
	addSan(sanTypeURI, uris)

	return subject, sans, diags
}

func challengeIdentity(ctx context.Context, data certificateResourceModel) ([]models.IndexedDNElement, []models.ListSANElement, diag.Diagnostics) {
	subject, diags := subjectElements(ctx, data)
	sans, sanDiags := sanElements(ctx, data)
	diags.Append(sanDiags...)

	if len(subject) == 0 && len(sans) == 0 && !data.Csr.IsNull() {
		csrSubject, csrSans, csrDiags := identityFromCSR(data.Csr.ValueString())
		diags.Append(csrDiags...)
		return csrSubject, csrSans, diags
	}
	return subject, sans, diags
}

func challengeSubmitTemplate(data certificateResourceModel, subject []models.IndexedDNElement, sans []models.ListSANElement) *models.WebRAChallengeSubmitRequestTemplate {
	template := models.NewWebRAChallengeSubmitRequestTemplate()
	if !data.Csr.IsNull() {
		csr := data.Csr.ValueString()
		template.Csr = &csr
	} else if !data.KeyType.IsNull() && !data.KeyType.IsUnknown() && data.KeyType.ValueString() != "" {
		keyType := data.KeyType.ValueString()
		template.KeyType = &keyType
	}
	if len(subject) > 0 {
		template.Subject = subject
	}
	if len(sans) > 0 {
		template.Sans = sans
	}
	return template
}

func templateDefinesIdentity(template *models.WebRAEnrollRequestOnTemplateResponse) bool {
	if template == nil {
		return false
	}
	t := template.Template
	return len(t.Subject) > 0 || len(t.Sans) > 0 || len(t.Extensions) > 0
}

type issuedChallenge struct {
	requestID string
	challenge string
}

func extractIssuedChallenge(resp *models.RequestSubmit201Response) (*issuedChallenge, diag.Diagnostics) {
	var diags diag.Diagnostics
	if resp == nil || resp.WebRAEnrollRequestOnSubmitResponse == nil {
		diags.AddError("Unexpected response type", "Expected WebRAEnrollRequestOnSubmitResponse")
		return nil, diags
	}
	enrollResp := resp.WebRAEnrollRequestOnSubmitResponse

	if enrollResp.Certificate.Get() != nil {
		diags.AddAttributeError(
			path.Root("request_challenge"),
			"Profile is not in Challenge mode",
			fmt.Sprintf("Horizon enrolled certificate %s without issuing a challenge. Terraform does not manage that certificate. Remove request_challenge to enroll on this profile.", enrollResp.Certificate.Get().Id),
		)
		return nil, diags
	}

	if enrollResp.Status != models.REQUESTSTATUS_APPROVED {
		diags.AddError(
			"Challenge request was not approved",
			fmt.Sprintf("Request %s is %s. request_challenge needs the permission to approve enrollments on the profile. Without it, have the request approved and pass the resulting challenge through the challenge attribute.", enrollResp.Id, enrollResp.Status),
		)
		return nil, diags
	}

	password := enrollResp.Password.Get()
	if password == nil || password.GetValue() == "" {
		diags.AddError(
			"No challenge in the approved request",
			fmt.Sprintf("Request %s was approved but carries no challenge. The profile is probably not in Challenge mode.", enrollResp.Id),
		)
		return nil, diags
	}

	return &issuedChallenge{requestID: enrollResp.Id, challenge: password.GetValue()}, diags
}

// requestChallenge issues a challenge with the provider credentials.
func (r *CertificateResource) requestChallenge(ctx context.Context, data certificateResourceModel, subject []models.IndexedDNElement, sans []models.ListSANElement) (*issuedChallenge, diag.Diagnostics) {
	var diags diag.Diagnostics

	onTemplate := models.NewWebRAEnrollRequestOnTemplate(webRAModule, workflowEnroll)
	onTemplate.SetProfile(data.Profile.ValueString())
	tmplResp, _, err := r.client.RequestAPI.RequestTemplate(ctx).
		RequestTemplateRequest(models.WebRAEnrollRequestOnTemplateAsRequestTemplateRequest(onTemplate)).
		Execute()
	if err != nil {
		diags.AddError("Failed to get enroll template", err.Error())
		return nil, diags
	}

	template := models.NewWebRAEnrollRequestTemplateWithDefaults()
	if templateDefinesIdentity(tmplResp.WebRAEnrollRequestOnTemplateResponse) {
		template.SetSubject(subject)
		template.SetSans(sans)
	}
	diags.Append(applyOwnership(ctx, template, data)...)
	if diags.HasError() {
		return nil, diags
	}

	submit := models.NewWebRAEnrollRequestOnSubmit(data.Profile.ValueString(), webRAModule, *template, workflowEnroll)
	submitResp, _, err := r.client.RequestAPI.RequestSubmit(ctx).
		RequestSubmitRequest(models.WebRAEnrollRequestOnSubmitAsRequestSubmitRequest(submit)).
		Execute()
	if err != nil {
		diags.AddError("Failed to request a challenge", err.Error())
		return nil, diags
	}

	issued, issueDiags := extractIssuedChallenge(submitResp)
	diags.Append(issueDiags...)
	return issued, diags
}

func (r *CertificateResource) cancelChallenge(ctx context.Context, requestID string) {
	cancel := models.NewRequestCancelRequest(requestID, webRAModule, workflowEnroll)
	if _, _, err := r.client.RequestAPI.RequestCancel(ctx).RequestCancelRequest(*cancel).Execute(); err != nil {
		tflog.Warn(ctx, fmt.Sprintf("Could not cancel challenge request %s", requestID), map[string]any{"error": err.Error()})
	}
}

func (r *CertificateResource) findCertificateID(ctx context.Context, certificatePEM string) (*models.Certificate, error) {
	find := models.FindCertificateByPemAsCertificateFindRequest(models.NewFindCertificateByPem(certificatePEM))
	found, _, err := r.client.CertificateAPI.CertificateFind(ctx).CertificateFindRequest(find).Execute()
	if err != nil {
		return nil, err
	}
	if found == nil || found.Certificate.Id == "" {
		return nil, errors.New("Horizon returned no certificate")
	}
	return &found.Certificate, nil
}

func (r *CertificateResource) createWithChallenge(ctx context.Context, data *certificateResourceModel, resp *resource.CreateResponse) {
	subject, sans, diags := challengeIdentity(ctx, *data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	challenge := data.Challenge.ValueString()
	issuedRequestID := ""
	if data.Challenge.IsNull() {
		issued, issueDiags := r.requestChallenge(ctx, *data, subject, sans)
		resp.Diagnostics.Append(issueDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		challenge = issued.challenge
		issuedRequestID = issued.requestID
		tflog.Info(ctx, fmt.Sprintf("Issued WebRA challenge request %s on profile %s", issued.requestID, data.Profile.ValueString()))
	}
	if challenge == "" {
		resp.Diagnostics.AddAttributeError(path.Root("challenge"), "challenge must not be empty", "")
		return
	}

	submit := models.NewWebRAChallengeSubmitRequest(challenge, data.Profile.ValueString(), *challengeSubmitTemplate(*data, subject, sans))
	enrolled, httpResp, err := r.client.ChallengeAPI.ChallengeSubmit(ctx).WebRAChallengeSubmitRequest(*submit).Execute()
	if err != nil {
		if issuedRequestID != "" {
			r.cancelChallenge(ctx, issuedRequestID)
		}
		detail := err.Error()
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			detail += "\n\nThis Horizon has no challenge endpoint: WebRA Challenge mode requires Horizon 2.11 or later."
		}
		resp.Diagnostics.AddError("Failed to enroll certificate with the challenge", detail)
		return
	}
	if enrolled == nil || enrolled.Certificate == "" {
		resp.Diagnostics.AddError("Missing certificate in challenge response", "The challenge submit response did not contain a certificate.")
		return
	}

	data.Certificate = types.StringValue(enrolled.Certificate)
	if data.Csr.IsNull() {
		if enrolled.Pkcs12 == nil || *enrolled.Pkcs12 == "" {
			resp.Diagnostics.AddError("Missing PKCS#12 in challenge response", "Horizon enrolled the certificate without returning its PKCS#12; the private key cannot be recovered.")
			return
		}
		data.Pkcs12 = types.StringValue(*enrolled.Pkcs12)
		// Horizon encrypts the PKCS#12 with the challenge.
		data.Password = types.StringValue(challenge)
		if data.PasswordWriteOnly.ValueBool() {
			data.Password = types.StringNull()
		}
	} else {
		data.Pkcs12 = types.StringNull()
		data.Password = types.StringNull()
	}

	cert, err := r.findCertificateID(ctx, enrolled.Certificate)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to look up the enrolled certificate",
			fmt.Sprintf("Horizon enrolled the certificate, but the provider credentials could not find it. They need the search permission on the profile. %s", err.Error()),
		)
		return
	}

	thirdParties := make([]string, 0, len(data.WaitForThirdParties.Elements()))
	resp.Diagnostics.Append(data.WaitForThirdParties.ElementsAs(ctx, &thirdParties, false)...)
	createTimeout, timeoutDiags := data.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(timeoutDiags...)
	if err := r.waitForThirdParties(ctx, cert.Id, thirdParties, createTimeout); err != nil {
		resp.Diagnostics.AddError("Failed to verify third parties after enrollment", err.Error())
		return
	}

	fillResourceFromCertificate(data, cert)

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}
