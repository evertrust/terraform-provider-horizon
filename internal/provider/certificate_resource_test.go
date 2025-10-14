package provider

import (
	"fmt"
	"testing"

	horizontypes "github.com/evertrust/horizon-go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type CertificateSuite struct {
	suite.Suite
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
	fmt.Println(dst)

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
