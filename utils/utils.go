package utils

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func FillCertificateSchema(
	d *schema.ResourceData,
	module string,
	profile string,
	owner string,
	certificate string,
	thumbprint string,
	selfSigned bool,
	publicKeyThumbprint string,
	dn string,
	serial string,
	issuer string,
	notBefore int,
	notAfter int,
	revocationDate int,
	revocationReason string,
	keyType string,
	signingAlgorithm string,
) {
	d.Set("module", module)
	d.Set("profile", profile)
	d.Set("owner", owner)
	d.Set("certificate", certificate)
	d.Set("thumbprint", thumbprint)
	d.Set("self_signed", selfSigned)
	d.Set("public_key_thumbprint", publicKeyThumbprint)
	d.Set("dn", dn)
	d.Set("serial", serial)
	d.Set("issuer", issuer)
	d.Set("not_before", notBefore)
	d.Set("not_after", notAfter)
	d.Set("revocation_date", revocationDate)
	d.Set("revocation_reason", revocationReason)
	d.Set("key_type", keyType)
	d.Set("signing_algorithm", signingAlgorithm)
}
