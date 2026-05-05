package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Compile-time assertions that the data source satisfies the framework
// interfaces it advertises (Configure + ValidateConfig).
var (
	_ datasource.DataSource                   = &CertificateTrustChainDataSource{}
	_ datasource.DataSourceWithConfigure      = &CertificateTrustChainDataSource{}
	_ datasource.DataSourceWithValidateConfig = &CertificateTrustChainDataSource{}
)

func TestOrderToHorizon(t *testing.T) {
	want := map[string]string{
		"leaf_to_root":        "ltr",
		"root_to_leaf":        "rtl",
		"issuer_leaf_to_root": "iltr",
		"issuer_root_to_leaf": "irtl",
	}
	if len(orderToHorizon) != len(want) {
		t.Fatalf("orderToHorizon must expose exactly %d entries, got %d", len(want), len(orderToHorizon))
	}
	for in, expected := range want {
		got, ok := orderToHorizon[in]
		if !ok {
			t.Errorf("orderToHorizon missing entry for %q", in)
			continue
		}
		if got != expected {
			t.Errorf("orderToHorizon[%q] = %q, want %q", in, got, expected)
		}
	}
}

func TestTrustChainID(t *testing.T) {
	const leaf = "-----BEGIN CERTIFICATE-----\nLEAF\n-----END CERTIFICATE-----"
	const root = "-----BEGIN CERTIFICATE-----\nROOT\n-----END CERTIFICATE-----"
	leafToRoot := leaf + "\n" + root
	rootToLeaf := root + "\n" + leaf

	t.Run("matches sha256(chain_pem) of the chain", func(t *testing.T) {
		want := sha256.Sum256([]byte(leafToRoot))
		got := trustChainID(leafToRoot)
		if got != hex.EncodeToString(want[:]) {
			t.Fatalf("trustChainID = %s, want %s", got, hex.EncodeToString(want[:]))
		}
	})

	t.Run("deterministic for the same chain", func(t *testing.T) {
		first := trustChainID(leafToRoot)
		second := trustChainID(leafToRoot)
		if first != second {
			t.Fatalf("trustChainID is not deterministic: %q vs %q", first, second)
		}
	})

	t.Run("changes when chain order changes", func(t *testing.T) {
		// Same certificates, reversed order: the id MUST differ so callers can
		// distinguish leaf_to_root and root_to_leaf reads on the same input.
		idLTR := trustChainID(leafToRoot)
		idRTL := trustChainID(rootToLeaf)
		if idLTR == idRTL {
			t.Fatalf("trustChainID must change with chain order (got %q for both)", idLTR)
		}
	})

	t.Run("changes when the chain content changes", func(t *testing.T) {
		other := "-----BEGIN CERTIFICATE-----\nOTHER\n-----END CERTIFICATE-----"
		idA := trustChainID(leafToRoot)
		idB := trustChainID(leaf + "\n" + other)
		if idA == idB {
			t.Fatalf("trustChainID must change with chain content (got %q for both)", idA)
		}
	})

	t.Run("sha256-hex length", func(t *testing.T) {
		if got := len(trustChainID(leafToRoot)); got != 64 {
			t.Fatalf("trustChainID must be a 64-char sha256 hex string, got %d chars", got)
		}
	})

	t.Run("empty chain still produces a stable digest", func(t *testing.T) {
		want := sha256.Sum256(nil)
		if got := trustChainID(""); got != hex.EncodeToString(want[:]) {
			t.Fatalf("trustChainID(\"\") = %s, want %s", got, hex.EncodeToString(want[:]))
		}
	})
}
