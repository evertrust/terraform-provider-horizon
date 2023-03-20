package utils

import (
	"fmt"
	"strings"

	"github.com/evertrust/horizon-go"
	"github.com/evertrust/horizon-go/certificates"
	"github.com/evertrust/horizon-go/requests"
	old "github.com/evertrust/horizon-go/v2"
	oldr "github.com/evertrust/horizon-go/v2/requests"
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

func ReEnrollCertificate(d *schema.ResourceData, m interface{}, c *horizon.Horizon, oldHorizon *old.Horizon, diags diag.Diagnostics) diag.Diagnostics {
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

	version := d.Get("horizon_version").(float64)
	if version >= 2.4 {
		res, err := c.Requests.CentralizedEnroll(
			d.Get("profile").(string),
			"",
			SetSubject2(d),
			SetSans2(d),
			SetLabels2(d),
			d.Get("key_type").(string),
			&owner,
			team)
		if err != nil {
			return diag.FromErr(err)
		}
		// SetId => Mandatory
		d.SetId(res.Certificate.Id)
		FillCertificateSchema(d, res.Certificate)
		return diags
	} else {
		o := oldr.Client(*oldHorizon.Requests)
		res, err := o.CentralizedEnroll(
			d.Get("profile").(string),
			"",
			SetSubject(d),
			SetSans(d),
			SetLabels(d),
			d.Get("key_type").(string),
			&owner,
			team)
		if err != nil {
			return diag.FromErr(err)
		}
		// SetId => Mandatory
		d.SetId(res.Certificate.Id)
		FillCertificateSchema(d, res.Certificate)
		return diags
	}
}

func SetSubject2(d *schema.ResourceData) []requests.IndexedDNElement {
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
	return subject
}

func SetSubject(d *schema.ResourceData) []oldr.IndexedDNElement {
	var subject []oldr.IndexedDNElement
	var typeCounts = make(map[string]int)
	dnElements := d.Get("subject").(*schema.Set)
	for _, dnElement := range dnElements.List() {
		dn := dnElement.(map[string]interface{})
		typeCounts[dn["type"].(string)]++
		subject = append(subject, oldr.IndexedDNElement{
			Element: fmt.Sprintf("%s.%d", strings.ToLower(dn["type"].(string)), typeCounts[dn["type"].(string)]),
			Type:    dn["type"].(string),
			Value:   fmt.Sprintf("%v", dn["value"].(string)),
		})
	}
	return subject
}

func SetSans2(d *schema.ResourceData) []requests.IndexedSANElement {
	var sans []requests.IndexedSANElement
	sanElements := d.Get("sans").(*schema.Set)
	for _, sanElement := range sanElements.List() {
		san := sanElement.(map[string]interface{})
		new := true
		for _, indexedSan := range sans {
			if strings.EqualFold(indexedSan.Type, san["type"].(string)) {
				indexedSan.Value = append(indexedSan.Value, san["value"].(string))
				new = false
			}
		}
		if new {
			sans = append(sans, requests.IndexedSANElement{
				Type:  strings.ToUpper(san["type"].(string)),
				Value: []string{san["value"].(string)},
			})
		}
	}
	return sans
}

func SetSans(d *schema.ResourceData) []oldr.IndexedSANElement {
	var sans []oldr.IndexedSANElement
	typeCounts := make(map[string]int)
	sanElements := d.Get("sans").(*schema.Set)
	for _, sanElement := range sanElements.List() {
		san := sanElement.(map[string]interface{})
		typeCounts[san["type"].(string)]++
		sans = append(sans, oldr.IndexedSANElement{
			Element: fmt.Sprintf("%s.%d", strings.ToLower(san["type"].(string)), typeCounts[san["type"].(string)]),
			Type:    strings.ToUpper(san["type"].(string)),
			Value:   fmt.Sprintf("%v", san["value"].(string)),
		})
	}
	return sans
}

func SetLabels2(d *schema.ResourceData) []requests.LabelElement {
	var labels []requests.LabelElement
	labelElements := d.Get("labels").(*schema.Set)
	for _, labelElement := range labelElements.List() {
		label := labelElement.(map[string]interface{})
		labels = append(labels, requests.LabelElement{
			Label: label["label"].(string),
			Value: label["value"].(string),
		})
	}
	return labels
}

func SetLabels(d *schema.ResourceData) []oldr.LabelElement {
	var labels []oldr.LabelElement
	labelElements := d.Get("labels").(*schema.Set)
	for _, labelElement := range labelElements.List() {
		label := labelElement.(map[string]interface{})
		labels = append(labels, oldr.LabelElement{
			Label: label["label"].(string),
			Value: label["value"].(string),
		})
	}
	return labels
}
