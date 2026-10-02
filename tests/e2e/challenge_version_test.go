//go:build e2e

// nolint
package tests

import "testing"

func TestSupportsWebRAChallenge(t *testing.T) {
	tests := map[string]bool{
		"2.7.21":     false,
		"2.10.7":     false,
		"2.11.0":     true,
		"2.11.0-rc1": true,
		"2.12.3":     true,
		"3.0.0":      true,
		"":           false,
		"latest":     false,
	}
	for version, want := range tests {
		t.Run(version, func(t *testing.T) {
			if got := supportsWebRAChallenge(version); got != want {
				t.Fatalf("supportsWebRAChallenge(%q) = %v, want %v", version, got, want)
			}
		})
	}
}
