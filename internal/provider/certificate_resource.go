package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/evertrust/horizon-go"
	horizontypes "github.com/evertrust/horizon-go/types"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"time"
)

// Ensure provider_with_certificate_auth defined types fully satisfy framework interfaces.
var _ resource.Resource = &CertificateResource{}
var _ resource.ResourceWithImportState = &CertificateResource{}

func NewCertificateResource() resource.Resource {
	return &CertificateResource{}
}

// CertificateResource defines the resource implementation.
type CertificateResource struct {
	client *horizon.Horizon
}

// CertificateResourceModel describes the resource data model.
type CertificateResourceModel struct {
	Id           types.String `tfsdk:"id"`
	Profile      types.String `tfsdk:"profile"`
	Owner        types.String `tfsdk:"owner"`
	Team         types.String `tfsdk:"team"`
	ContactEmail types.String `tfsdk:"contact_email"`
	Subject      types.Set    `tfsdk:"subject"`
	Sans         types.Set    `tfsdk:"sans"`
	Labels       types.Set    `tfsdk:"labels"`

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
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Certificate resource",
		Description:         "Provides a Certificate resource. This resource allow you to manage the life cycle of a certificate.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"profile": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Profile where the certificate will be enrolled into.",
			},
			"subject": schema.SetNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"element": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Subject element.",
						},
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Subject element type.",
						},
						"value": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Subject element value.",
						},
					},
				},
			},
			"sans": schema.SetNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "SAN type.",
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
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"label": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Label name.",
						},
						"value": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Label value.",
						},
					},
				},
			},
			"owner": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Owner of the certificate.",
			},
			"team": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Team of the certificate.",
			},
			"contact_email": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Contact email of the certificate.",
			},
			"serial": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Serial number of the certificate.",
			},
			"issuer": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Issuer of the certificate.",
			},
			"not_before": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Not before date of the certificate.",
			},
			"not_after": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Not after date of the certificate.",
			},
			"revocation_date": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Revocation date of the certificate.",
			},
			"key_type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Key type of the certificate.",
			},
			"signing_algorithm": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Signing algorithm of the certificate.",
			},
			"revoke_on_delete": schema.BoolAttribute{
				MarkdownDescription: "Revoke certificate on delete.",
				Optional:            true,
			},
			"renew_before": schema.Int64Attribute{
				MarkdownDescription: "Renew certificate before expiration.",
				Optional:            true,
			},
			"csr": schema.StringAttribute{
				MarkdownDescription: "CSR to enroll the certificate.",
				Optional:            true,
			},
			"pkcs12": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded PKCS12 file containing the certificate and the private key. Provided when using centralized enrollment.",
				Optional:            true,
				Computed:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password of the PKCS12 file. Provided when using centralized enrollment.",
				Optional:            true,
				Computed:            true,
			},
			"certificate": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded certificate. Provided when using centralized enrollment.",
				Optional:            true,
				Computed:            true,
			},
			"thumbprint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Thumbprint of the certificate.",
			},
			"self_signed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Self-signed certificate.",
			},
			"public_key_thumbprint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Public key thumbprint of the certificate.",
			},
			"dn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "DN of the certificate.",
			},
		},
	}
}

func (r *CertificateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider_with_certificate_auth has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*horizon.Horizon)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider_with_certificate_auth developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *CertificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CertificateResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	template, err := enrollTemplateFromResource(r.client, data)

	if err != nil {
		resp.Diagnostics.AddError("Failed to get enroll template", err.Error())
		return
	}

	response, err := r.client.Requests.NewEnrollRequest(horizontypes.WebRAEnrollRequestParams{
		Profile:  data.Profile.ValueString(),
		Template: template,
		Password: data.Password.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to enroll certificate", err.Error())
		return
	}

	err = fillResourceFromCertificate(&data, response.Certificate)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fill resource from certificate", err.Error())
		return
	}

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
	var data CertificateResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.Certificate.Get(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get certificate", err.Error())
	}

	err = fillResourceFromCertificate(&data, res)
	if err != nil {
		resp.Diagnostics.AddWarning("Failed to fill resource from certificate", err.Error())
		resp.State.RemoveResource(ctx)
	}

	notAfter := time.Unix(int64(res.NotAfter/1000), 0)
	if time.Now().After(notAfter) && data.RenewBefore.ValueInt64() > 0 {
		resp.State.RemoveResource(ctx)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CertificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CertificateResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Revoke the old certificate
	if data.RevokeOnDelete.ValueBool() {
		_, err := r.client.Requests.NewRevokeRequest(horizontypes.WebRARevokeRequestParams{
			RevocationReason: horizontypes.Superseded,
			CertificateId:    data.Id.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Failed to revoke certificate", err.Error())
		}
	}

	template, err := enrollTemplateFromResource(r.client, data)

	if err != nil {
		resp.Diagnostics.AddError("Failed to get enroll template", err.Error())
		return
	}

	response, err := r.client.Requests.NewEnrollRequest(horizontypes.WebRAEnrollRequestParams{
		Profile:  data.Profile.ValueString(),
		Template: template,
		Password: data.Password.ValueString(),
	})

	err = fillResourceFromCertificate(&data, response.Certificate)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fill resource from certificate", err.Error())
		return
	}

	if response.Pkcs12 != nil {
		data.Pkcs12 = types.StringValue(response.Pkcs12.Value)
	}

	if response.Password != nil {
		data.Password = types.StringValue(response.Password.Value)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CertificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CertificateResourceModel

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

func enrollTemplateFromResource(c *horizon.Horizon, d CertificateResourceModel) (*horizontypes.WebRAEnrollTemplate, error) {
	var template *horizontypes.WebRAEnrollTemplate

	if !d.Csr.IsNull() {
		// Start a decentralized enrollment
		if d.KeyType.IsNull() {
			return nil, errors.New("the parameter 'key_type' is not compatible with the parameter 'csr'")
		}

		var err error

		template, err = c.Requests.GetEnrollTemplate(horizontypes.WebRAEnrollTemplateParams{
			Profile: d.Profile.ValueString(),
			Csr:     d.Csr.ValueString(),
		})

		if err != nil {
			return nil, err
		}

	} else {
		// Start a centralized enrollment
		var err error
		template, err = c.Requests.GetEnrollTemplate(horizontypes.WebRAEnrollTemplateParams{
			Profile: d.Profile.ValueString(),
		})

		if err != nil {
			return nil, err
		}

		// Set Subject
		dnElements := make([]attr.Value, 0, len(d.Subject.Elements()))
		diag := d.Subject.ElementsAs(context.Background(), &dnElements, false)
		if diag.HasError() {
			return nil, errors.New("failed to unmarshall subject elements")
		}

		var subject []horizontypes.IndexedDNElement
		for _, dnElement := range dnElements {
			value := dnElement.(basetypes.ObjectValue).Attributes()
			subject = append(subject, horizontypes.IndexedDNElement{
				Element: value["element"].String(),
				Type:    value["type"].String(),
				Value:   value["value"].String(),
			})
		}
		template.Subject = subject

		// Set SANs
		sanElements := make([]attr.Value, 0, len(d.Sans.Elements()))
		diag = d.Sans.ElementsAs(context.Background(), &sanElements, false)
		if diag.HasError() {
			return nil, errors.New("failed to unmarshall san elements")
		}

		var sans []horizontypes.ListSANElement
		d.Sans.ElementsAs(context.Background(), &sans, false)
		for _, sanElement := range sanElements {
			san := sanElement.(basetypes.ObjectValue).Attributes()
			values := []string{}
			for _, value := range san["value"].(basetypes.ObjectValue).Attributes() {
				values = append(values, value.String())
			}
			sans = append(sans, horizontypes.ListSANElement{
				Type:  san["type"].String(),
				Value: values,
			})
		}
		template.Sans = sans

		template.KeyType = d.KeyType.ValueString()

	}

	// Set Labels
	labelElements := make([]attr.Value, 0, len(d.Labels.Elements()))
	diag := d.Labels.ElementsAs(context.Background(), &labelElements, false)
	if diag.HasError() {
		return nil, errors.New("failed to unmarshall label elements")
	}

	var labels []horizontypes.LabelElement
	for _, labelElement := range labelElements {
		label := labelElement.(basetypes.ObjectValue).Attributes()
		labels = append(labels, horizontypes.LabelElement{
			Label: label["label"].String(),
			Value: &horizontypes.String{String: label["value"].String()},
		})
	}
	template.Labels = labels

	if !d.Owner.IsNull() {
		template.Owner = &horizontypes.OwnerElement{Value: &horizontypes.String{String: d.Owner.ValueString()}}
	}

	if !d.Team.IsNull() {
		template.Team = &horizontypes.TeamElement{Value: &horizontypes.String{String: d.Team.ValueString()}}
	}

	// Get contact email
	if !d.ContactEmail.IsNull() {
		template.ContactEmail = &horizontypes.ContactEmailElement{Value: &horizontypes.String{String: d.ContactEmail.ValueString()}}
	}

	return template, nil
}

func fillResourceFromCertificate(d *CertificateResourceModel, certificate *horizontypes.Certificate) error {
	d.Id = types.StringValue(certificate.Id)
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

	return nil
}
