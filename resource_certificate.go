package main

// TODO: Setup the client to use the one we initialized in the provider
// Actually my creds are implemented in code and it is really ugly

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/evertrust/horizon-go"
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
			// "module": {
			// 	Type:     schema.TypeString,
			// 	Required: true,
			// },
			"profile": {
				Type:     schema.TypeString,
				Required: true,
			},
			"owner": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"team": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"type": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"certificate": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			// "thumbprint": {
			// 	Type:     schema.TypeString,
			// 	Optional: true,
			// },
			// "selfSigned": {
			// 	Type:     schema.TypeBool,
			// 	Optional: true,
			// },
			// "publicKeyThumbprint": {
			// 	Type:     schema.TypeString,
			// 	Optional: true,
			// },

			// => DN or subject ?

			// "dn": {
			// 	Type:     schema.TypeString,
			// 	Required: true,
			// },
			"subject": {
				Type:     schema.TypeSet,
				Required: true,
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
				Required: true,
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
			// "serial": {
			// 	Type:     schema.TypeString,
			// 	Optional: true,
			// },
			// "issuer": {
			// 	Type:     schema.TypeString,
			// 	Optional: true,
			// },
			// "notBefore": {
			// 	Type:     schema.TypeInt,
			// 	Optional: true,
			// },
			// "notAfter": {
			// 	Type:     schema.TypeInt,
			// 	Optional: true,
			// },
			// "revocationDate": {
			// 	Type:     schema.TypeInt,
			// 	Optional: true,
			// },
			"revocation_reason": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"key_type": {
				Type:     schema.TypeString,
				Optional: true,
			},
			// "signingAlgorithm": {
			// 	Type:     schema.TypeString,
			// 	Optional: true,
			// },
			"certificate_pem": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceCertificateCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*horizon.Horizon)

	var diags diag.Diagnostics

	// // Set Subject
	var subject []requests.IndexedDNElement
	var typeCounts = make(map[string]int)
	dnElements := d.Get("subject").(*schema.Set)
	for _, dnElement := range dnElements.List() {
		dn := dnElement.(map[string]interface{})
		// fmt.Printf("---- ELEMENT ====> %v ----\n", dn["element"])
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
		fmt.Printf("sanElement: %v\n", sanElement)
		typeCounts[san["type"].(string)]++
		sans = append(sans, requests.IndexedSANElement{
			Element: fmt.Sprintf("%s.%d", strings.ToLower(san["type"].(string)), typeCounts[san["type"].(string)]),
			Type:    san["type"].(string),
			Value:   fmt.Sprintf("%v", san["value"].(string)),
		})
	}

	// Set Labels
	var labels []requests.LabelElement
	labelElements := d.Get("labels").(*schema.Set)
	fmt.Printf("labelElements: %v\n", labelElements)
	for _, labelElemant := range labelElements.List() {
		label := labelElemant.(map[string]interface{})
		labels = append(labels, requests.LabelElement{
			Label: label["label"].(string),
			Value: label["value"].(string),
		})
	}

	// Get parameters
	profile := d.Get("profile").(string)
	key_type := d.Get("key_type").(string)
	// owner := ""
	// team := ""
	// owner := d.Get("owner").(string)
	// team := d.Get("team").(string)
	// fmt.Printf("Test line 217 -----\n")
	// fmt.Printf("------- %v, %v, %v, %v -------\n", profile, key_type, owner, team)

	// Get CSR

	res, err := c.Requests.CentralizedEnroll(profile, subject, sans, labels, key_type, nil, nil)

	fmt.Printf("Certificate : %v\n%v", res.Certificate.Certificate, reflect.TypeOf(res.Certificate.Certificate))

	// SetId => Mandatory
	d.SetId(res.Id)
	d.Get("certificate")
	d.Set("certificate", string(res.Certificate.Certificate))

	if err != nil {
		fmt.Printf("--- CENTRALIZED ENROLL ERROR : %v\n", err)
		return diag.FromErr(err)
	}

	resourceCertificateRead(ctx, d, m)

	return diags
}

func resourceCertificateRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	d.Id()
	return diags
}

func resourceCertificateUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return resourceCertificateRead(ctx, d, m)
}

func resourceCertificateDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	// c := new(horizon.Horizon)
	// url, _ := url.Parse("https://horizon-qa.evertrust.io")
	// c.Init(*url, "adu", "adu")

	var diags diag.Diagnostics

	// certificate_pem := d.Get("certificate_pem").(string)
	// revocation_reason := certificates.RevocationReason(d.Get("revocation_reason").(string))

	// _, err := c.Requests.Revoke(certificate_pem, revocation_reason)
	// if err != nil {
	// 	return diag.FromErr(err)
	// }

	d.SetId("")

	return diags
}
