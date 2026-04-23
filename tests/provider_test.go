package provider_test

import (
	"os"
	"testing"

	"github.com/evertrust/terraform-provider-horizon/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"horizon": providerserver.NewProtocol6WithError(provider.New("test")()),
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, env := range []string{"HORIZON_ENDPOINT", "HORIZON_USERNAME", "HORIZON_PASSWORD", "HORIZON_PROFILE"} {
		if os.Getenv(env) == "" {
			t.Fatalf("acceptance tests require %s to be set", env)
		}
	}
}

func testAccPreCheckDecentralized(t *testing.T) {
	t.Helper()
	for _, env := range []string{"HORIZON_ENDPOINT", "HORIZON_USERNAME", "HORIZON_PASSWORD", "HORIZON_DECENTRALIZED_PROFILE"} {
		if os.Getenv(env) == "" {
			t.Fatalf("acceptance tests require %s to be set", env)
		}
	}
}

func testAccProviderConfig() string {
	return `
provider "horizon" {
  endpoint = "` + os.Getenv("HORIZON_ENDPOINT") + `"
  username = "` + os.Getenv("HORIZON_USERNAME") + `"
  password = "` + os.Getenv("HORIZON_PASSWORD") + `"
}
`
}
