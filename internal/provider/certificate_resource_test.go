package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestValidateWriteOnlyFlags(t *testing.T) {
	tests := []struct {
		name              string
		pkcs12WriteOnly   types.Bool
		passwordWriteOnly types.Bool
		wantErrs          int
		wantAttrs         []string
	}{
		{
			name:              "both known: no error",
			pkcs12WriteOnly:   types.BoolValue(true),
			passwordWriteOnly: types.BoolValue(false),
			wantErrs:          0,
		},
		{
			name:              "both null: no error",
			pkcs12WriteOnly:   types.BoolNull(),
			passwordWriteOnly: types.BoolNull(),
			wantErrs:          0,
		},
		{
			name:              "pkcs12 unknown: error on pkcs12_write_only",
			pkcs12WriteOnly:   types.BoolUnknown(),
			passwordWriteOnly: types.BoolValue(false),
			wantErrs:          1,
			wantAttrs:         []string{"pkcs12_write_only"},
		},
		{
			name:              "password unknown: error on password_write_only",
			pkcs12WriteOnly:   types.BoolValue(false),
			passwordWriteOnly: types.BoolUnknown(),
			wantErrs:          1,
			wantAttrs:         []string{"password_write_only"},
		},
		{
			name:              "both unknown: error on both",
			pkcs12WriteOnly:   types.BoolUnknown(),
			passwordWriteOnly: types.BoolUnknown(),
			wantErrs:          2,
			wantAttrs:         []string{"pkcs12_write_only", "password_write_only"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := certificateResourceModel{
				Pkcs12WriteOnly:   tt.pkcs12WriteOnly,
				PasswordWriteOnly: tt.passwordWriteOnly,
			}
			diags := validateWriteOnlyFlags(data)
			if got := diags.ErrorsCount(); got != tt.wantErrs {
				t.Fatalf("got %d errors, want %d: %v", got, tt.wantErrs, diags)
			}
			for _, want := range tt.wantAttrs {
				found := false
				for _, d := range diags.Errors() {
					if d.Summary() == want+" must be known at plan time" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing error for attribute %q in %v", want, diags)
				}
			}
		})
	}
}
