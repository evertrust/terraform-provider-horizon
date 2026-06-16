package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	horizon "github.com/evertrust/horizon-go/v2"
	"github.com/evertrust/horizon-go/v2/models"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	// workflowEnroll is declared in certificate_resource.go.
	workflowRecover = "recover"

	sourceEnrollRequest          = "enroll_request"
	sourceRecoverRequest         = "recover_request"
	sourceCreatedRecoveryRequest = "created_recovery_request"
)

var _ ephemeral.EphemeralResource = &RetrieveCentralizedPkcs12EphemeralResource{}
var _ ephemeral.EphemeralResourceWithConfigure = &RetrieveCentralizedPkcs12EphemeralResource{}

func NewRetrieveCentralizedPkcs12EphemeralResource() ephemeral.EphemeralResource {
	return &RetrieveCentralizedPkcs12EphemeralResource{}
}

type RetrieveCentralizedPkcs12EphemeralResource struct {
	client *horizon.APIClient
}

type retrieveCentralizedPkcs12Model struct {
	CertificateID   types.String `tfsdk:"certificate_id"`
	HolderID        types.String `tfsdk:"holder_id"`
	RequestID       types.String `tfsdk:"request_id"`
	RequestWorkflow types.String `tfsdk:"request_workflow"`
	RequestStatus   types.String `tfsdk:"request_status"`
	Source          types.String `tfsdk:"source"`
	Pkcs12          types.String `tfsdk:"pkcs12"`
	Password        types.String `tfsdk:"password"`
}

type pkcs12Material struct {
	CertificateID   string
	HolderID        string
	RequestID       string
	RequestWorkflow string
	RequestStatus   string
	Source          string
	Pkcs12          string
	Password        string
}

func (m pkcs12Material) hasMaterial() bool {
	return m.Pkcs12 != "" && m.Password != ""
}

func (r *RetrieveCentralizedPkcs12EphemeralResource) Metadata(ctx context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_retrieve_centralized_pkcs12"
}

func (r *RetrieveCentralizedPkcs12EphemeralResource) Schema(ctx context.Context, req ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves the centralized PKCS#12 bundle and its password for an existing Horizon certificate. " +
			"The returned material is exposed only as ephemeral output and is never written to Terraform state, saved plan files, or provider private state.\n\n" +
			"!> **Warning — not a recommended pattern.** Distributing private key material through Terraform is discouraged. Prefer decentralized enrollment, where the private key never leaves the consumer, or use Horizon's native integrations to deliver the certificate to its target system. This resource may be deprecated in a future release if a safer delivery mechanism becomes available.\n\n" +
			"~> **Recovery requires key escrow.** When no existing request already exposes the material, the provider creates a WebRA recover request, which Horizon only allows for certificates whose private key was escrowed at enrollment. Against a non-escrow profile the resource returns an actionable error.",
		Attributes: map[string]schema.Attribute{
			"certificate_id": schema.StringAttribute{
				Required:    true,
				Description: "Horizon certificate ID whose centralized PKCS#12 material should be retrieved.",
			},
			"holder_id": schema.StringAttribute{
				Computed:    true,
				Description: "Horizon holder ID associated with the certificate. Sourced from the request response when a request is reused, otherwise from the certificate lookup.",
			},
			"request_id": schema.StringAttribute{
				Computed:    true,
				Description: "Horizon request ID that produced the returned PKCS#12 material.",
			},
			"request_workflow": schema.StringAttribute{
				Computed:    true,
				Description: "Request workflow that produced the returned PKCS#12 material. One of `enroll` or `recover`.",
			},
			"request_status": schema.StringAttribute{
				Computed:    true,
				Description: "Final status of the request used to retrieve the PKCS#12 material.",
			},
			"source": schema.StringAttribute{
				Computed:    true,
				Description: "How the provider obtained the material. One of `enroll_request`, `recover_request`, or `created_recovery_request`.",
			},
			"pkcs12": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Base64-encoded PKCS#12 returned by Horizon.",
			},
			"password": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Password for decrypting `pkcs12`.",
			},
		},
	}
}

func (r *RetrieveCentralizedPkcs12EphemeralResource) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*horizon.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Ephemeral Resource Configure Type",
			fmt.Sprintf("Expected *horizon.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *RetrieveCentralizedPkcs12EphemeralResource) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var data retrieveCentralizedPkcs12Model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	certID := strings.TrimSpace(data.CertificateID.ValueString())
	if certID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("certificate_id"),
			"certificate_id must not be empty",
			"Provide the Horizon certificate ID to retrieve PKCS#12 material for.",
		)
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The Horizon client is not configured. Please report this issue to the provider developers.",
		)
		return
	}

	material, diags := resolvePkcs12(ctx, horizonRequestClient{client: r.client}, certID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.CertificateID = types.StringValue(material.CertificateID)
	data.HolderID = types.StringValue(material.HolderID)
	data.RequestID = types.StringValue(material.RequestID)
	data.RequestWorkflow = types.StringValue(material.RequestWorkflow)
	data.RequestStatus = types.StringValue(material.RequestStatus)
	data.Source = types.StringValue(material.Source)
	data.Pkcs12 = types.StringValue(material.Pkcs12)
	data.Password = types.StringValue(material.Password)

	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}

type requestClient interface {
	certificateHolderID(ctx context.Context, certID string) (string, error)
	search(ctx context.Context, holderID, workflow string) (*models.RequestSearchResultsResponse, error)
	get(ctx context.Context, id string) (*models.RequestGet200Response, error)
	submitRecover(ctx context.Context, certID, password string) (*models.RequestSubmit201Response, error)
}

type horizonRequestClient struct {
	client *horizon.APIClient
}

func (c horizonRequestClient) certificateHolderID(ctx context.Context, certID string) (string, error) {
	cert, _, err := c.client.CertificateAPI.CertificateGetId(ctx, certID).Execute()
	if err != nil {
		return "", err
	}
	return cert.Certificate.GetHolderId(), nil
}

func (c horizonRequestClient) search(ctx context.Context, holderID, workflow string) (*models.RequestSearchResultsResponse, error) {
	query := models.NewRequestSearchQuery()
	query.SetPageSize(100)
	// HRQL field names are lowercase and combinators use `equals` / `and`.
	query.SetQuery(fmt.Sprintf("holderid equals %q and workflow equals %q", holderID, workflow))
	query.SetSortedBy([]models.SortElement{*models.NewSortElement("lastModificationDate", "Desc")})
	query.SetFields([]string{"_id", "certificateId", "workflow", "status", "holderId", "lastModificationDate", "registrationDate"})

	resp, _, err := c.client.RequestAPI.RequestSearch(ctx).RequestSearchQuery(*query).Execute()
	return resp, err
}

func (c horizonRequestClient) get(ctx context.Context, id string) (*models.RequestGet200Response, error) {
	resp, _, err := c.client.RequestAPI.RequestGet(ctx, id).Execute()
	return resp, err
}

func (c horizonRequestClient) submitRecover(ctx context.Context, certID, password string) (*models.RequestSubmit201Response, error) {
	req := models.NewWebRARecoverRequestOnSubmit(workflowRecover)
	req.SetCertificateId(certID)

	secret := models.NewSecretString()
	secret.SetValue(password)
	req.SetPassword(*secret)

	resp, _, err := c.client.RequestAPI.RequestSubmit(ctx).
		RequestSubmitRequest(models.WebRARecoverRequestOnSubmitAsRequestSubmitRequest(req)).
		Execute()
	return resp, err
}

func resolvePkcs12(ctx context.Context, rc requestClient, certID string) (*pkcs12Material, diag.Diagnostics) {
	var diags diag.Diagnostics

	holderID, err := rc.certificateHolderID(ctx, certID)
	if err != nil {
		diags.AddError("Failed to retrieve certificate", err.Error())
		return nil, diags
	}

	if material, found := tryExistingRequest(ctx, rc, certID, holderID, workflowEnroll, sourceEnrollRequest); found {
		return material, nil
	}

	if material, found := tryExistingRequest(ctx, rc, certID, holderID, workflowRecover, sourceRecoverRequest); found {
		return material, nil
	}

	return createRecovery(ctx, rc, certID, holderID)
}

func tryExistingRequest(ctx context.Context, rc requestClient, certID, holderID, workflow, source string) (*pkcs12Material, bool) {
	if holderID == "" {
		// Without a holder there is nothing to scope the search by.
		return nil, false
	}

	searchResp, err := rc.search(ctx, holderID, workflow)
	if err != nil {
		tflog.Debug(ctx, fmt.Sprintf("Skipping reuse of %s requests; search was not usable", workflow), map[string]any{"error": err.Error()})
		return nil, false
	}

	id := selectUsableRequestID(searchResp, certID, workflow)
	if id == "" {
		return nil, false
	}

	getResp, err := rc.get(ctx, id)
	if err != nil {
		tflog.Debug(ctx, fmt.Sprintf("Skipping reuse of %s request %s; get was not usable", workflow, id), map[string]any{"error": err.Error()})
		return nil, false
	}

	material, ok := materialFromGet(getResp)
	if !ok || !material.hasMaterial() {
		return nil, false
	}

	material.Source = source
	if material.CertificateID == "" {
		material.CertificateID = certID
	}
	if material.HolderID == "" {
		material.HolderID = holderID
	}
	return &material, true
}

func createRecovery(ctx context.Context, rc requestClient, certID, holderID string) (*pkcs12Material, diag.Diagnostics) {
	var diags diag.Diagnostics

	password, err := generatePkcs12Password()
	if err != nil {
		diags.AddError(
			"Failed to generate recovery password",
			"Could not generate a secure password for the recovery request.",
		)
		return nil, diags
	}

	submitResp, err := rc.submitRecover(ctx, certID, password)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "escrow") {
			diags.AddError(
				"Certificate is not escrowed",
				"Horizon can only recover centralized PKCS#12 material for certificates whose private key was escrowed at enrollment. "+
					"Enable key escrow on the certificate's profile, or retrieve the PKCS#12 from the original centralized enrollment instead. "+
					"Underlying error: "+err.Error(),
			)
			return nil, diags
		}
		diags.AddError("Failed to submit recovery request", err.Error())
		return nil, diags
	}

	submit := submitResp.WebRARecoverRequestOnSubmitResponse
	if submit == nil {
		diags.AddError(
			"Unexpected request type",
			fmt.Sprintf("Horizon returned an unexpected response type (%T) for the recovery request.", submitResp.GetActualInstance()),
		)
		return nil, diags
	}

	material := materialFromRecoverSubmit(submit)
	material.Source = sourceCreatedRecoveryRequest
	if material.CertificateID == "" {
		material.CertificateID = certID
	}
	if material.HolderID == "" {
		material.HolderID = holderID
	}
	if material.Password == "" {
		material.Password = password
	}

	if material.hasMaterial() {
		return &material, diags
	}

	switch material.RequestStatus {
	case string(models.REQUESTSTATUS_PENDING), string(models.REQUESTSTATUS_DENIED), string(models.REQUESTSTATUS_CANCELED):
		diags.AddError(
			"Recovery request did not complete",
			fmt.Sprintf("Recovery request %s is in status %q and did not return PKCS#12 material. Approval may be required before the material is available.", material.RequestID, material.RequestStatus),
		)
		return nil, diags
	}

	if material.RequestID == "" {
		diags.AddError(
			"Recovery request did not return material",
			"The recovery request completed without a request ID or PKCS#12 material.",
		)
		return nil, diags
	}

	getResp, err := rc.get(ctx, material.RequestID)
	if err != nil {
		diags.AddError("Failed to retrieve recovery request", err.Error())
		return nil, diags
	}

	getMaterial, ok := materialFromGet(getResp)
	if !ok {
		diags.AddError(
			"Unexpected request type",
			fmt.Sprintf("Horizon returned an unexpected response type for recovery request %s.", material.RequestID),
		)
		return nil, diags
	}

	getMaterial.Source = sourceCreatedRecoveryRequest
	if getMaterial.CertificateID == "" {
		getMaterial.CertificateID = certID
	}
	if getMaterial.HolderID == "" {
		getMaterial.HolderID = holderID
	}
	if getMaterial.Password == "" {
		getMaterial.Password = password
	}

	if !getMaterial.hasMaterial() {
		diags.AddError(
			"Recovery request did not return material",
			fmt.Sprintf("Recovery request %s completed without PKCS#12 material.", getMaterial.RequestID),
		)
		return nil, diags
	}

	return &getMaterial, diags
}

func selectUsableRequestID(resp *models.RequestSearchResultsResponse, certID, workflow string) string {
	if resp == nil {
		return ""
	}
	for _, result := range resp.Results {
		if !result.HasCertificateId() || result.GetCertificateId() != certID {
			continue
		}
		if result.HasWorkflow() && string(result.GetWorkflow()) != workflow {
			continue
		}
		if id := result.GetId(); id != "" {
			return id
		}
	}
	return ""
}

func materialFromGet(resp *models.RequestGet200Response) (pkcs12Material, bool) {
	if resp == nil {
		return pkcs12Material{}, false
	}
	if r := resp.WebRARecoverRequestOnApproveResponse; r != nil {
		return materialFromRecoverApprove(r), true
	}
	if r := resp.WebRAEnrollRequestOnGetResponse; r != nil {
		return materialFromEnrollGet(r), true
	}
	return pkcs12Material{}, false
}

func materialFromEnrollGet(r *models.WebRAEnrollRequestOnGetResponse) pkcs12Material {
	m := pkcs12Material{
		RequestID:       r.GetId(),
		RequestWorkflow: r.Workflow,
		RequestStatus:   string(r.GetStatus()),
		HolderID:        r.HolderId,
	}
	if r.HasCertificate() {
		cert := r.GetCertificate()
		m.CertificateID = cert.GetId()
	}
	if r.HasPkcs12() {
		pkcs12 := r.GetPkcs12()
		m.Pkcs12 = pkcs12.GetValue()
	}
	if r.HasPassword() {
		password := r.GetPassword()
		m.Password = password.GetValue()
	}
	return m
}

func materialFromRecoverApprove(r *models.WebRARecoverRequestOnApproveResponse) pkcs12Material {
	m := pkcs12Material{
		RequestID:       r.GetId(),
		RequestWorkflow: r.Workflow,
		RequestStatus:   string(r.GetStatus()),
		HolderID:        r.HolderId,
	}
	if r.HasCertificate() {
		cert := r.GetCertificate()
		m.CertificateID = cert.GetId()
	}
	if r.HasPkcs12() {
		pkcs12 := r.GetPkcs12()
		m.Pkcs12 = pkcs12.GetValue()
	}
	if r.HasPassword() {
		password := r.GetPassword()
		m.Password = password.GetValue()
	}
	return m
}

func materialFromRecoverSubmit(r *models.WebRARecoverRequestOnSubmitResponse) pkcs12Material {
	m := pkcs12Material{
		RequestID:       r.GetId(),
		RequestWorkflow: r.Workflow,
		RequestStatus:   string(r.GetStatus()),
		HolderID:        r.HolderId,
	}
	if r.HasCertificate() {
		cert := r.GetCertificate()
		m.CertificateID = cert.GetId()
	}
	if r.HasPkcs12() {
		pkcs12 := r.GetPkcs12()
		m.Pkcs12 = pkcs12.GetValue()
	}
	if r.HasPassword() {
		password := r.GetPassword()
		m.Password = password.GetValue()
	}
	return m
}

const (
	passwordLength      = 24
	passwordLowerset    = "abcdefghijklmnopqrstuvwxyz"
	passwordUpperset    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	passwordDigitset    = "0123456789"
	passwordSpecialset  = "!@#$%^&*()-_=+"
	passwordFullCharset = passwordLowerset + passwordUpperset + passwordDigitset + passwordSpecialset
)

func generatePkcs12Password() (string, error) {
	classes := []string{passwordLowerset, passwordUpperset, passwordDigitset, passwordSpecialset}
	out := make([]byte, 0, passwordLength)

	for _, class := range classes {
		c, err := randomChar(class)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}

	for len(out) < passwordLength {
		c, err := randomChar(passwordFullCharset)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}

	if err := cryptoShuffle(out); err != nil {
		return "", err
	}

	return string(out), nil
}

func randomChar(charset string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
	if err != nil {
		return 0, err
	}
	return charset[n.Int64()], nil
}

func cryptoShuffle(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		b[i], b[j.Int64()] = b[j.Int64()], b[i]
	}
	return nil
}
