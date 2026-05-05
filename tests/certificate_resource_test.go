package provider_test

import (
	"fmt"
	"os"
	"regexp"
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

func TestAccCertificate_ImportState(t *testing.T) {
	cfg := testAccCentralizedConfig("import-state.tf-test.internal", false, false)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "serial"),
				),
			},
			{
				ResourceName:      "horizon_certificate.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"pkcs12",
					"password",
					"pkcs12_write_only",
					"password_write_only",
					"profile",
					"key_type",
					"renew_before",
					"revoke_on_delete",
					"subject",
					"sans",
					"labels",
					"owner",
					"team",
					"contact_email",
					"wait_for_third_parties",
				},
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

// testAccCentralizedConfigWithTimeouts builds a centralized-enrollment config
// with an explicit timeouts.create value (Go duration string).
func testAccCentralizedConfigWithTimeouts(cn, createTimeout string) string {
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

  timeouts {
    create = %q
  }
}
`, testAccProfile(), cn, createTimeout)
}

// TestAccCertificate_Timeouts: configuring an explicit timeouts.create value
// must succeed, persist into state verbatim, and produce no drift on re-plan.
func TestAccCertificate_Timeouts(t *testing.T) {
	cfg := testAccCentralizedConfigWithTimeouts("timeouts.tf-test.internal", "10m")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					resource.TestCheckResourceAttr("horizon_certificate.test", "timeouts.create", "10m"),
				),
			},
			// No perpetual drift introduced by the timeouts block.
			{
				Config:             cfg,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccCertificate_TimeoutsInvalid: an unparseable Go duration must be
// rejected at plan time by the framework's duration validator (no API call
// reaches Horizon).
func TestAccCertificate_TimeoutsInvalid(t *testing.T) {
	cfg := testAccCentralizedConfigWithTimeouts("timeouts-invalid.tf-test.internal", "not-a-duration")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`(?i)duration`),
			},
		},
	})
}
