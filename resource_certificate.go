package main

// TODO: Setup the client to use the one we initialized in the provider
// Actually my creds are implemented in code and it is really ugly

import (
	"context"
	"fmt"
	"strings"

	"github.com/evertrust/horizon-go"
	"github.com/evertrust/horizon-go/certificates"
	"github.com/evertrust/horizon-go/requests"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceCertificate() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCertificateCreate,
		ReadContext:   resourceCertificateRead,
		UpdateContext: resourceCertificateUpdate,
		DeleteContext: resourceCertificateDelete,
		Schema: map[string]*schema.Schema{
			"module": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"profile": {
				Type:     schema.TypeString,
				Required: true,
			},
			"owner": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"team": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"type": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"certificate": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"thumbprint": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"self_signed": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
			"public_key_thumbprint": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"dn": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"subject": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"element": {
							Type:     schema.TypeString,
							Required: true,
						},
						"type": {
							Type:     schema.TypeString,
							Required: true,
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"sans": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"element": {
							Type:     schema.TypeString,
							Required: true,
						},
						"type": {
							Type:     schema.TypeString,
							Required: true,
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"labels": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"label": {
							Type:     schema.TypeString,
							Required: true,
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"serial": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"issuer": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"not_before": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},
			"not_after": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},
			"revocation_date": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},
			"revocation_reason": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"key_type": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"signing_algorithm": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"certificate_pem": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"csr": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"revoke_on_delete": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
		},
	}
}

func resourceCertificateCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*horizon.Horizon)

	var diags diag.Diagnostics

	// Get the values used in both enrollment method

	profile := d.Get("profile").(string)
	// Set Labels
	var labels []requests.LabelElement
	labelElements := d.Get("labels").(*schema.Set)
	for _, labelElement := range labelElements.List() {
		label := labelElement.(map[string]interface{})
		labels = append(labels, requests.LabelElement{
			Label: label["label"].(string),
			Value: label["value"].(string),
		})
	}
	// Get owner
	var owner *string
	tempOwner, ownerOk := d.GetOk("owner")
	if ownerOk {
		*owner = tempOwner.(string)
	} else {
		owner = nil
	}
	// Get team
	var team *string
	tempTeam, teamOk := d.GetOk("team")
	if teamOk {
		*team = tempTeam.(string)
	} else {
		team = nil
	}
	// The presence of a CSR will determine which enrollment method will be used
	// Get CSR
	tempCsr, csrOk := d.GetOk("csr")
	if csrOk {
		csr := []byte(tempCsr.(string))

		_, keyTypeOk := d.GetOk("key_type")
		if keyTypeOk {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Not needed argument",
				Detail:   "The parameter 'key_type' is not compatible with the parameter 'csr'.",
			})
			return diags
		}

		res, err := c.Requests.DecentralizedEnroll(profile, csr, labels, owner, team)
		if err != nil {
			return diag.FromErr(err)
		}

		// SetId => Mandatory
		d.SetId(res.Certificate.Id)

		fillCertificateSchema(
			d,
			string(res.Certificate.Module),
			string(res.Certificate.Profile),
			string(res.Certificate.Owner),
			string(res.Certificate.Certificate),
			string(res.Certificate.Thumbprint),
			bool(res.Certificate.SelfSigned),
			string(res.Certificate.PublicKeyThumbprint),
			string(res.Certificate.Dn),
			string(res.Certificate.Serial),
			string(res.Certificate.Issuer),
			int(res.Certificate.NotBefore),
			int(res.Certificate.NotAfter),
			int(res.Certificate.RevocationDate),
			string(res.Certificate.RevocationReason),
			string(res.Certificate.KeyType),
			string(res.Certificate.SigningAlgorithm),
		)

	} else {
		// Set Subject
		var subject []requests.IndexedDNElement
		var typeCounts = make(map[string]int)
		dnElements := d.Get("subject").(*schema.Set)
		for _, dnElement := range dnElements.List() {
			dn := dnElement.(map[string]interface{})
			typeCounts[dn["type"].(string)]++
			subject = append(subject, requests.IndexedDNElement{
				Element: fmt.Sprintf("%s.%d", strings.ToLower(dn["type"].(string)), typeCounts[dn["type"].(string)]),
				Type:    strings.ToUpper(dn["type"].(string)),
				Value:   fmt.Sprintf("%v", dn["value"].(string)),
			})
		}

		// Set SANs
		var sans []requests.IndexedSANElement
		typeCounts = make(map[string]int)
		sanElements := d.Get("sans").(*schema.Set)
		for _, sanElement := range sanElements.List() {
			san := sanElement.(map[string]interface{})
			typeCounts[san["type"].(string)]++
			sans = append(sans, requests.IndexedSANElement{
				Element: fmt.Sprintf("%s.%d", strings.ToLower(san["type"].(string)), typeCounts[san["type"].(string)]),
				Type:    strings.ToUpper(san["type"].(string)),
				Value:   fmt.Sprintf("%v", san["value"].(string)),
			})
		}
		// Get keyType
		keyType := d.Get("key_type").(string)

		res, err := c.Requests.CentralizedEnroll(profile, subject, sans, labels, keyType, owner, team)
		if err != nil {
			return diag.FromErr(err)
		}

		// SetId => Mandatory
		d.SetId(res.Certificate.Id)

		fillCertificateSchema(
			d,
			string(res.Certificate.Module),
			string(res.Certificate.Profile),
			string(res.Certificate.Owner),
			string(res.Certificate.Certificate),
			string(res.Certificate.Thumbprint),
			bool(res.Certificate.SelfSigned),
			string(res.Certificate.PublicKeyThumbprint),
			string(res.Certificate.Dn),
			string(res.Certificate.Serial),
			string(res.Certificate.Issuer),
			int(res.Certificate.NotBefore),
			int(res.Certificate.NotAfter),
			int(res.Certificate.RevocationDate),
			string(res.Certificate.RevocationReason),
			string(res.Certificate.KeyType),
			string(res.Certificate.SigningAlgorithm),
		)

	}

	// Call read
	// Is it necessary ?
	resourceCertificateRead(ctx, d, m)

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

	fillCertificateSchema(
		d,
		string(res.Module),
		string(res.Profile),
		string(res.Owner),
		string(res.Certificate),
		string(res.Thumbprint),
		bool(res.SelfSigned),
		string(res.PublicKeyThumbprint),
		string(res.Dn),
		string(res.Serial),
		string(res.Issuer),
		int(res.NotBefore),
		int(res.NotAfter),
		int(res.RevocationDate),
		string(res.RevocationReason),
		string(res.KeyType),
		string(res.SigningAlgorithm),
	)

	return diags
}

func resourceCertificateUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*horizon.Horizon)

	var diags diag.Diagnostics

	// Get all values
	profile := d.Get("profile").(string)
	// Set Labels
	var labels []requests.LabelElement
	labelElements := d.Get("labels").(*schema.Set)
	for _, labelElement := range labelElements.List() {
		label := labelElement.(map[string]interface{})
		labels = append(labels, requests.LabelElement{
			Label: label["label"].(string),
			Value: label["value"].(string),
		})
	}
	// Set subject
	var subject []requests.IndexedDNElement
	var typeCounts = make(map[string]int)
	dnElements := d.Get("subject").(*schema.Set)
	for _, dnElement := range dnElements.List() {
		dn := dnElement.(map[string]interface{})
		typeCounts[dn["type"].(string)]++
		subject = append(subject, requests.IndexedDNElement{
			Element: fmt.Sprintf("%s.%d", strings.ToLower(dn["type"].(string)), typeCounts[dn["type"].(string)]),
			Type:    dn["type"].(string),
			Value:   fmt.Sprintf("%v", dn["value"].(string)),
		})
	}
	// Set SANs
	var sans []requests.IndexedSANElement
	typeCounts = make(map[string]int)
	sanElements := d.Get("sans").(*schema.Set)
	for _, sanElement := range sanElements.List() {
		san := sanElement.(map[string]interface{})
		typeCounts[san["type"].(string)]++
		sans = append(sans, requests.IndexedSANElement{
			Element: fmt.Sprintf("%s.%d", strings.ToLower(san["type"].(string)), typeCounts[san["type"].(string)]),
			Type:    san["type"].(string),
			Value:   fmt.Sprintf("%v", san["value"].(string)),
		})
	}
	// Get keyType
	keyType := d.Get("key_type").(string)
	// Get owner
	owner := d.Get("owner").(string)
	// Get team
	var team *string
	tempTeam, teamOk := d.GetOk("team")
	if teamOk {
		*team = tempTeam.(string)
	} else {
		team = nil
	}

	// Revoke the old certificate
	revocationReason := certificates.RevocationReason(d.Get("revocation_reason").(string))
	tempCertificate, ok := d.GetOk("certificate")
	if ok {
		certificate := tempCertificate.(string)
		_, err := c.Requests.Revoke(certificate, revocationReason)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	// Enroll the new certificate with same parameters
	res, err := c.Requests.CentralizedEnroll(profile, subject, sans, labels, keyType, &owner, team)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(res.Certificate.Id)

	// Update the schema with values from new certificate
	fillCertificateSchema(
		d,
		string(res.Certificate.Module),
		string(res.Certificate.Profile),
		string(res.Certificate.Owner),
		string(res.Certificate.Certificate),
		string(res.Certificate.Thumbprint),
		bool(res.Certificate.SelfSigned),
		string(res.Certificate.PublicKeyThumbprint),
		string(res.Certificate.Dn),
		string(res.Certificate.Serial),
		string(res.Certificate.Issuer),
		int(res.Certificate.NotBefore),
		int(res.Certificate.NotAfter),
		int(res.Certificate.RevocationDate),
		string(res.Certificate.RevocationReason),
		string(res.Certificate.KeyType),
		string(res.Certificate.SigningAlgorithm),
	)

	// Call read
	// Is it necessary ?
	resourceCertificateRead(ctx, d, m)

	return diags
}

func resourceCertificateDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*horizon.Horizon)

	var diags diag.Diagnostics

	if d.Get("revoke_on_delete").(bool) {
		revocation_reason := certificates.RevocationReason(d.Get("revocation_reason").(string))
		tempCertificate, ok := d.GetOk("certificate")
		if ok {
			certificate := tempCertificate.(string)
			_, err := c.Requests.Revoke(certificate, revocation_reason)
			if err != nil {
				return diag.FromErr(err)
			}
		}
	}

	d.SetId("")

	return diags
}
