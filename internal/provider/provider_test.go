package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ provider.ProviderWithValidateConfig = &HorizonProvider{}

func rawConfig(t *testing.T, typ tftypes.Type, values map[string]tftypes.Value) tftypes.Value {
	t.Helper()
	objType, ok := typ.(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T, want tftypes.Object", typ)
	}
	attrs := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, attrType := range objType.AttributeTypes {
		attrs[name] = tftypes.NewValue(attrType, nil)
	}
	for name, value := range values {
		if _, ok := objType.AttributeTypes[name]; !ok {
			t.Fatalf("attribute %q is not part of the schema", name)
		}
		attrs[name] = value
	}
	return tftypes.NewValue(objType, attrs)
}

func str(v string) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }

func containsWarningSummary(diags diag.Diagnostics, want string) bool {
	for _, d := range diags.Warnings() {
		if d.Summary() == want {
			return true
		}
	}
	return false
}

func TestProviderValidateConfig(t *testing.T) {
	ctx := context.Background()
	p := &HorizonProvider{}

	var schemaResp provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &schemaResp)
	schemaType := schemaResp.Schema.Type().TerraformType(ctx)

	tests := []struct {
		name        string
		values      map[string]tftypes.Value
		wantError   string
		wantWarning string
	}{
		{
			name:   "username and password",
			values: map[string]tftypes.Value{"username": str("admin"), "password": str("secret")},
		},
		{
			name:   "client certificate and key",
			values: map[string]tftypes.Value{"client_cert_pem": str("cert"), "client_key_pem": str("key")},
		},
		{
			name:      "no authentication method",
			values:    map[string]tftypes.Value{"endpoint": str("https://horizon.example")},
			wantError: "No authentication method provided",
		},
		{
			name:      "username without password",
			values:    map[string]tftypes.Value{"username": str("admin")},
			wantError: "Password is required when username is provided.",
		},
		{
			name:      "password alone is not an authentication method",
			values:    map[string]tftypes.Value{"password": str("secret")},
			wantError: "No authentication method provided",
		},
		{
			name:      "username with client certificate",
			values:    map[string]tftypes.Value{"username": str("admin"), "password": str("secret"), "client_cert_pem": str("cert")},
			wantError: "Client certificate is not supported when username is provided.",
		},
		{
			name:      "username with client key",
			values:    map[string]tftypes.Value{"username": str("admin"), "password": str("secret"), "client_key_pem": str("key")},
			wantError: "Client key is not supported when username is provided.",
		},
		{
			name:      "client certificate without key",
			values:    map[string]tftypes.Value{"client_cert_pem": str("cert")},
			wantError: "client_key_pem is required when client_cert_pem is provided.",
		},
		{
			name:      "client certificate with password",
			values:    map[string]tftypes.Value{"client_cert_pem": str("cert"), "client_key_pem": str("key"), "password": str("secret")},
			wantError: "Password is not supported when client_cert_pem is provided.",
		},
		{
			name: "skip_tls_verify with ca_bundle_pem warns",
			values: map[string]tftypes.Value{
				"username":        str("admin"),
				"password":        str("secret"),
				"skip_tls_verify": tftypes.NewValue(tftypes.Bool, true),
				"ca_bundle_pem":   str("bundle"),
			},
			wantWarning: "skip_tls_verify is not recommended when ca_bundle_pem is provided.",
		},
		{
			name: "skip_tls_verify false with ca_bundle_pem does not warn",
			values: map[string]tftypes.Value{
				"username":        str("admin"),
				"password":        str("secret"),
				"skip_tls_verify": tftypes.NewValue(tftypes.Bool, false),
				"ca_bundle_pem":   str("bundle"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := provider.ValidateConfigRequest{
				Config: tfsdk.Config{Raw: rawConfig(t, schemaType, tt.values), Schema: schemaResp.Schema},
			}
			var resp provider.ValidateConfigResponse
			p.ValidateConfig(ctx, req, &resp)

			if tt.wantError == "" && resp.Diagnostics.HasError() {
				t.Fatalf("unexpected errors: %v", resp.Diagnostics.Errors())
			}
			if tt.wantError != "" && !containsErrorSummary(resp.Diagnostics, tt.wantError) {
				t.Fatalf("expected error %q, got %v", tt.wantError, resp.Diagnostics)
			}
			if tt.wantWarning == "" && resp.Diagnostics.WarningsCount() > 0 {
				t.Fatalf("unexpected warnings: %v", resp.Diagnostics.Warnings())
			}
			if tt.wantWarning != "" && !containsWarningSummary(resp.Diagnostics, tt.wantWarning) {
				t.Fatalf("expected warning %q, got %v", tt.wantWarning, resp.Diagnostics)
			}
		})
	}
}
