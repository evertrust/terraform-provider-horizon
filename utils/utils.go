package utils

import (
	"github.com/evertrust/horizon-go/certificates"
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
