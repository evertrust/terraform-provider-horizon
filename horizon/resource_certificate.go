package horizon

// TODO: Setup the client to use the one we initialized in the provider
// Actually my creds are implemented in code and it is really ugly

import (
	"context"
	"time"

	"evertrust.fr/horizon/utils"
	"github.com/evertrust/horizon-go"
	"github.com/evertrust/horizon-go/certificates"
	old "github.com/evertrust/horizon-go/v2"
	oldr "github.com/evertrust/horizon-go/v2/requests"
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
			"horizon_version": {
				Description: "Horizon version. Useful for version before 2.4.",
				Type:        schema.TypeFloat,
				Optional:    true,
				Computed:    false,
				Default:     2.4,
			},
			"module": {
				Description: "Enrollment module.",
				Type:        schema.TypeString,
				Optional:    false,
				Computed:    true,
			},
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
				Default:     nil,
			},
			"team": {
				Description: "The team linked to the enrolling certificate.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Default:     nil,
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
						"element": {
							Description: "SAN element.",
							Type:        schema.TypeString,
							Optional:    true,
						},
						"type": {
							Description: "SAN element type.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"value": {
							Description: "SAN element value.",
							Type:        schema.TypeString,
							Required:    true,
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
			"revocation_reason": {
				Description: "Certificate revocation reason.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"key_type": {
				Description: "This is the keyType you'd like to use in the enrollment of the crtificate. It is not compatible with the `csr`argument.",
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
			"certificate_pem": {
				Description: "Enrolled certificate pem file.",
				Type:        schema.TypeString,
				Optional:    false,
				Computed:    true,
			},
			"csr": {
				Description: "A CSR file to use the decentralize enroll on Horizon. Be aware that the certificate will be enrolled with the value of your csr. The arguments `subject` and `sans` won't overwrite the CSR.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
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
	var diags diag.Diagnostics
	version := d.Get("horizon_version").(float64)

	// Get the values used in both enrollment method
	profile := d.Get("profile").(string)
	var owner *string
	tempOwner, ownerOk := d.GetOk("team")
	if ownerOk {
		owner = tempOwner.(*string)
	} else {
		owner = nil
	}
	var team *string
	tempTeam, teamOk := d.GetOk("team")
	if teamOk {
		team = tempTeam.(*string)
	} else {
		team = nil
	}
	// The presence of a CSR will determine which enrollment method will be used
	// Get CSR
	tempCsr, csrOk := d.GetOk("csr")
	if csrOk {
		csr := []byte(tempCsr.(string))
		// Manage the keyType to avoid errors later
		_, keyTypeOk := d.GetOk("key_type")
		if keyTypeOk {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Not needed argument",
				Detail:   "The parameter 'key_type' is not compatible with the parameter 'csr'.",
			})
			return diags
		}

		if version >= 2.4 {
			c := m.(*horizon.Horizon)
			res, err := c.Requests.DecentralizedEnroll(profile, csr, utils.SetLabels2(d), owner, team)
			if err != nil {
				return diag.FromErr(err)
			}
			// SetId => Mandatory
			d.SetId(res.Certificate.Id)
			utils.FillCertificateSchema(d, res.Certificate)
		} else {
			tmp := m.(*old.Horizon)
			o := oldr.Client(*tmp.Requests)
			res, err := o.DecentralizedEnroll(profile, csr, utils.SetLabels(d), owner, team)
			if err != nil {
				return diag.FromErr(err)
			}
			// SetId => Mandatory
			d.SetId(res.Certificate.Id)
			utils.FillCertificateSchema(d, res.Certificate)
		}
	} else { // No CSR -> centralized enroll
		if version >= 2.4 {
			c := m.(*horizon.Horizon)
			res, err := c.Requests.CentralizedEnroll(profile, "", utils.SetSubject2(d), utils.SetSans2(d), utils.SetLabels2(d), d.Get("key_type").(string), owner, team)
			if err != nil {
				return diag.FromErr(err)
			}
			// SetId => Mandatory
			d.SetId(res.Certificate.Id)
			utils.FillCertificateSchema(d, res.Certificate)
		} else {
			tmp := m.(*old.Horizon)
			o := oldr.Client(*tmp.Requests)
			res, err := o.CentralizedEnroll(profile, "", utils.SetSubject(d), utils.SetSans(d), utils.SetLabels(d), d.Get("key_type").(string), owner, team)
			if err != nil {
				return diag.FromErr(err)
			}

			// SetId => Mandatory
			d.SetId(res.Certificate.Id)
			utils.FillCertificateSchema(d, res.Certificate)
		}
	}
	resourceCertificateRead(ctx, d, m)
	return diags
}

func resourceCertificateRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	version := d.Get("horizon_version").(float64)

	if version >= 2.4 {
		c := m.(*horizon.Horizon)
		res, err := c.Certificate.Get(d.Id())
		if err != nil {
			d.SetId("")
			return diag.FromErr(err)
		}
		utils.FillCertificateSchema(d, res)
	} else {
		o := m.(*old.Horizon)
		res, err := o.Certificate.Get(d.Id())
		if err != nil {
			d.SetId("")
			return diag.FromErr(err)
		}
		utils.FillCertificateSchema(d, res)
	}

	if d.Get("revocation_date") != 0 {
		d.SetId("")
		return diags
	}

	notAfter := time.Unix(int64(d.Get("not_after").(int)/1000), 0)
	if time.Now().After(notAfter) && d.Get("auto_renew").(bool) {
		d.SetId("")
		return diags
	}

	return diags
}

func resourceCertificateUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	// Revoke the old certificate
	revocationReason := certificates.RevocationReason(d.Get("revocation_reason").(string))
	tempCertificate, ok := d.GetOk("certificate")
	if ok {
		certificate := tempCertificate.(string)
		version := d.Get("horizon_version").(float64)
		if version <= 2.4 {
			c := m.(*horizon.Horizon)
			_, err := c.Requests.Revoke(certificate, revocationReason)
			if err != nil {
				return diag.FromErr(err)
			}
			diags = utils.ReEnrollCertificate(d, m, c, nil, diags)
		} else {
			o := m.(*old.Horizon)
			_, err := o.Requests.Revoke(certificate, revocationReason)
			if err != nil {
				return diag.FromErr(err)
			}
			diags = utils.ReEnrollCertificate(d, m, nil, o, diags)
		}
	}

	resourceCertificateRead(ctx, d, m)

	return diags
}

func resourceCertificateDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	if d.Get("revoke_on_delete").(bool) {
		revocation_reason := certificates.RevocationReason(d.Get("revocation_reason").(string))
		tempCertificate, ok := d.GetOk("certificate")
		if ok {
			certificate := tempCertificate.(string)
			version := d.Get("horizon_version").(float64)
			if version >= 2.4 {
				c := m.(*horizon.Horizon)
				_, err := c.Requests.Revoke(certificate, revocation_reason)
				if err != nil {
					return diag.FromErr(err)
				}
			} else {
				o := m.(*old.Horizon)
				_, err := o.Requests.Revoke(certificate, revocation_reason)
				if err != nil {
					return diag.FromErr(err)
				}
			}

		}
	}
	d.SetId("")
	return diags
}
