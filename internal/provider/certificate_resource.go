package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/evertrust/horizon-go"
	horizontypes "github.com/evertrust/horizon-go/types"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &CertificateResource{}
var _ resource.ResourceWithImportState = &CertificateResource{}

func NewCertificateResource() resource.Resource {
	return &CertificateResource{}
}

// CertificateResource defines the resource implementation.
type CertificateResource struct {
	client *horizon.Horizon
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
	Id           types.String `tfsdk:"id"`
	Profile      types.String `tfsdk:"profile"`
	Owner        types.String `tfsdk:"owner"`
	Team         types.String `tfsdk:"team"`
	ContactEmail types.String `tfsdk:"contact_email"`
	Subject      types.Set    `tfsdk:"subject"`
	Sans         types.Set    `tfsdk:"sans"`
	Labels       types.Set    `tfsdk:"labels"`
	ThirdParties types.Set    `tfsdk:"third_parties"`

	// Settings

	RevokeOnDelete types.Bool  `tfsdk:"revoke_on_delete"`
	RenewBefore    types.Int64 `tfsdk:"renew_before"`

	Csr         types.String `tfsdk:"csr"`
	Pkcs12      types.String `tfsdk:"pkcs12"`
	Password    types.String `tfsdk:"password"`
	Certificate types.String `tfsdk:"certificate"`

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
			"third_parties": schema.SetAttribute{
				Description: "Third parties ids to which the certificate will be published.",
				Required:    false,
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
				Description: "How many days to renew the certificate before it expires. Certificate renewals rely on the Terraform workspace being run regularly. If the workspace is not run, the certificate will expire.",
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
		},
	}
}

func (r *CertificateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*horizon.Horizon)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
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

	var template *horizontypes.WebRAEnrollTemplate

	if !data.Csr.IsNull() {
		var err error
		template, err = r.client.Requests.GetEnrollTemplate(horizontypes.WebRAEnrollTemplateParams{
			Profile: data.Profile.ValueString(),
			Csr:     data.Csr.ValueString(),
		})

		if err != nil {
			resp.Diagnostics.AddError("Failed to get enroll template", err.Error())
			return
		}

		// This is a decentralized enrollment, so we'll ignore the PKCS12 and password parameters.
		data.Pkcs12 = types.StringNull()
		data.Password = types.StringNull()
	} else {
		// Start a centralized enrollment
		var err error
		template, err = r.client.Requests.GetEnrollTemplate(horizontypes.WebRAEnrollTemplateParams{
			Profile: data.Profile.ValueString(),
		})

		if err != nil {
			resp.Diagnostics.AddError("Failed to get enroll template", err.Error())
			return
		}

		// Set Subject
		subject := make([]certificateSubjectModel, 0, len(data.Subject.Elements()))
		resp.Diagnostics.Append(data.Subject.ElementsAs(context.Background(), &subject, false)...)
		template.Subject = make([]horizontypes.IndexedDNElement, 0, len(subject))
		for _, dnElement := range subject {
			template.Subject = append(template.Subject, horizontypes.IndexedDNElement{
				Element: dnElement.Element.ValueString(),
				Type:    dnElement.Type.ValueString(),
				Value:   dnElement.Value.ValueString(),
			})
		}

		// Set SANs
		sans := make([]certificateSanModel, 0, len(data.Sans.Elements()))
		resp.Diagnostics.Append(data.Sans.ElementsAs(context.Background(), &sans, false)...)
		template.Sans = make([]horizontypes.ListSANElement, 0, len(sans))
		for _, sanElement := range sans {
			values := make([]string, 0, len(sanElement.Value))
			for _, value := range sanElement.Value {
				values = append(values, value.ValueString())
			}
			template.Sans = append(template.Sans, horizontypes.ListSANElement{
				Type:  sanElement.Type.ValueString(),
				Value: values,
			})
		}

		template.KeyType = data.KeyType.ValueString()

	}

	// Set Labels
	labels := make([]certificateLabelModel, 0, len(data.Labels.Elements()))
	resp.Diagnostics.Append(data.Labels.ElementsAs(context.Background(), &labels, false)...)
	template.Labels = make([]horizontypes.LabelElement, 0, len(labels))
	for _, label := range labels {
		template.Labels = append(template.Labels, horizontypes.LabelElement{
			Label: label.Label.ValueString(),
			Value: &horizontypes.String{String: label.Value.ValueString()},
		})
	}

	if !data.Owner.IsNull() {
		template.Owner = &horizontypes.OwnerElement{Value: &horizontypes.String{String: data.Owner.ValueString()}}
	}

	if !data.Team.IsNull() {
		template.Team = &horizontypes.TeamElement{Value: &horizontypes.String{String: data.Team.ValueString()}}
	}

	// Get contact email
	if !data.ContactEmail.IsNull() {
		template.ContactEmail = &horizontypes.ContactEmailElement{Value: &horizontypes.String{String: data.ContactEmail.ValueString()}}
	}

	// Enroll the certificate
	tflog.Info(ctx, fmt.Sprintf("Enrolling certificate into profile %s", data.Profile.ValueString()))

	response, err := r.client.Requests.NewEnrollRequest(horizontypes.WebRAEnrollRequestParams{
		Profile:  data.Profile.ValueString(),
		Template: template,
		Password: data.Password.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to enroll certificate", err.Error())
		return
	}

	// Check that certificates are successfully added to Third Parties
	thirdParties := make([]string, 0, len(data.ThirdParties.Elements()))
	resp.Diagnostics.Append(data.ThirdParties.ElementsAs(context.Background(), &thirdParties, false)...)
	// If ThirdParties were defined, poll the certificate until all of them are in the 'thirdPartyData' field
	if len(thirdParties) > 0 {
		err = pollForThirdParties(r.client, response.Id, thirdParties)
		if err != nil {
			resp.Diagnostics.AddError("Failed to verify third parties after enrollment", err.Error())
			return
		}
	}

	fillResourceFromCertificate(&data, response.Certificate)

	if response.Pkcs12 != nil {
		data.Pkcs12 = types.StringValue(response.Pkcs12.Value)
	}

	if response.Password != nil {
		data.Password = types.StringValue(response.Password.Value)
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
	res, err := r.client.Certificate.Get(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get certificate", err.Error())
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Successfully got certificate %s", data.Id.ValueString()))

	fillResourceFromCertificate(&data, res)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	//notAfter := time.Unix(int64(res.NotAfter/1000), 0)
	renewalDate := time.UnixMilli(data.NotAfter.ValueInt64()).AddDate(0, 0, -int(data.RenewBefore.ValueInt64()))
	if time.Now().After(renewalDate) && data.RenewBefore.ValueInt64() > 0 {
		tflog.Info(ctx, fmt.Sprintf("Certificate is in its renewal period, renewing it (expires at %s, computed renewal date is %s).", time.UnixMilli(int64(res.NotAfter)), renewalDate))
		resp.State.RemoveResource(ctx)
	} else {
		tflog.Debug(ctx, fmt.Sprintf("Certificate is not in its renewal period (expires at %s, computed renewal date is %s).", time.UnixMilli(int64(res.NotAfter)), renewalDate))
	}
}

func (r *CertificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data certificateResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	// Preserve existing PKCS12 and password from state
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("pkcs12"), &data.Pkcs12)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("password"), &data.Password)...)

	if resp.Diagnostics.HasError() {
		return
	}

	template, err := r.client.Requests.GetUpdateTemplate(horizontypes.WebRAUpdateTemplateParams{
		CertificateId: data.Id.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to get update template", err.Error())
		return
	}

	// Set Labels
	labels := make([]certificateLabelModel, 0, len(data.Labels.Elements()))
	resp.Diagnostics.Append(data.Labels.ElementsAs(context.Background(), &labels, false)...)
	template.Labels = make([]horizontypes.LabelElement, 0, len(labels))
	for _, label := range labels {
		template.Labels = append(template.Labels, horizontypes.LabelElement{
			Label: label.Label.ValueString(),
			Value: &horizontypes.String{String: label.Value.ValueString()},
		})
	}

	if !data.Owner.IsNull() {
		template.Owner = &horizontypes.OwnerElement{Value: &horizontypes.String{String: data.Owner.ValueString()}}
	}

	if !data.Team.IsNull() {
		template.Team = &horizontypes.TeamElement{Value: &horizontypes.String{String: data.Team.ValueString()}}
	}

	// Get contact email
	if !data.ContactEmail.IsNull() {
		template.ContactEmail = &horizontypes.ContactEmailElement{Value: &horizontypes.String{String: data.ContactEmail.ValueString()}}
	}

	response, err := r.client.Requests.NewUpdateRequest(horizontypes.WebRAUpdateRequestParams{
		CertificateId: data.Id.ValueString(),
		Template:      template,
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to update certificate", err.Error())
		return
	}

	fillResourceFromCertificate(&data, response.Certificate)

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
		_, err := r.client.Requests.NewRevokeRequest(horizontypes.WebRARevokeRequestParams{
			RevocationReason: horizontypes.CessationOfOperation,
			CertificateId:    data.Id.ValueString(),
		})
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
	}
}

// Fill the computed attributes of the resource
func fillResourceFromCertificate(d *certificateResourceModel, certificate *horizontypes.Certificate) {
	d.Id = types.StringValue(certificate.Id)
	d.Certificate = types.StringValue(certificate.Certificate)
	d.Thumbprint = types.StringValue(certificate.Thumbprint)
	d.SelfSigned = types.BoolValue(certificate.SelfSigned)
	d.PublicKeyThumbprint = types.StringValue(certificate.PublicKeyThumbprint)
	d.Dn = types.StringValue(certificate.Dn)
	d.Serial = types.StringValue(certificate.Serial)
	d.Issuer = types.StringValue(certificate.Issuer)
	d.NotBefore = types.Int64Value(int64(certificate.NotBefore))
	d.NotAfter = types.Int64Value(int64(certificate.NotAfter))
	d.RevocationDate = types.Int64Value(int64(certificate.RevocationDate))
	d.KeyType = types.StringValue(certificate.KeyType)
	d.SigningAlgorithm = types.StringValue(certificate.SigningAlgorithm)
}

// Poll the certificate until all third parties are present in the 'thirdPartyData' field
func pollForThirdParties(horizonClient *horizon.Horizon, requestId string, thirdParties []string) error {

	foundThirdParties := make(map[string]bool, len(thirdParties))
	for _, thirdParty := range thirdParties {
		foundThirdParties[thirdParty] = false
	}

	const MAX_RETRIES = 10
	timePadding := 15 * time.Second
	nbToFind := len(thirdParties)
	for retries := 0; retries < MAX_RETRIES; retries++ {
		time.Sleep(timePadding)
		polledEnrollRequest, err := horizonClient.Requests.GetEnrollRequest(requestId)
		if err != nil {
			return fmt.Errorf("failed to poll certificate after enrollment: %s", err.Error())
		}
		if polledEnrollRequest.Certificate.ThirdPartyData != nil {
			// Check if the certificate have been added to all third parties
			for _, thirdPartyData := range polledEnrollRequest.Certificate.ThirdPartyData {
				_, ok := foundThirdParties[thirdPartyData.Id]
				if !ok {
					// Found a third party we didn't expect, ignore it
					continue
				} else if !foundThirdParties[thirdPartyData.Id] {
					// if the third party was not already found, mark it as found
					foundThirdParties[thirdPartyData.Id] = true
					nbToFind--
					if nbToFind == 0 {
						// All third parties have been found, stop polling
						break
					}
				}
			}
		}
	}
	if nbToFind > 0 {
		thirdPartiesNotFound := make([]string, 0)
		for thirdParty, found := range foundThirdParties {
			if !found {
				thirdPartiesNotFound = append(thirdPartiesNotFound, thirdParty)
			}
		}
		return fmt.Errorf("timeout... failed to find all third parties after enrollment. Could not find: %v", thirdPartiesNotFound)
	}
	return nil
}
