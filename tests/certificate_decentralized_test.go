package provider_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// testAccDecentralizedProfile returns the Horizon decentralized enrollment profile from the environment.
func testAccDecentralizedProfile() string {
	return os.Getenv("HORIZON_DECENTRALIZED_PROFILE")
}

// generateCSR generates an RSA-2048 CSR in PEM format for the given CN.
func generateCSR(t *testing.T, cn string) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: cn},
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		t.Fatalf("failed to create CSR: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))
}

// testAccDecentralizedConfig builds a decentralized-enrollment config from a PEM CSR.
func testAccDecentralizedConfig(csr string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "horizon_certificate" "test" {
  profile = %q
  csr     = %q
}
`, testAccDecentralizedProfile(), csr)
}

// TestAccCertificate_Decentralized_Basic: enrollment with a CSR — certificate must be present,
// pkcs12 and password must be absent from state.
func TestAccCertificate_Decentralized_Basic(t *testing.T) {
	csr := generateCSR(t, "decentralized-basic.tf-test.internal")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckDecentralized(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDecentralizedConfig(csr),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "thumbprint"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "certificate"),
					// No PKCS12 / password in decentralized mode
					resource.TestCheckNoResourceAttr("horizon_certificate.test", "pkcs12"),
					resource.TestCheckNoResourceAttr("horizon_certificate.test", "password"),
				),
			},
		},
	})
}

// TestAccCertificate_Decentralized_NoDrift: after applying with a CSR, a second plan must be empty.
func TestAccCertificate_Decentralized_NoDrift(t *testing.T) {
	csr := generateCSR(t, "decentralized-no-drift.tf-test.internal")
	cfg := testAccDecentralizedConfig(csr)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckDecentralized(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "id"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "certificate"),
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

// TestAccCertificate_Decentralized_Metadata: verify that computed metadata is populated
// (dn, serial, issuer, not_before, not_after, key_type).
func TestAccCertificate_Decentralized_Metadata(t *testing.T) {
	csr := generateCSR(t, "decentralized-metadata.tf-test.internal")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckDecentralized(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDecentralizedConfig(csr),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "dn"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "serial"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "issuer"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "not_before"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "not_after"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "key_type"),
					resource.TestCheckResourceAttrSet("horizon_certificate.test", "signing_algorithm"),
				),
			},
		},
	})
}
