package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// profile returns the Horizon enrollment profile from the environment.
func testAccProfile() string {
	return os.Getenv("HORIZON_PROFILE")
}

// testAccCentralizedConfig builds a centralized-enrollment config with optional write-only flags.
func testAccCentralizedConfig(cn string, pkcs12WriteOnly, passwordWriteOnly bool) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "horizon_certificate" "test" {
  profile             = %q
  key_type            = "rsa-2048"
  pkcs12_write_only   = %t
  password_write_only = %t

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = %q
    }
  ]
}
`, testAccProfile(), pkcs12WriteOnly, passwordWriteOnly, cn)
}

// TestAccCertificate_WriteOnlyBoth: both flags true → pkcs12 and password must be absent from state.
func TestAccCertificate_WriteOnlyBoth(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCentralizedConfig("write-only-both.tf-test.internal", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					// certificate metadata must be present
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "thumbprint"),
					// both secrets must be absent from state
					resource.TestCheckNoResourceAttr("horizon_certificate.test", "pkcs12"),
					resource.TestCheckNoResourceAttr("horizon_certificate.test", "password"),
				),
			},
			// Re-plan: no perpetual drift
			{
				Config:             testAccCentralizedConfig("write-only-both.tf-test.internal", true, true),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccCertificate_WriteOnlyPkcs12: only pkcs12 write-only → pkcs12 absent, password retained.
func TestAccCertificate_WriteOnlyPkcs12(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCentralizedConfig("write-only-pkcs12.tf-test.internal", true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					resource.TestCheckNoResourceAttr("horizon_certificate.test", "pkcs12"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "password"),
				),
			},
			{
				Config:             testAccCentralizedConfig("write-only-pkcs12.tf-test.internal", true, false),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccCertificate_WriteOnlyPassword: only password write-only → password absent, pkcs12 retained.
func TestAccCertificate_WriteOnlyPassword(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCentralizedConfig("write-only-password.tf-test.internal", false, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "pkcs12"),
					resource.TestCheckNoResourceAttr("horizon_certificate.test", "password"),
				),
			},
			{
				Config:             testAccCentralizedConfig("write-only-password.tf-test.internal", false, true),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccCertificate_DefaultBehavior: no flags → both secrets retained (backward compatibility).
func TestAccCertificate_DefaultBehavior(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCentralizedConfig("default-behavior.tf-test.internal", false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "pkcs12"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "password"),
				),
			},
			{
				Config:             testAccCentralizedConfig("default-behavior.tf-test.internal", false, false),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccCertificate_NoDriftAfterWriteOnly: after applying with both write-only flags enabled,
// two consecutive plans must produce no changes (null secrets must not cause perpetual drift).
func TestAccCertificate_NoDriftAfterWriteOnly(t *testing.T) {
	cfg := testAccCentralizedConfig("no-drift.tf-test.internal", true, true)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					resource.TestCheckNoResourceAttr("horizon_certificate.test", "pkcs12"),
					resource.TestCheckNoResourceAttr("horizon_certificate.test", "password"),
				),
			},
			// First plan after apply — must be empty.
			{
				Config:             cfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
			// Second plan — still no changes.
			{
				Config:             cfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}
