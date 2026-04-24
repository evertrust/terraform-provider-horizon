//go:build e2e

// nolint
package tests

// Credentials and profile names come from the Horizon DB dump under
// tests/e2e/resources/horizon_conf/. Updating the dump requires updating these.
const (
	AdminUsername = "administrator"
	AdminPassword = "kVQF8XWPbrSTSx32QFhZUMbU" //nolint:gosec

	CentralizedProfile   = "webra-centralized"
	DecentralizedProfile = "webra-decentralized"
)
