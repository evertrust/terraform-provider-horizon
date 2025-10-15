package provider

import (
	"fmt"
	"regexp"
	"testing"

	horizontypes "github.com/evertrust/horizon-go/types"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type Credentials struct {
	Username string
	Password string
}
type CertificateSuite struct {
	suite.Suite
	HorizonEndpoint string
	Credentials     Credentials
	Provider        schema.Provider
	ProfileName     string
}

func (suite *CertificateSuite) SetupTest() {
	creds := Credentials{
		Username: "test",
		Password: "test",
	}
	suite.HorizonEndpoint = "http://localhost:9000"
	suite.Credentials = creds
	suite.ProfileName = "webra-profile"

}

func NewProvider(version string) provider.Provider {
	prov := New(version)
	return prov()
}

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"horizon": func() (tfprotov6.ProviderServer, error) {
		return providerserver.NewProtocol6WithError(NewProvider("0.3.1"))()
	},
}

func (s *CertificateSuite) TestAcc_Certificate_CREATE() {
	t := s.T()

	resourceName := "horizon_certificate.test"

	badProfile, err := uuid.GenerateUUID()
	assert.NoError(t, err)
	cfgBadProfile := fmt.Sprintf(`
provider "horizon" {
  alias = "with-creds"
  endpoint = "%s"
  username = "%s"
  password = "%s"
}

resource "horizon_certificate" "test" {
  provider = horizon.with-creds
  profile   = "%s"
  key_type         = "rsa-2048"
  subject = [
    {
      element = "CN"
      type    = "CN"
      value   = "example.com"
    }
  ]
}
`, s.HorizonEndpoint, s.Credentials.Username, s.Credentials.Password, badProfile)

	badUsername, err := uuid.GenerateUUID()
	assert.NoError(t, err)
	badPassword, err := uuid.GenerateUUID()
	assert.NoError(t, err)
	cfgBadCreds := fmt.Sprintf(`
provider "horizon" {
  alias = "with-creds"
  endpoint = "%s"
  username = "%s"
  password = "%s"
}

resource "horizon_certificate" "test" {
  provider = horizon.with-creds
  profile   = "webra-profile"
  key_type         = "rsa-2048"
  subject = [
    {
      element = "CN"
      type    = "CN"
      value   = "example.com"
    }
  ]
}
`, s.HorizonEndpoint, badUsername, badPassword)

	cfgCreate := fmt.Sprintf(`
provider "horizon" {
  alias = "with-creds"
  endpoint = "%s"
  username = "%s"
  password = "%s"
}

resource "horizon_certificate" "test" {
  provider = horizon.with-creds
  profile          = "%s"
  key_type         = "rsa-2048"
  revoke_on_delete = true
  renew_before     = 30

  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = "creation-test.com"
    }
  ]
}
`, s.HorizonEndpoint, s.Credentials.Username, s.Credentials.Password, s.ProfileName)

	cfgCreate2 := fmt.Sprintf(`
provider "horizon" {
  alias = "with-creds"
  endpoint = "%s"
  username = "%s"
  password = "%s"
}

resource "horizon_certificate" "test" {
  provider = horizon.with-creds
  profile          = "%s"
  key_type         = "rsa-2048"
  revoke_on_delete = false
  subject = [
    {
      element = "cn.1"
      type    = "CN"
      value   = "creation-test-2.com"
    }
  ]
  sans = [
    {
      type  = "DNSNAME"
      value = ["creation-2-dns-example.com", "www.creation-2-dns-example.com"]
    }
  ]
}
`, s.HorizonEndpoint, s.Credentials.Username, s.Credentials.Password, s.ProfileName)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Bad creds should imply error
				Config:      cfgBadCreds,
				ExpectError: regexp.MustCompile("Horizon returned a SEC-AUTH-002 error: Invalid credentials or principal"),
			},
			{
				// Bad profile should imply error
				Config:      cfgBadProfile,
				ExpectError: regexp.MustCompile("Horizon returned a REQ-010 error: Profile does not exist or is disabled"),
			},
			{
				Config: cfgCreate,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "profile", s.ProfileName),
					resource.TestCheckResourceAttrSet(resourceName, "pkcs12"),
					resource.TestCheckResourceAttr(resourceName, "key_type", "rsa-2048"),
					resource.TestCheckResourceAttr(resourceName, "revoke_on_delete", "true"),
					resource.TestCheckResourceAttr(resourceName, "renew_before", "30"),
					resource.TestCheckResourceAttr(resourceName, "subject.0.element", "cn.1"),
					resource.TestCheckResourceAttr(resourceName, "subject.0.type", "CN"),
					resource.TestCheckResourceAttr(resourceName, "subject.0.value", "creation-test.com"),
				),
			},
			{
				Config: cfgCreate2,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "profile", s.ProfileName),
					resource.TestCheckResourceAttr(resourceName, "key_type", "rsa-2048"),
					resource.TestCheckResourceAttr(resourceName, "revoke_on_delete", "false"),
					resource.TestCheckNoResourceAttr(resourceName, "renew_before"),
					resource.TestCheckResourceAttr(resourceName, "subject.0.element", "cn.1"),
					resource.TestCheckResourceAttr(resourceName, "subject.0.type", "CN"),
					resource.TestCheckResourceAttr(resourceName, "subject.0.value", "update-test.com"),
					resource.TestCheckResourceAttr(resourceName, "sans.0.type", "DNSNAME"),
					resource.TestCheckResourceAttr(resourceName, "sans.0.value.0", "update-dns-example.com"),
					resource.TestCheckResourceAttr(resourceName, "sans.0.value.1", "www.update-dns-example.com"),
				),
			},
		},
	})
}

func (s *CertificateSuite) TestHasThirdParties_AllPresent() {
	// Test that if all the third parties provided by the user are contained in the polled certificate,
	// the check of third-parties return true, even if the certificate contains more third-parties

	t := s.T()

	cert := &horizontypes.Certificate{
		Id: "cert-1",
		ThirdPartyData: []horizontypes.ThirdPartyItem{
			{Connector: "third-party-1"},
			{Connector: "third-party-2"},
			{Connector: "third-party-3"},
		},
	}

	wanted := []string{"third-party-1", "third-party-2"}
	result := hasThirdParties(cert, wanted)
	assert.True(t, result)
}

func (s *CertificateSuite) TestHasThirdParties_MissingAnyReturnsFalse() {
	t := s.T()

	cert := &horizontypes.Certificate{
		Id: "cert-2",
		ThirdPartyData: []horizontypes.ThirdPartyItem{
			{Connector: "tp-1"},
			{Connector: "tp-2"},
		},
	}

	wanted := []string{"tp-1", "tp-3", "tp-2"} // tp-3 not contained in certificate thirdParties
	result := hasThirdParties(cert, wanted)
	assert.False(t, result)
}

func (s *CertificateSuite) TestHasThirdParties_EmptyWantedIsTrue() {
	t := s.T()

	cert := &horizontypes.Certificate{
		Id: "cert-3",
		ThirdPartyData: []horizontypes.ThirdPartyItem{
			{Connector: "tp-1"},
		},
	}
	var want []string
	result := hasThirdParties(cert, want)
	assert.True(t, result)
}

func (s *CertificateSuite) TestFillResourceFromCertificate_OK() {
	t := s.T()

	src := &horizontypes.Certificate{
		Id:                  "id-123",
		Certificate:         "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----",
		Thumbprint:          "THUMB",
		SelfSigned:          false,
		PublicKeyThumbprint: "PKTHUMB",
		Dn:                  "CN=example",
		Serial:              "01AB",
		Issuer:              "CN=issuer",
		NotBefore:           1111111111,
		NotAfter:            2222222222,
		RevocationDate:      0,
		KeyType:             "rsa-2048",
		SigningAlgorithm:    "SHA256WITHRSA",
	}

	var dst certificateResourceModel
	fillResourceFromCertificate(&dst, src)

	assert.Equal(t, dst.Id.ValueString(), src.Id)
	assert.Equal(t, dst.Certificate.ValueString(), src.Certificate)
	assert.Equal(t, dst.Thumbprint.ValueString(), src.Thumbprint)
	assert.Equal(t, dst.SelfSigned.ValueBool(), src.SelfSigned)
	assert.Equal(t, dst.PublicKeyThumbprint.ValueString(), src.PublicKeyThumbprint)
	assert.Equal(t, dst.Dn.ValueString(), src.Dn)
	assert.Equal(t, dst.Serial.ValueString(), src.Serial)
	assert.Equal(t, dst.Issuer.ValueString(), src.Issuer)
	assert.Equal(t, dst.NotBefore.ValueInt64(), int64(src.NotBefore))
	assert.Equal(t, dst.NotAfter.ValueInt64(), int64(src.NotAfter))
	assert.Equal(t, dst.RevocationDate.ValueInt64(), int64(src.RevocationDate))
	assert.Equal(t, dst.KeyType.ValueString(), src.KeyType)
	assert.Equal(t, dst.SigningAlgorithm.ValueString(), src.SigningAlgorithm)
}

func TestCertificateTestSuite(t *testing.T) {
	suite.Run(t, new(CertificateSuite))
}
