package horizon

import (
	"context"
	"github.com/evertrust/horizon-go/types"
	"time"

	"evertrust.fr/horizon/utils"
	"github.com/evertrust/horizon-go"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceCertificate() *schema.Resource {
	return &schema.Resource{
		Description:   "Provides a Certificate resource. This resource allow you to manage the life cycle of a certificate.",
		CreateContext: resourceCertificateCreate,
		ReadContext:   resourceCertificateRead,
		UpdateContext: resourceCertificateUpdate,
		DeleteContext: resourceCertificateDelete,
		Schema: map[string]*schema.Schema{
			"profile": {
				Description: "Enrollment profile.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"owner": {
				Description: "The owner for the enrolling certificate. By default it will be the user connected in the Provider.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"team": {
				Description: "The team linked to the enrolling certificate.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"contact_email": {
				Description: "The contact email for the enrolling certificate.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"certificate": {
				Description: "Enrolled certificate.",
				Type:        schema.TypeString,
				Optional:    false,
				Computed:    true,
			},
			"thumbprint": {
				Description: "Certificate thumbprint.",
				Type:        schema.TypeString,
				Optional:    false,
				Computed:    true,
			},
			"self_signed": {
				Description: "",
				Type:        schema.TypeBool,
				Optional:    false,
				Computed:    true,
			},
			"public_key_thumbprint": {
				Description: "Certificate publicKeyThumbprint.",
				Type:        schema.TypeString,
				Optional:    false,
				Computed:    true,
			},
			"dn": {
				Description: "certificate DN.",
				Type:        schema.TypeString,
				Optional:    false,
				Computed:    true,
			},
			"subject": {
				Description: "Subject element for the certificate.",
				Type:        schema.TypeSet,
				Optional:    true,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"element": {
							Description: "Subject element.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"type": {
							Description: "Subject element type.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"value": {
							Description: "Subject element value.",
							Type:        schema.TypeString,
							Required:    true,
						},
					},
				},
			},
			"sans": {
				Description: "SAN element for the certificate.",
				Type:        schema.TypeSet,
				Optional:    true,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Description: "SAN element type.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"value": {
							Description: "SAN element values.",
							Type:        schema.TypeList,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
							Required: true,
						},
					},
				},
			},
			"labels": {
				Description: "Labels for the certificate.",
				Type:        schema.TypeSet,
				Optional:    true,
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"label": {
							Description: "Label name.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"value": {
							Description: "Label value.",
							Type:        schema.TypeString,
							Required:    true,
						},
					},
				},
			},
			"serial": {
				Description: "Certificate serial.",
				Type:        schema.TypeString,
				Optional:    false,
				Computed:    true,
			},
			"issuer": {
				Description: "Certificate issuer.",
				Type:        schema.TypeString,
				Optional:    false,
				Computed:    true,
			},
			"not_before": {
				Description: "Certificate creation date.",
				Type:        schema.TypeInt,
				Optional:    false,
				Computed:    true,
			},
			"not_after": {
				Description: "Certificate expiration date.",
				Type:        schema.TypeInt,
				Optional:    false,
				Computed:    true,
			},
			"revocation_date": {
				Description: "Certificate revocation date.",
				Type:        schema.TypeInt,
				Optional:    false,
				Computed:    true,
			},
			"key_type": {
				Description: "This is the keyType you'd like to use in the enrollment of the certificate. It is not compatible with the `csr` argument.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"signing_algorithm": {
				Description: "Certificate signing algorithm.",
				Type:        schema.TypeString,
				Optional:    false,
				Computed:    true,
			},
			"csr": {
				Description: "A CSR file to use the decentralize enroll on Horizon. Be aware that the certificate will be enrolled with the value of your csr. The arguments `subject` and `sans` won't overwrite the CSR.",
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
			},
			"pkcs12": {
				Description: "A base64-encoded PKCS12 certificate data. Available only on centralized enrollments.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
			},
			"password": {
				Description: "The password to use for the PKCS12 certificate. Available only on centralized enrollments.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
			},
			"revoke_on_delete": {
				Description: "An option that allows you to delete the resource without causing the revocation of the certificate. By default it is set at true.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
			},
			"auto_renew": {
				Description: "An option that allows the certificate to automatically renew on read if the peremption date is passed. By default it set at false.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
		},
	}
}

func resourceCertificateCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*horizon.Horizon)

	var diags diag.Diagnostics

	template, err := utils.EnrollTemplateFromResource(c, d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := c.Requests.NewEnrollRequest(types.WebRAEnrollRequestParams{
		Profile:  d.Get("profile").(string),
		Template: template,
		Password: d.Get("password").(string),
	})

	if err != nil {
		return diag.FromErr(err)
	}

	utils.FillCertificateSchema(d, resp.Certificate)

	if resp.Pkcs12 != nil {
		d.Set("pkcs12", resp.Pkcs12.Value)
	}

	if resp.Password != nil {
		d.Set("password", resp.Password.Value)
	}

	return diags
}

func resourceCertificateRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*horizon.Horizon)

	var diags diag.Diagnostics

	res, err := c.Certificate.Get(d.Id())
	if err != nil {
		d.SetId("")
		return diag.FromErr(err)
	}

	utils.FillCertificateSchema(d, res)

	if d.Get("pkcs12") != "" {
		d.Set("pkcs12", d.Get("pkcs12"))
	}

	if d.Get("password") != "" {
		d.Set("password", d.Get("password"))
	}

	if d.Get("revocation_date") != 0 {
		d.SetId("")
		return diags
	}

	notAfter := time.Unix(int64(res.NotAfter/1000), 0)
	if time.Now().After(notAfter) && d.Get("auto_renew").(bool) {
		d.SetId("")
		return diags
	}

	return diags
}

func resourceCertificateUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*horizon.Horizon)

	var diags diag.Diagnostics

	// Revoke the old certificate
	certificate, ok := d.GetOk("certificate")
	if ok {
		_, err := c.Requests.NewRevokeRequest(types.WebRARevokeRequestParams{
			RevocationReason: types.Superseded,
			CertificatePEM:   certificate.(string),
		})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	template, err := utils.EnrollTemplateFromResource(c, d)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := c.Requests.NewEnrollRequest(types.WebRAEnrollRequestParams{
		Profile:  d.Get("profile").(string),
		Template: template,
		Password: d.Get("password").(string),
	})

	if err != nil {
		return diag.FromErr(err)
	}

	utils.FillCertificateSchema(d, resp.Certificate)

	if resp.Pkcs12 != nil {
		d.Set("pkcs12", resp.Pkcs12.Value)
	}

	if resp.Password != nil {
		d.Set("password", resp.Password.Value)
	}

	return diags
}

func resourceCertificateDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*horizon.Horizon)

	var diags diag.Diagnostics

	if d.Get("revoke_on_delete").(bool) {
		certificate, ok := d.GetOk("certificate")
		if ok {
			_, err := c.Requests.NewRevokeRequest(types.WebRARevokeRequestParams{
				RevocationReason: types.CessationOfOperation,
				CertificatePEM:   certificate.(string),
			})
			if err != nil {
				return diag.FromErr(err)
			}
		}
	}

	d.SetId("")

	return diags
}
