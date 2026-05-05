package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Compile-time assertions that the data source satisfies the framework
// interfaces it advertises (Configure + ValidateConfig).
var (
	_ datasource.DataSource                     = &CertificateTrustChainDataSource{}
	_ datasource.DataSourceWithConfigure        = &CertificateTrustChainDataSource{}
	_ datasource.DataSourceWithValidateConfig   = &CertificateTrustChainDataSource{}
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
	const pemA = "-----BEGIN CERTIFICATE-----\nAAA\n-----END CERTIFICATE-----\n"
	const pemB = "-----BEGIN CERTIFICATE-----\nBBB\n-----END CERTIFICATE-----\n"

	t.Run("deterministic for the same inputs", func(t *testing.T) {
		first := trustChainID(pemA, "leaf_to_root")
		second := trustChainID(pemA, "leaf_to_root")
		if first != second {
			t.Fatalf("trustChainID is not deterministic: %q vs %q", first, second)
		}
	})

	t.Run("changes when the certificate changes", func(t *testing.T) {
		idA := trustChainID(pemA, "leaf_to_root")
		idB := trustChainID(pemB, "leaf_to_root")
		if idA == idB {
			t.Fatalf("trustChainID must change with the certificate content (got %q for both)", idA)
		}
	})

	t.Run("changes when the order changes", func(t *testing.T) {
		idLTR := trustChainID(pemA, "leaf_to_root")
		idRTL := trustChainID(pemA, "root_to_leaf")
		if idLTR == idRTL {
			t.Fatalf("trustChainID must change with the order (got %q for both)", idLTR)
		}
	})

	t.Run("not vulnerable to pem/order concatenation collisions", func(t *testing.T) {
		// Without the separator byte, "ltr" + "AAA" == "l" + "trAAA" would
		// collide. Use the same total content split differently to make sure
		// the separator is taken into account.
		idA := trustChainID("trAAA", "l")
		idB := trustChainID("AAA", "ltr")
		if idA == idB {
			t.Fatalf("trustChainID must use a separator between order and pem (got %q for both)", idA)
		}
	})

	t.Run("sha256-hex length", func(t *testing.T) {
		if got := len(trustChainID(pemA, "leaf_to_root")); got != 64 {
			t.Fatalf("trustChainID must be a 64-char sha256 hex string, got %d chars", got)
		}
	})
}
