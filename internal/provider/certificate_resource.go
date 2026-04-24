package provider

import (
	"context"
	"fmt"
	"time"

	horizon "github.com/evertrust/horizon-go/v2"
	"github.com/evertrust/horizon-go/v2/models"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

const (
	webRAModule      = "webra"
	workflowEnroll   = "enroll"
	workflowUpdate   = "update"
	workflowRenew    = "renew"
	workflowRevoke   = "revoke"
	revocationReason = "cessationofoperation"
)

func NewCertificateResource() resource.Resource {
	return &CertificateResource{}
}

// CertificateResource defines the resource implementation.
type CertificateResource struct {
	client *horizon.APIClient
}

type certificateSubjectModel struct {
	Element types.String `tfsdk:"element"`
	Type    types.String `tfsdk:"type"`
	Value   types.String `tfsdk:"value"`
}

type certificateSanModel struct {
	Type  types.String   `tfsdk:"type"`
	Value []types.String `tfsdk:"value"`
}

type certificateLabelModel struct {
	Label types.String `tfsdk:"label"`
	Value types.String `tfsdk:"value"`
}

// certificateResourceModel describes the resource data model.
type certificateResourceModel struct {
	Id                  types.String `tfsdk:"id"`
	Profile             types.String `tfsdk:"profile"`
	Owner               types.String `tfsdk:"owner"`
	Team                types.String `tfsdk:"team"`
	ContactEmail        types.String `tfsdk:"contact_email"`
	Subject             types.Set    `tfsdk:"subject"`
	Sans                types.Set    `tfsdk:"sans"`
	Labels              types.Set    `tfsdk:"labels"`
	WaitForThirdParties types.Set    `tfsdk:"wait_for_third_parties"`

	// Settings

	RevokeOnDelete types.Bool  `tfsdk:"revoke_on_delete"`
	RenewBefore    types.Int64 `tfsdk:"renew_before"`

	Csr               types.String `tfsdk:"csr"`
	Pkcs12            types.String `tfsdk:"pkcs12"`
	Password          types.String `tfsdk:"password"`
	Pkcs12WriteOnly   types.Bool   `tfsdk:"pkcs12_write_only"`
	PasswordWriteOnly types.Bool   `tfsdk:"password_write_only"`
	Certificate       types.String `tfsdk:"certificate"`

	Thumbprint          types.String `tfsdk:"thumbprint"`
	SelfSigned          types.Bool   `tfsdk:"self_signed"`
	PublicKeyThumbprint types.String `tfsdk:"public_key_thumbprint"`
	Dn                  types.String `tfsdk:"dn"`
	Serial              types.String `tfsdk:"serial"`
	Issuer              types.String `tfsdk:"issuer"`
	NotBefore           types.Int64  `tfsdk:"not_before"`
	NotAfter            types.Int64  `tfsdk:"not_after"`
	RevocationDate      types.Int64  `tfsdk:"revocation_date"`
	KeyType             types.String `tfsdk:"key_type"`
	SigningAlgorithm    types.String `tfsdk:"signing_algorithm"`
	RenewalTrigger      types.String `tfsdk:"renewal_trigger"`

	Timeouts timeouts.Value `tfsdk:"timeouts"`
}

func (r *CertificateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_certificate"
}

func (r *CertificateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a Certificate resource. This resource allow you to manage the lifecycle of a certificate. To enroll a certificate, you can either provide a CSR (Certificate Signing Request), or a subject and a list of SANs. If you provide a CSR, the enrollment will be decentralized. If you provide a subject and SANs, the enrollment will be centralized.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Internal certificate identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"profile": schema.StringAttribute{
				Required:    true,
				Description: "Profile where the certificate will be enrolled into.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subject": schema.SetNestedAttribute{
				Optional:    true,
				Description: "Subject elements of the certificate. This is ignored when csr is provided. ",
				NestedObject: schema.NestedAttributeObject{
					PlanModifiers: []planmodifier.Object{
						objectplanmodifier.RequiresReplace(),
					},
					Attributes: map[string]schema.Attribute{
						"element": schema.StringAttribute{
							Required:    true,
							Description: "Subject element, followed by a dot and the index of the element. For example: `cn.1` for the first common name.",
						},
						"type": schema.StringAttribute{
							Required:    true,
							Description: "Subject element type. For example: `CN` for common name.",
						},
						"value": schema.StringAttribute{
							Required:    true,
							Description: "Subject element value. For example: `www.example.com` for common name.",
						},
					},
				},
			},
			"sans": schema.SetNestedAttribute{
				Description: "Subject alternative names of the certificate. This is ignored when csr is provided.",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					PlanModifiers: []planmodifier.Object{
						objectplanmodifier.RequiresReplace(),
					},
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Required:    true,
							Description: "SAN type. Accepted values are: `RFC822NAME`, `DNSNAME`, `URI`, `IPADDRESS`, `OTHERNAME_UPN`, `OTHERNAME_GUID`",
						},
						"value": schema.SetAttribute{
							Required:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Subject element values.",
						},
					},
				},
			},
			"labels": schema.SetNestedAttribute{
				Description: "Labels of the certificate, used to enrich the certificate metadata on Horizon.",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"label": schema.StringAttribute{
							Required:    true,
							Description: "Label name.",
						},
						"value": schema.StringAttribute{
							Required:    true,
							Description: "Label value.",
						},
					},
				},
			},
			"wait_for_third_parties": schema.SetAttribute{
				Description: "Third parties ids to which the certificate will be published.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"owner": schema.StringAttribute{
				Optional:    true,
				Description: "Owner associated with the certificate.",
			},
			"team": schema.StringAttribute{
				Optional:    true,
				Description: "Team associated with the certificate.",
			},
			"contact_email": schema.StringAttribute{
				Optional:    true,
				Description: "Contact email associated with the certificate.",
			},
			"serial": schema.StringAttribute{
				Computed:    true,
				Description: "Serial number of the certificate.",
			},
			"issuer": schema.StringAttribute{
				Computed:    true,
				Description: "Issuer DN of the certificate.",
			},
			"not_before": schema.Int64Attribute{
				Computed:    true,
				Description: "NotBefore attribute of the certificate.",
			},
			"not_after": schema.Int64Attribute{
				Computed:    true,
				Description: "NotAfter attribute (expiration date) of the certificate.",
			},
			"revocation_date": schema.Int64Attribute{
				Computed:    true,
				Description: "Revocation date of the certificate. Empty when the certificate is not revoked.",
			},
			"key_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Key type of the certificate. For example: `rsa-2048`.",
			},
			"signing_algorithm": schema.StringAttribute{
				Computed:    true,
				Description: "Signing algorithm of the certificate. For example: `SHA256WITHRSA`",
			},
			"revoke_on_delete": schema.BoolAttribute{
				Description: "Whether to revoke certificate when it is removed from the Terraform state or not.",
				Optional:    true,
			},
			"renew_before": schema.Int64Attribute{
				Description: "How many days before expiration the certificate should be renewed. When a `terraform plan` or `terraform apply` runs inside that window, the provider triggers a real WebRA renew (in-place update) for centralized enrollments, and a destroy/create for decentralized enrollments (a renew with the same CSR would reuse the key, which is rarely desirable). Renewals rely on the Terraform workspace being run regularly; if it is not run, the certificate will expire.",
				Optional:    true,
			},
			"csr": schema.StringAttribute{
				Description: "A CSR (Certificate Signing Request) in PEM format. Providing this attribute will trigger a decentralized enrollment. Incompatible with `subject` and `sans`.",
				Optional:    true,
			},
			"pkcs12": schema.StringAttribute{
				Description: "Base64-encoded PKCS12 file containing the certificate and the private key. Provided when using centralized enrollment.",
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
			},
			"password": schema.StringAttribute{
				Description: "Password of the PKCS12 file. Can be provided when using centralized enrollment, or will be generated by Horizon if not set.",
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
			},
			"pkcs12_write_only": schema.BoolAttribute{
				Description: "When true, the PKCS12 value returned/generated for centralized enrollment is not persisted to Terraform state. Only meaningful for centralized enrollment. Sensitive material will not be recoverable from state after apply.",
				Optional:    true,
			},
			"password_write_only": schema.BoolAttribute{
				Description: "When true, the PKCS12 password is not persisted to Terraform state. Only meaningful for centralized enrollment. Sensitive material will not be recoverable from state after apply.",
				Optional:    true,
			},
			"certificate": schema.StringAttribute{
				Description: "Certificate in the PEM format.",
				Optional:    true,
				Computed:    true,
			},
			"thumbprint": schema.StringAttribute{
				Computed:    true,
				Description: "Thumbprint of the certificate.",
			},
			"self_signed": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this is a self-signed certificate.",
			},
			"public_key_thumbprint": schema.StringAttribute{
				Computed:    true,
				Description: "Public key thumbprint of the certificate.",
			},
			"dn": schema.StringAttribute{
				Computed:    true,
				Description: "DN of the certificate.",
			},
			"renewal_trigger": schema.StringAttribute{
				Computed:    true,
				Description: "Internal marker derived from `not_after`. The provider flips this value to force Terraform to plan a renewal when the `renew_before` window opens. Not meant to be set or referenced by users; it exists only to make renewal plannable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
			}),
		},
	}
}

func (r *CertificateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*horizon.APIClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *horizon.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *CertificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data certificateResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateWriteOnlyFlags(data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	template := models.NewWebRAEnrollRequestTemplateWithDefaults()

	if !data.Csr.IsNull() {
		// Decentralized enrollment: the submit endpoint does not parse the
		// CSR to fill subject/sans, so fetch the template first so the server
		// extracts them, then reuse that seed for submit.
		onTemplate := models.NewWebRAEnrollRequestOnTemplate(webRAModule, workflowEnroll)
		onTemplate.SetProfile(data.Profile.ValueString())
		seed := map[string]interface{}{"csr": data.Csr.ValueString()}
		onTemplate.SetTemplate(seed)

		tmplResp, _, err := r.client.RequestAPI.RequestTemplate(ctx).
			RequestTemplateRequest(models.WebRAEnrollRequestOnTemplateAsRequestTemplateRequest(onTemplate)).
			Execute()
		if err != nil {
			resp.Diagnostics.AddError("Failed to get enroll template", err.Error())
			return
		}
		onTemplateResp := tmplResp.WebRAEnrollRequestOnTemplateResponse
		if onTemplateResp == nil {
			resp.Diagnostics.AddError("Unexpected template response type", "Expected WebRAEnrollRequestOnTemplateResponse")
			return
		}

		template.SetCsr(data.Csr.ValueString())
		respTemplate := onTemplateResp.Template
		if len(respTemplate.Subject) > 0 {
			subjectElements := make([]models.IndexedDNElement, 0, len(respTemplate.Subject))
			for _, e := range respTemplate.Subject {
				el := models.IndexedDNElement{Element: e.Element}
				if v, ok := e.GetValueOk(); ok && v != nil {
					el.SetValue(*v)
				}
				subjectElements = append(subjectElements, el)
			}
			template.SetSubject(subjectElements)
		}
		if len(respTemplate.Sans) > 0 {
			sanElements := make([]models.ListSANElement, 0, len(respTemplate.Sans))
			for _, e := range respTemplate.Sans {
				el := models.ListSANElement{Value: e.Value}
				if t, ok := e.GetTypeOk(); ok && t != nil {
					el.SetType(*t)
				}
				sanElements = append(sanElements, el)
			}
			template.SetSans(sanElements)
		}

		// This is a decentralized enrollment, so we'll ignore the PKCS12 and password parameters.
		data.Pkcs12 = types.StringNull()
		data.Password = types.StringNull()
	} else {
		// Set Subject
		subject := make([]certificateSubjectModel, 0, len(data.Subject.Elements()))
		resp.Diagnostics.Append(data.Subject.ElementsAs(ctx, &subject, false)...)
		subjectElements := make([]models.IndexedDNElement, 0, len(subject))
		for _, dnElement := range subject {
			el := models.IndexedDNElement{Element: dnElement.Element.ValueString()}
			el.SetValue(dnElement.Value.ValueString())
			subjectElements = append(subjectElements, el)
		}
		template.SetSubject(subjectElements)

		// Set SANs
		sans := make([]certificateSanModel, 0, len(data.Sans.Elements()))
		resp.Diagnostics.Append(data.Sans.ElementsAs(ctx, &sans, false)...)
		sanElements := make([]models.ListSANElement, 0, len(sans))
		for _, sanElement := range sans {
			values := make([]string, 0, len(sanElement.Value))
			for _, value := range sanElement.Value {
				values = append(values, value.ValueString())
			}
			el := models.ListSANElement{Value: values}
			el.SetType(sanElement.Type.ValueString())
			sanElements = append(sanElements, el)
		}
		template.SetSans(sanElements)

		if !data.KeyType.IsNull() {
			template.SetKeyType(data.KeyType.ValueString())
		}
	}

	// Set Labels
	labels := make([]certificateLabelModel, 0, len(data.Labels.Elements()))
	resp.Diagnostics.Append(data.Labels.ElementsAs(ctx, &labels, false)...)
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

	// Build the submit payload. NewWebRAEnrollRequestOnSubmit takes a profile
	// argument but the generated constructor does not assign it — set it
	// explicitly.
	submit := models.NewWebRAEnrollRequestOnSubmit(
		data.Profile.ValueString(),
		webRAModule,
		*template,
		workflowEnroll,
	)
	submit.SetProfile(data.Profile.ValueString())
	if !data.Password.IsNull() && data.Password.ValueString() != "" {
		secret := models.NewSecretStringWithDefaults()
		secret.SetValue(data.Password.ValueString())
		submit.SetPassword(*secret)
	}

	apiReq := r.client.RequestAPI.RequestSubmit(ctx).
		RequestSubmitRequest(models.WebRAEnrollRequestOnSubmitAsRequestSubmitRequest(submit))
	submitResp, _, err := apiReq.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Failed to enroll certificate", err.Error())
		return
	}

	enrollResp := submitResp.WebRAEnrollRequestOnSubmitResponse
	if enrollResp == nil {
		resp.Diagnostics.AddError("Unexpected response type", "Expected WebRAEnrollRequestOnSubmitResponse")
		return
	}

	cert := enrollResp.Certificate.Get()
	if cert == nil {
		resp.Diagnostics.AddError("Missing certificate in enroll response", "The enroll response did not contain a certificate.")
		return
	}

	// Check that certificates are successfully added to Third Parties
	thirdParties := make([]string, 0, len(data.WaitForThirdParties.Elements()))
	resp.Diagnostics.Append(data.WaitForThirdParties.ElementsAs(ctx, &thirdParties, false)...)

	createTimeout, diags := data.Timeouts.Create(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)

	// If ThirdParties were defined, poll the certificate until all of them are in the 'thirdPartyData' field
	if len(thirdParties) > 0 {
		err = retry.RetryContext(ctx, createTimeout, func() *retry.RetryError {
			certificateId := cert.Id
			tflog.Info(ctx, fmt.Sprintf("Polling certificate %s for third parties: %v", certificateId, thirdParties))

			polledResp, _, pErr := r.client.CertificateAPI.CertificateGetId(ctx, certificateId).Execute()
			if pErr != nil {
				return retry.RetryableError(fmt.Errorf("failed to poll certificate after enrollment: %s", pErr.Error()))
			}
			polled := polledResp.GetCertificate()
			tflog.Info(ctx, fmt.Sprintf("Polling certificate, get third parties: %v", polled.ThirdPartyData))
			// Check if the certificate has been added to all third parties
			if !hasThirdParties(polled.ThirdPartyData, thirdParties) {
				return retry.RetryableError(fmt.Errorf("failed to find all third parties after enrollment : %v", polled.ThirdPartyData))
			}
			return nil
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to verify third parties after enrollment", err.Error())
			return
		}
	}

	fillResourceFromCertificate(&data, cert)

	if enrollResp.Pkcs12.IsSet() && enrollResp.Pkcs12.Get() != nil && !data.Pkcs12WriteOnly.ValueBool() {
		data.Pkcs12 = types.StringValue(enrollResp.Pkcs12.Get().GetValue())
	} else if data.Pkcs12WriteOnly.ValueBool() {
		data.Pkcs12 = types.StringNull()
	}

	if enrollResp.Password.IsSet() && enrollResp.Password.Get() != nil && !data.PasswordWriteOnly.ValueBool() {
		data.Password = types.StringValue(enrollResp.Password.Get().GetValue())
	} else if data.PasswordWriteOnly.ValueBool() {
		data.Password = types.StringNull()
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CertificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data certificateResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, fmt.Sprintf("Getting certificate %s", data.Id.ValueString()))
	certResp, _, err := r.client.CertificateAPI.CertificateGetId(ctx, data.Id.ValueString()).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Failed to get certificate", err.Error())
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Successfully got certificate %s", data.Id.ValueString()))

	cert := certResp.GetCertificate()
	fillResourceFromCertificate(&data, toCertificate(&cert))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CertificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data certificateResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	resp.Diagnostics.Append(validateWriteOnlyFlags(data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve existing PKCS12 and password from state only when not in write-only mode
	if !data.Pkcs12WriteOnly.ValueBool() {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("pkcs12"), &data.Pkcs12)...)
	} else {
		data.Pkcs12 = types.StringNull()
	}
	if !data.PasswordWriteOnly.ValueBool() {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("password"), &data.Password)...)
	} else {
		data.Password = types.StringNull()
	}
	if resp.Diagnostics.HasError() {
		return
	}

	var plannedTrigger types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("renewal_trigger"), &plannedTrigger)...)
	var stateID types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &stateID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	renewRequested := plannedTrigger.IsUnknown()

	certID := stateID.ValueString()

	if renewRequested {
		tflog.Info(ctx, fmt.Sprintf("Renewing certificate %s via WebRA renew", certID))

		renewTemplate := models.NewWebRARenewRequestTemplateWithDefaults()
		if !data.KeyType.IsNull() && !data.KeyType.IsUnknown() && data.KeyType.ValueString() != "" {
			renewTemplate.SetKeyType(data.KeyType.ValueString())
		}

		renewSubmit := models.NewWebRARenewRequestOnSubmit(webRAModule, workflowRenew)
		renewSubmit.SetCertificateId(certID)
		renewSubmit.SetTemplate(*renewTemplate)
		if !data.Password.IsNull() && !data.Password.IsUnknown() && data.Password.ValueString() != "" {
			secret := models.NewSecretStringWithDefaults()
			secret.SetValue(data.Password.ValueString())
			renewSubmit.SetPassword(*secret)
		}

		renewResp, _, err := r.client.RequestAPI.RequestSubmit(ctx).
			RequestSubmitRequest(models.WebRARenewRequestOnSubmitAsRequestSubmitRequest(renewSubmit)).
			Execute()
		if err != nil {
			resp.Diagnostics.AddError("Failed to renew certificate", err.Error())
			return
		}

		renewed, renewDiags := extractRenewedCertificate(renewResp)
		resp.Diagnostics.Append(renewDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		renewedCert := renewed.Certificate.Get()

		certResp, _, err := r.client.CertificateAPI.CertificateGetId(ctx, renewedCert.Id).Execute()
		if err != nil {
			resp.Diagnostics.AddError("Failed to fetch renewed certificate", err.Error())
			return
		}
		normalized := certResp.GetCertificate()
		fillResourceFromCertificate(&data, toCertificate(&normalized))
		certID = data.Id.ValueString()

		if renewed.Pkcs12.IsSet() && renewed.Pkcs12.Get() != nil && !data.Pkcs12WriteOnly.ValueBool() {
			data.Pkcs12 = types.StringValue(renewed.Pkcs12.Get().GetValue())
		} else if data.Pkcs12WriteOnly.ValueBool() {
			data.Pkcs12 = types.StringNull()
		}
		if renewed.Password.IsSet() && renewed.Password.Get() != nil && !data.PasswordWriteOnly.ValueBool() {
			data.Password = types.StringValue(renewed.Password.Get().GetValue())
		} else if data.PasswordWriteOnly.ValueBool() {
			data.Password = types.StringNull()
		}
	}

	template := models.NewWebRAUpdateRequestTemplateWithDefaults()

	labels := make([]certificateLabelModel, 0, len(data.Labels.Elements()))
	resp.Diagnostics.Append(data.Labels.ElementsAs(ctx, &labels, false)...)
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

	submit := models.NewWebRAUpdateRequestOnSubmit(*template, workflowUpdate)
	submit.SetCertificateId(certID)

	submitResp, _, err := r.client.RequestAPI.RequestSubmit(ctx).
		RequestSubmitRequest(models.WebRAUpdateRequestOnSubmitAsRequestSubmitRequest(submit)).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Failed to update certificate", err.Error())
		return
	}

	cert, updateDiags := extractUpdatedCertificate(submitResp)
	resp.Diagnostics.Append(updateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	fillResourceFromCertificate(&data, cert)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CertificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data certificateResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.RevokeOnDelete.ValueBool() {
		revokeTemplate := models.NewWebRARevokeRequestTemplateWithDefaults()
		reason := revocationReason
		revokeTemplate.RevocationReason.Set(&reason)

		submit := models.NewWebRARevokeRequestOnSubmit(*revokeTemplate, workflowRevoke)
		submit.SetCertificateId(data.Id.ValueString())

		_, _, err := r.client.RequestAPI.RequestSubmit(ctx).
			RequestSubmitRequest(models.WebRARevokeRequestOnSubmitAsRequestSubmitRequest(submit)).
			Execute()
		if err != nil {
			resp.Diagnostics.AddError("Failed to revoke certificate", err.Error())
		}
	}
}

func (r *CertificateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r CertificateResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data certificateResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if !data.Csr.IsNull() {
		// We assume we're in a decentralized enrollment mode, so we'll warn about useless parameters.
		if !data.KeyType.IsNull() {
			resp.Diagnostics.AddAttributeWarning(path.Root("key_type"), "key_type is ignored when csr is provided.", "")
		}

		if len(data.Subject.Elements()) > 0 {
			resp.Diagnostics.AddAttributeWarning(path.Root("subject"), "subject is ignored when csr is provided.", "")
		}

		if len(data.Sans.Elements()) > 0 {
			resp.Diagnostics.AddAttributeWarning(path.Root("sans"), "sans is ignored when csr is provided.", "")
		}

		if data.Pkcs12WriteOnly.ValueBool() {
			resp.Diagnostics.AddAttributeWarning(path.Root("pkcs12_write_only"), "pkcs12_write_only has no effect when csr is provided (decentralized enrollment).", "")
		}

		if data.PasswordWriteOnly.ValueBool() {
			resp.Diagnostics.AddAttributeWarning(path.Root("password_write_only"), "password_write_only has no effect when csr is provided (decentralized enrollment).", "")
		}
	}
}

func (r *CertificateResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state, plan certificateResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !isInRenewalWindow(state.NotAfter, plan.RenewBefore, time.Now()) {
		return
	}

	tflog.Info(ctx, fmt.Sprintf("Certificate %s is in its renewal window (expires at %s).", state.Id.ValueString(), time.UnixMilli(state.NotAfter.ValueInt64())))

	if !plan.Csr.IsNull() {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("renewal_trigger"))
	}

	plan.RenewalTrigger = types.StringUnknown()
	plan.Id = types.StringUnknown()
	plan.Serial = types.StringUnknown()
	plan.Issuer = types.StringUnknown()
	plan.Certificate = types.StringUnknown()
	plan.Thumbprint = types.StringUnknown()
	plan.PublicKeyThumbprint = types.StringUnknown()
	plan.Dn = types.StringUnknown()
	plan.SigningAlgorithm = types.StringUnknown()
	plan.NotBefore = types.Int64Unknown()
	plan.NotAfter = types.Int64Unknown()

	if plan.Csr.IsNull() {
		if !plan.Pkcs12WriteOnly.ValueBool() {
			plan.Pkcs12 = types.StringUnknown()
		}
		if !plan.PasswordWriteOnly.ValueBool() {
			plan.Password = types.StringUnknown()
		}
	}

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// Fill the computed attributes of the resource
func fillResourceFromCertificate(d *certificateResourceModel, certificate *models.Certificate) {
	d.Id = types.StringValue(certificate.Id)
	d.Certificate = types.StringValue(certificate.Certificate)
	d.Thumbprint = types.StringValue(certificate.Thumbprint)
	d.SelfSigned = types.BoolValue(certificate.SelfSigned)
	d.PublicKeyThumbprint = types.StringValue(certificate.PublicKeyThumbprint)
	d.Dn = types.StringValue(certificate.Dn)
	d.Serial = types.StringValue(certificate.Serial)
	d.Issuer = types.StringValue(certificate.Issuer)
	d.NotBefore = types.Int64Value(certificate.NotBefore)
	d.NotAfter = types.Int64Value(certificate.NotAfter)
	if certificate.RevocationDate.IsSet() && certificate.RevocationDate.Get() != nil {
		d.RevocationDate = types.Int64Value(*certificate.RevocationDate.Get())
	} else {
		d.RevocationDate = types.Int64Value(0)
	}
	d.KeyType = types.StringValue(certificate.KeyType)
	d.SigningAlgorithm = types.StringValue(certificate.SigningAlgorithm)
	d.RenewalTrigger = types.StringValue(renewalTriggerFor(certificate.NotAfter))
}

func renewalTriggerFor(notAfter int64) string {
	return fmt.Sprintf("renew-%d", notAfter)
}

// extractRenewedCertificate validates the shape of a renew response and
// returns the inner WebRARenewRequestOnSubmitResponse. Diagnostics carry a
// clear error for each failure mode; the caller must check diags before
// dereferencing the returned pointer.
func extractRenewedCertificate(resp *models.RequestSubmit201Response) (*models.WebRARenewRequestOnSubmitResponse, diag.Diagnostics) {
	var diags diag.Diagnostics
	if resp == nil {
		diags.AddError("Empty renew response", "The WebRA renew request returned no response body.")
		return nil, diags
	}
	renewed := resp.WebRARenewRequestOnSubmitResponse
	if renewed == nil {
		diags.AddError("Unexpected response type", "Expected WebRARenewRequestOnSubmitResponse")
		return nil, diags
	}
	if renewed.Certificate.Get() == nil {
		diags.AddError("Missing certificate in renew response", "The renew response did not contain a certificate.")
		return nil, diags
	}
	return renewed, diags
}

// extractUpdatedCertificate validates the shape of a metadata update response
// and returns the Certificate payload. Same failure-mode contract as
// extractRenewedCertificate.
func extractUpdatedCertificate(resp *models.RequestSubmit201Response) (*models.Certificate, diag.Diagnostics) {
	var diags diag.Diagnostics
	if resp == nil {
		diags.AddError("Empty update response", "The WebRA update request returned no response body.")
		return nil, diags
	}
	updated := resp.WebRAUpdateRequestOnSubmitResponse
	if updated == nil {
		diags.AddError("Unexpected response type", "Expected WebRAUpdateRequestOnSubmitResponse")
		return nil, diags
	}
	cert := updated.Certificate.Get()
	if cert == nil {
		diags.AddError("Missing certificate in update response", "The update response did not contain a certificate.")
		return nil, diags
	}
	return cert, diags
}

func isInRenewalWindow(notAfter types.Int64, renewBeforeDays types.Int64, now time.Time) bool {
	if renewBeforeDays.IsNull() || renewBeforeDays.IsUnknown() || renewBeforeDays.ValueInt64() <= 0 {
		return false
	}
	if notAfter.IsNull() || notAfter.IsUnknown() || notAfter.ValueInt64() == 0 {
		return false
	}
	renewalDate := time.UnixMilli(notAfter.ValueInt64()).AddDate(0, 0, -int(renewBeforeDays.ValueInt64()))
	return !now.Before(renewalDate)
}

// toCertificate converts a CertificateResponse into a Certificate so downstream
// helpers can keep working with a single shape.
func toCertificate(r *models.CertificateResponse) *models.Certificate {
	return &models.Certificate{
		Id:                    r.Id,
		Certificate:           r.Certificate,
		ContactEmail:          r.ContactEmail,
		CrlSynchronized:       r.CrlSynchronized,
		DiscoveredTrusted:     r.DiscoveredTrusted,
		DiscoveryData:         r.DiscoveryData,
		DiscoveryInfo:         r.DiscoveryInfo,
		Dn:                    r.Dn,
		Escrowed:              r.Escrowed,
		Extensions:            r.Extensions,
		Grades:                r.Grades,
		HolderId:              r.HolderId,
		Issuer:                r.Issuer,
		KeyType:               r.KeyType,
		Labels:                r.Labels,
		Metadata:              r.Metadata,
		Module:                r.Module,
		NotAfter:              r.NotAfter,
		NotBefore:             r.NotBefore,
		Owner:                 r.Owner,
		Profile:               r.Profile,
		PublicKeyThumbprint:   r.PublicKeyThumbprint,
		RevocationDate:        r.RevocationDate,
		RevocationReason:      r.RevocationReason,
		Revoked:               r.Revoked,
		SelfSigned:            r.SelfSigned,
		Serial:                r.Serial,
		SigningAlgorithm:      r.SigningAlgorithm,
		SubjectAlternateNames: r.SubjectAlternateNames,
		Team:                  r.Team,
		ThirdPartyData:        r.ThirdPartyData,
		Thumbprint:            r.Thumbprint,
		TriggerResults:        r.TriggerResults,
	}
}

// validateWriteOnlyFlags rejects unknown values for the write-only flags.
func validateWriteOnlyFlags(data certificateResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if data.Pkcs12WriteOnly.IsUnknown() {
		diags.AddAttributeError(
			path.Root("pkcs12_write_only"),
			"pkcs12_write_only must be known at plan time",
			"This flag controls whether sensitive PKCS12 material is persisted to state and cannot be derived from another resource's computed output.",
		)
	}
	if data.PasswordWriteOnly.IsUnknown() {
		diags.AddAttributeError(
			path.Root("password_write_only"),
			"password_write_only must be known at plan time",
			"This flag controls whether the sensitive PKCS12 password is persisted to state and cannot be derived from another resource's computed output.",
		)
	}
	return diags
}

func hasThirdParties(thirdPartyData []models.ThirdPartyItem, thirdParties []string) bool {
	// build a set of connectors present in the certificate
	connectors := make(map[string]struct{}, len(thirdPartyData))
	for _, tpData := range thirdPartyData {
		connectors[tpData.Connector] = struct{}{}
	}

	// check that all expected IDs are present
	for _, tp := range thirdParties {
		if _, ok := connectors[tp]; !ok {
			return false
		}
	}
	return true
}
