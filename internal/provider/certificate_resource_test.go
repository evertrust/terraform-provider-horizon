package provider

import (
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

func TestCertificateTestSuite(t *testing.T) {
	suite.Run(t, new(CertificateSuite))
}
