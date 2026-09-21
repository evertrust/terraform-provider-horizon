package provider_test

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"golang.org/x/crypto/pkcs12"
)

func testAccChallengeProfile() string {
	return os.Getenv("HORIZON_CHALLENGE_PROFILE")
}

func testAccChallengeCertificate(name, profile, cn, challengeAttributes string) string {
	return fmt.Sprintf(`
resource "horizon_certificate" %q {
  profile  = %q
  key_type = "rsa-2048"
  %s

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = %q
    }
  ]
}
`, name, profile, challengeAttributes, cn)
}

func testAccCheckPkcs12OpensWithPassword(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("%s not found in state", resourceName)
		}
		p12, err := base64.StdEncoding.DecodeString(rs.Primary.Attributes["pkcs12"])
		if err != nil {
			return fmt.Errorf("pkcs12 is not base64: %w", err)
		}
		blocks, err := pkcs12.ToPEM(p12, rs.Primary.Attributes["password"])
		if err != nil {
			return fmt.Errorf("pkcs12 does not open with the password attribute: %w", err)
		}
		for _, block := range blocks {
			if block.Type == "PRIVATE KEY" {
				return nil
			}
		}
		return fmt.Errorf("pkcs12 holds no private key")
	}
}
func TestAccCertificate_Challenge_SingleUse(t *testing.T) {
	profile := testAccChallengeProfile()
	challenge := os.Getenv("HORIZON_ISSUED_CHALLENGE")
	if profile == "" || challenge == "" {
		t.Skip("HORIZON_CHALLENGE_PROFILE / HORIZON_ISSUED_CHALLENGE not set; WebRA Challenge mode needs Horizon 2.11+")
	}
	challengeAttribute := fmt.Sprintf("challenge = %q", challenge)
	first := testAccProviderConfig() + testAccChallengeCertificate("test", profile, "challenge-single-use.tf-test.internal", challengeAttribute)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: first,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					resource.TestCheckResourceAttr("horizon_certificate.test", "dn", "CN=challenge-single-use.tf-test.internal"),
					// Horizon encrypts the PKCS#12 with the challenge.
					resource.TestCheckResourceAttr("horizon_certificate.test", "password", challenge),
					testAccCheckPkcs12OpensWithPassword("horizon_certificate.test"),
				),
			},
			{
				Config:      first + testAccChallengeCertificate("replay", profile, "challenge-replay.tf-test.internal", challengeAttribute),
				ExpectError: regexp.MustCompile(`(?s)Failed to enroll certificate with the challenge.*Invalid\s+challenge`),
			},
		},
	})
}

func TestAccCertificate_RequestChallenge_Pkcs12(t *testing.T) {
	profile := testAccChallengeProfile()
	if profile == "" {
		t.Skip("HORIZON_CHALLENGE_PROFILE not set; WebRA Challenge mode needs Horizon 2.11+")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + testAccChallengeCertificate("test", profile, "challenge-requested-p12.tf-test.internal", "request_challenge = true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					testAccCheckPkcs12OpensWithPassword("horizon_certificate.test"),
				),
			},
		},
	})
}

func TestAccCertificate_Challenge_Invalid(t *testing.T) {
	profile := testAccChallengeProfile()
	if profile == "" {
		t.Skip("HORIZON_CHALLENGE_PROFILE not set; WebRA Challenge mode needs Horizon 2.11+")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccProviderConfig() + testAccChallengeCertificate("test", profile, "challenge-invalid.tf-test.internal", `challenge = "not-a-valid-challenge"`),
				ExpectError: regexp.MustCompile(`(?s)Failed to enroll certificate with the challenge.*Invalid\s+challenge`),
			},
		},
	})
}

func TestAccCertificate_Challenge_UnsupportedHorizon(t *testing.T) {
	if testAccChallengeProfile() != "" {
		t.Skip("this Horizon supports WebRA Challenge mode")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccProviderConfig() + testAccChallengeCertificate("test", testAccProfile(), "challenge-unsupported.tf-test.internal", `challenge = "any-challenge"`),
				ExpectError: regexp.MustCompile(`(?s)Failed to enroll certificate with the challenge.*requires\s+Horizon\s+2\.11`),
			},
		},
	})
}

func TestAccCertificate_Challenge_InvalidConfig(t *testing.T) {
	tests := map[string]struct {
		attributes string
		want       string
	}{
		"challenge and request_challenge": {
			attributes: "challenge = \"otp\"\n  request_challenge = true",
			want:       `request_challenge conflicts with challenge`,
		},
		"password": {
			attributes: "challenge = \"otp\"\n  password = \"secret\"",
			want:       `password cannot be set when enrolling with a challenge`,
		},
		"pkcs12_write_only": {
			attributes: "request_challenge = true\n  pkcs12_write_only = true",
			want:       `pkcs12_write_only would lose the private key`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:      testAccProviderConfig() + testAccChallengeCertificate("test", testAccProfile(), "challenge-config.tf-test.internal", tt.attributes),
						ExpectError: regexp.MustCompile(tt.want),
					},
				},
			})
		})
	}
}
