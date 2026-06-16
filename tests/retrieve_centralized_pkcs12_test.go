package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// testAccEscrowProfile returns the optional escrow-enabled profile used by the
// happy-path test. When unset, the happy-path test is skipped because Horizon
// can only recover PKCS#12 material for certificates whose key was escrowed.
func testAccEscrowProfile() string {
	return os.Getenv("HORIZON_ESCROW_PROFILE")
}

// testAccRetrievePkcs12Config builds a config that enrolls a centralized
// certificate and retrieves its PKCS#12 material through the ephemeral resource.
// A check block references the ephemeral result so Terraform opens the ephemeral
// resource during apply (ephemeral resources are only opened when referenced).
func testAccRetrievePkcs12Config(profile, cn string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "horizon_certificate" "test" {
  profile             = %q
  key_type            = "rsa-2048"
  pkcs12_write_only   = true
  password_write_only = true

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = %q
    }
  ]
}

ephemeral "horizon_retrieve_centralized_pkcs12" "test" {
  certificate_id = horizon_certificate.test.id
}

check "pkcs12_present" {
  assert {
    condition = (
      ephemeral.horizon_retrieve_centralized_pkcs12.test.pkcs12 != "" &&
      ephemeral.horizon_retrieve_centralized_pkcs12.test.password != ""
    )
    error_message = "pkcs12 and password must not be empty"
  }
}
`, profile, cn)
}

// The non-escrow error path is covered by the always-on unit test
// TestResolvePkcs12 (case "non-escrowed certificate returns an escrow
// diagnostic"); it has no dedicated acceptance test because the seeded e2e
// profile is escrow-enabled and the escrow error cannot be provoked against it.

// TestAccRetrieveCentralizedPkcs12_Escrowed exercises the happy path against an
// escrow-enabled profile. It is skipped unless HORIZON_ESCROW_PROFILE is set,
// because the default seeded profiles do not escrow keys. The provider returns a
// hard error when no material is produced, so a successful apply proves the
// ephemeral resource returned a non-empty PKCS#12 bundle and password. Those
// values are ephemeral and never persist to state.
func TestAccRetrieveCentralizedPkcs12_Escrowed(t *testing.T) {
	profile := testAccEscrowProfile()
	if profile == "" {
		t.Skip("HORIZON_ESCROW_PROFILE not set; skipping escrow-based PKCS#12 retrieval test")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRetrievePkcs12Config(profile, "retrieve-escrowed.tf-test.internal"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					// The ephemeral material is never written to state.
					resource.TestCheckNoResourceAttr("horizon_certificate.test", "pkcs12"),
					resource.TestCheckNoResourceAttr("horizon_certificate.test", "password"),
				),
			},
		},
	})
}
