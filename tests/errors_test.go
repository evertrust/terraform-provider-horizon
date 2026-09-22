package provider_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const unknownCertificateID = "000000000000000000000000"

func testAccCentralizedConfigWithProfile(profile, cn string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "horizon_certificate" "test" {
  profile  = %q
  key_type = "rsa-2048"

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = %q
    }
  ]
}
`, profile, cn)
}

func TestAccCertificate_UnknownProfile(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCentralizedConfigWithProfile("tf-test-profile-that-does-not-exist", "unknown-profile.tf-test.internal"),
				ExpectError: regexp.MustCompile(`(?s)Failed to enroll certificate.*does\s+not\s+exist`),
			},
		},
	})
}

func TestAccCertificate_Decentralized_InvalidCSR(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckDecentralized(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccDecentralizedConfig("-----BEGIN CERTIFICATE REQUEST-----\nbm90IGEgY3Ny\n-----END CERTIFICATE REQUEST-----\n"),
				ExpectError: regexp.MustCompile(`Failed to get enroll template`),
			},
		},
	})
}

// A CSR sent to a centralized-only profile is an enrollment mode mismatch.
func TestAccCertificate_CSROnCentralizedProfile(t *testing.T) {
	csr := generateCSR(t, "csr-on-centralized.tf-test.internal")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + fmt.Sprintf(`
resource "horizon_certificate" "test" {
  profile = %q
  csr     = %q
}
`, testAccProfile(), csr),
				ExpectError: regexp.MustCompile(`(?s)Failed to enroll certificate.*Decentralized\s+enrollment\s+is\s+not\s+enabled`),
			},
		},
	})
}

func testAccRetrieveUnknownCertificateConfig(skipEscrowCheck bool) string {
	return testAccProviderConfig() + fmt.Sprintf(`
ephemeral "horizon_retrieve_centralized_pkcs12" "test" {
  certificate_id    = %q
  skip_escrow_check = %t
}

# Ephemeral resources are only opened when referenced.
check "opened" {
  assert {
    condition     = ephemeral.horizon_retrieve_centralized_pkcs12.test.pkcs12 == null
    error_message = "no material can exist for an unknown certificate"
  }
}
`, unknownCertificateID, skipEscrowCheck)
}

func TestAccRetrieveCentralizedPkcs12_UnknownCertificate(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccRetrieveUnknownCertificateConfig(false),
				ExpectError: regexp.MustCompile(`(?s)Failed to retrieve certificate.*404`),
			},
		},
	})
}

// skip_escrow_check is best-effort: an unknown certificate is not an error, the
// ephemeral resource opens with null values.
func TestAccRetrieveCentralizedPkcs12_UnknownCertificate_SkipEscrowCheck(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRetrieveUnknownCertificateConfig(true),
			},
		},
	})
}
