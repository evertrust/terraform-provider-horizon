package utils

import (
	"errors"
	horizontypes "github.com/evertrust/horizon-go"
	horizon "github.com/evertrust/horizon-go/client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func FillCertificateSchema(d *schema.ResourceData, cert *horizontypes.Certificate) {
	d.SetId(cert.Id)
	d.Set("profile", cert.Profile)
	d.Set("owner", cert.Owner)
	d.Set("certificate", cert.Certificate)
	d.Set("thumbprint", cert.Thumbprint)
	d.Set("self_signed", cert.SelfSigned)
	d.Set("public_key_thumbprint", cert.PublicKeyThumbprint)
	d.Set("dn", cert.Dn)
	d.Set("serial", cert.Serial)
	d.Set("issuer", cert.Issuer)
	d.Set("not_before", cert.NotBefore)
	d.Set("not_after", cert.NotAfter)
	d.Set("revocation_date", cert.RevocationDate)
	d.Set("revocation_reason", cert.RevocationReason)
	d.Set("key_type", cert.KeyType)
	d.Set("signing_algorithm", cert.SigningAlgorithm)
}

func EnrollTemplateFromResource(c *horizon.Client, d *schema.ResourceData) (*horizontypes.WebRAEnrollTemplate, error) {
	var template *horizontypes.WebRAEnrollTemplate

	csr, isDecentralized := d.GetOk("csr")

	if isDecentralized {
		_, keyTypeOk := d.GetOk("key_type")
		if keyTypeOk {
			return nil, errors.New("the parameter 'key_type' is not compatible with the parameter 'csr'")
		}

		var err error

		template, err = c.Requests.GetEnrollTemplate(horizontypes.WebRAEnrollTemplateParams{
			Profile: d.Get("profile").(string),
			Csr:     csr.(string),
		})

		if err != nil {
			return nil, err
		}

	} else {
		var err error
		template, err = c.Requests.GetEnrollTemplate(horizontypes.WebRAEnrollTemplateParams{
			Profile: d.Get("profile").(string),
		})

		if err != nil {
			return nil, err
		}

		// Set Subject
		var subject []horizontypes.IndexedDNElement
		dnElements := d.Get("subject").(*schema.Set)
		for _, dnElement := range dnElements.List() {
			dn := dnElement.(map[string]interface{})
			subject = append(subject, horizontypes.IndexedDNElement{
				Element: dn["element"].(string),
				Type:    dn["type"].(string),
				Value:   dn["value"].(string),
			})
		}
		template.Subject = subject

		// Set SANs
		var sans []horizontypes.ListSANElement
		sanElements := d.Get("sans").(*schema.Set)
		for _, sanElement := range sanElements.List() {
			san := sanElement.(map[string]interface{})
			values := []string{}
			for _, value := range san["value"].([]interface{}) {
				values = append(values, value.(string))
			}
			sans = append(sans, horizontypes.ListSANElement{
				Type:  san["type"].(string),
				Value: values,
			})
		}
		template.Sans = sans

		// Get keyType
		keyType := d.Get("key_type").(string)

		template.KeyType = keyType

	}

	// Set Labels
	var labels []horizontypes.LabelElement
	labelElements := d.Get("labels").(*schema.Set)
	for _, labelElement := range labelElements.List() {
		label := labelElement.(map[string]interface{})
		labels = append(labels, horizontypes.LabelElement{
			Label: label["label"].(string),
			Value: &horizontypes.String{String: label["value"].(string)},
		})
	}
	template.Labels = labels

	// Get owner
	owner, hasOwner := d.GetOk("owner")
	if hasOwner {
		template.Owner = &horizontypes.OwnerElement{Value: &horizontypes.String{String: owner.(string)}}
	}

	// Get team
	team, hasTeam := d.GetOk("team")
	if hasTeam {
		template.Team = &horizontypes.TeamElement{Value: &horizontypes.String{String: team.(string)}}
	}

	// Get contact email
	contactEmail, hasContactEmail := d.GetOk("contact_email")
	if hasContactEmail {
		template.ContactEmail = &horizontypes.ContactEmailElement{Value: &horizontypes.String{String: contactEmail.(string)}}
	}

	return template, nil
}
