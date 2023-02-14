package utils

import (
	"fmt"
	"strings"

	"github.com/evertrust/horizon-go"
	"github.com/evertrust/horizon-go/certificates"
	"github.com/evertrust/horizon-go/requests"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func FillCertificateSchema(d *schema.ResourceData, cert *certificates.Certificate) {
	d.Set("module", cert.Module)
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

func ReEnrollCertificate(d *schema.ResourceData, m interface{}, c *horizon.Horizon, diags diag.Diagnostics) (*requests.HorizonRequest, diag.Diagnostics) {
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

	// Enroll the new certificate with same parameters
	res, err := c.Requests.CentralizedEnroll(profile, subject, sans, labels, keyType, &owner, team)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	d.SetId(res.Certificate.Id)

	return res, diags
}
