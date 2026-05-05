package provider

import (
	"testing"
	"time"

	"github.com/evertrust/horizon-go/v2/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &CertificateResource{}
	_ resource.ResourceWithImportState    = &CertificateResource{}
	_ resource.ResourceWithModifyPlan     = &CertificateResource{}
	_ resource.ResourceWithValidateConfig = &CertificateResource{}
)

func TestRenewalTriggerFor(t *testing.T) {
	tests := []struct {
		name     string
		notAfter int64
		want     string
	}{
		{name: "zero", notAfter: 0, want: "renew-0"},
		{name: "typical UnixMilli", notAfter: 1_808_557_186_000, want: "renew-1808557186000"},
		{name: "negative (sentinel)", notAfter: -1, want: "renew--1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := renewalTriggerFor(tt.notAfter); got != tt.want {
				t.Fatalf("renewalTriggerFor(%d) = %q, want %q", tt.notAfter, got, tt.want)
			}
		})
	}
}

func TestIsInRenewalWindow(t *testing.T) {
	// Reference "now" anchors the window: renewalDate = notAfter - renewBeforeDays.
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	day := (24 * time.Hour).Milliseconds()

	tests := []struct {
		name            string
		notAfter        types.Int64
		renewBeforeDays types.Int64
		want            bool
	}{
		{
			name:            "renew_before null → not in window",
			notAfter:        types.Int64Value(now.UnixMilli() + 30*day),
			renewBeforeDays: types.Int64Null(),
			want:            false,
		},
		{
			name:            "renew_before unknown → not in window",
			notAfter:        types.Int64Value(now.UnixMilli() + 30*day),
			renewBeforeDays: types.Int64Unknown(),
			want:            false,
		},
		{
			name:            "renew_before = 0 → not in window",
			notAfter:        types.Int64Value(now.UnixMilli() + 30*day),
			renewBeforeDays: types.Int64Value(0),
			want:            false,
		},
		{
			name:            "renew_before < 0 → not in window",
			notAfter:        types.Int64Value(now.UnixMilli() + 30*day),
			renewBeforeDays: types.Int64Value(-5),
			want:            false,
		},
		{
			name:            "not_after null → not in window",
			notAfter:        types.Int64Null(),
			renewBeforeDays: types.Int64Value(30),
			want:            false,
		},
		{
			name:            "not_after unknown → not in window",
			notAfter:        types.Int64Unknown(),
			renewBeforeDays: types.Int64Value(30),
			want:            false,
		},
		{
			name:            "not_after = 0 → not in window",
			notAfter:        types.Int64Value(0),
			renewBeforeDays: types.Int64Value(30),
			want:            false,
		},
		{
			name:            "expires in 60 days, renew_before 30 → outside window",
			notAfter:        types.Int64Value(now.UnixMilli() + 60*day),
			renewBeforeDays: types.Int64Value(30),
			want:            false,
		},
		{
			name:            "expires in 30 days, renew_before 30 → exactly at boundary (exclusive)",
			notAfter:        types.Int64Value(now.UnixMilli() + 30*day),
			renewBeforeDays: types.Int64Value(30),
			want:            false,
		},
		{
			name:            "1ms past boundary (29d 23h 59m 59.999s remaining), renew_before 30 → inside window",
			notAfter:        types.Int64Value(now.UnixMilli() + 30*day - 1),
			renewBeforeDays: types.Int64Value(30),
			want:            true,
		},
		{
			name:            "expires in 15 days, renew_before 30 → inside window",
			notAfter:        types.Int64Value(now.UnixMilli() + 15*day),
			renewBeforeDays: types.Int64Value(30),
			want:            true,
		},
		{
			name:            "already expired, renew_before 30 → inside window",
			notAfter:        types.Int64Value(now.UnixMilli() - 1*day),
			renewBeforeDays: types.Int64Value(30),
			want:            true,
		},
		{
			name:            "1-year cert, renew_before 400 → always inside window (real renew_test scenario)",
			notAfter:        types.Int64Value(now.UnixMilli() + 365*day),
			renewBeforeDays: types.Int64Value(400),
			want:            true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInRenewalWindow(tt.notAfter, tt.renewBeforeDays, now); got != tt.want {
				t.Fatalf("isInRenewalWindow(notAfter=%v, renewBeforeDays=%v, now=%s) = %v, want %v",
					tt.notAfter, tt.renewBeforeDays, now.Format(time.RFC3339), got, tt.want)
			}
		})
	}
}

func TestExtractRenewedCertificate(t *testing.T) {
	// A minimal valid Certificate inside a renew response.
	validCert := &models.Certificate{Id: "cert-42"}

	renewedWithCert := models.NewWebRARenewRequestOnSubmitResponseWithDefaults()
	renewedWithCert.Certificate.Set(validCert)

	renewedWithoutCert := models.NewWebRARenewRequestOnSubmitResponseWithDefaults()
	// Certificate left unset (NullableCertificate.Get() returns nil).

	tests := []struct {
		name        string
		resp        *models.RequestSubmit201Response
		wantErr     bool
		wantSummary string // substring match on the error summary
	}{
		{
			name:        "nil response → empty renew error",
			resp:        nil,
			wantErr:     true,
			wantSummary: "Empty renew response",
		},
		{
			name:        "response without WebRARenewRequestOnSubmitResponse → unexpected type",
			resp:        &models.RequestSubmit201Response{},
			wantErr:     true,
			wantSummary: "Unexpected response type",
		},
		{
			name: "renew response without certificate → missing certificate",
			resp: &models.RequestSubmit201Response{
				WebRARenewRequestOnSubmitResponse: renewedWithoutCert,
			},
			wantErr:     true,
			wantSummary: "Missing certificate in renew response",
		},
		{
			name: "valid renew response → no diagnostic, certificate returned",
			resp: &models.RequestSubmit201Response{
				WebRARenewRequestOnSubmitResponse: renewedWithCert,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := extractRenewedCertificate(tt.resp)
			if diags.HasError() != tt.wantErr {
				t.Fatalf("diags.HasError() = %v, want %v (diags: %v)", diags.HasError(), tt.wantErr, diags)
			}
			if tt.wantErr {
				if got != nil {
					t.Errorf("expected nil response on error, got %+v", got)
				}
				if !containsErrorSummary(diags, tt.wantSummary) {
					t.Errorf("expected error summary containing %q, got %v", tt.wantSummary, diags)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected non-nil response on success")
			}
			if got.Certificate.Get() == nil || got.Certificate.Get().Id != "cert-42" {
				t.Errorf("expected embedded certificate id 'cert-42', got %+v", got.Certificate.Get())
			}
		})
	}
}

func TestExtractUpdatedCertificate(t *testing.T) {
	validCert := &models.Certificate{Id: "cert-42"}

	updatedWithCert := models.NewWebRAUpdateRequestOnSubmitResponseWithDefaults()
	updatedWithCert.Certificate.Set(validCert)

	updatedWithoutCert := models.NewWebRAUpdateRequestOnSubmitResponseWithDefaults()

	tests := []struct {
		name        string
		resp        *models.RequestSubmit201Response
		wantErr     bool
		wantSummary string
	}{
		{
			name:        "nil response → empty update error",
			resp:        nil,
			wantErr:     true,
			wantSummary: "Empty update response",
		},
		{
			name:        "response without WebRAUpdateRequestOnSubmitResponse → unexpected type",
			resp:        &models.RequestSubmit201Response{},
			wantErr:     true,
			wantSummary: "Unexpected response type",
		},
		{
			name: "update response without certificate → missing certificate",
			resp: &models.RequestSubmit201Response{
				WebRAUpdateRequestOnSubmitResponse: updatedWithoutCert,
			},
			wantErr:     true,
			wantSummary: "Missing certificate in update response",
		},
		{
			name: "valid update response → no diagnostic, certificate returned",
			resp: &models.RequestSubmit201Response{
				WebRAUpdateRequestOnSubmitResponse: updatedWithCert,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diags := extractUpdatedCertificate(tt.resp)
			if diags.HasError() != tt.wantErr {
				t.Fatalf("diags.HasError() = %v, want %v (diags: %v)", diags.HasError(), tt.wantErr, diags)
			}
			if tt.wantErr {
				if got != nil {
					t.Errorf("expected nil certificate on error, got %+v", got)
				}
				if !containsErrorSummary(diags, tt.wantSummary) {
					t.Errorf("expected error summary containing %q, got %v", tt.wantSummary, diags)
				}
				return
			}
			if got == nil || got.Id != "cert-42" {
				t.Errorf("expected certificate id 'cert-42', got %+v", got)
			}
		})
	}
}

func containsErrorSummary(diags diag.Diagnostics, want string) bool {
	for _, e := range diags.Errors() {
		if e.Summary() == want {
			return true
		}
	}
	return false
}

func TestMetadataChanged(t *testing.T) {
	labelObjType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"label": types.StringType,
		"value": types.StringType,
	}}
	labelSet := func(pairs ...[2]string) types.Set {
		elements := make([]attr.Value, 0, len(pairs))
		for _, p := range pairs {
			elements = append(elements, types.ObjectValueMust(labelObjType.AttrTypes, map[string]attr.Value{
				"label": types.StringValue(p[0]),
				"value": types.StringValue(p[1]),
			}))
		}
		return types.SetValueMust(labelObjType, elements)
	}

	tests := []struct {
		name  string
		plan  certificateResourceModel
		prior certificateResourceModel
		want  bool
	}{
		{
			name: "all metadata equal → unchanged",
			plan: certificateResourceModel{
				Owner:        types.StringValue("alice"),
				Team:         types.StringValue("platform"),
				ContactEmail: types.StringValue("alice@example.com"),
				Labels:       labelSet([2]string{"env", "prod"}),
			},
			prior: certificateResourceModel{
				Owner:        types.StringValue("alice"),
				Team:         types.StringValue("platform"),
				ContactEmail: types.StringValue("alice@example.com"),
				Labels:       labelSet([2]string{"env", "prod"}),
			},
			want: false,
		},
		{
			name: "owner changed",
			plan: certificateResourceModel{
				Owner:  types.StringValue("bob"),
				Labels: labelSet(),
			},
			prior: certificateResourceModel{
				Owner:  types.StringValue("alice"),
				Labels: labelSet(),
			},
			want: true,
		},
		{
			name: "team changed",
			plan: certificateResourceModel{
				Team:   types.StringValue("security"),
				Labels: labelSet(),
			},
			prior: certificateResourceModel{
				Team:   types.StringValue("platform"),
				Labels: labelSet(),
			},
			want: true,
		},
		{
			name: "contact_email changed",
			plan: certificateResourceModel{
				ContactEmail: types.StringValue("bob@example.com"),
				Labels:       labelSet(),
			},
			prior: certificateResourceModel{
				ContactEmail: types.StringValue("alice@example.com"),
				Labels:       labelSet(),
			},
			want: true,
		},
		{
			name: "owner cleared (set in state, null in plan)",
			plan: certificateResourceModel{
				Owner:  types.StringNull(),
				Labels: labelSet(),
			},
			prior: certificateResourceModel{
				Owner:  types.StringValue("alice"),
				Labels: labelSet(),
			},
			want: true,
		},
		{
			name: "labels added",
			plan: certificateResourceModel{
				Labels: labelSet([2]string{"env", "prod"}, [2]string{"tier", "1"}),
			},
			prior: certificateResourceModel{
				Labels: labelSet([2]string{"env", "prod"}),
			},
			want: true,
		},
		{
			name: "label value flipped",
			plan: certificateResourceModel{
				Labels: labelSet([2]string{"env", "staging"}),
			},
			prior: certificateResourceModel{
				Labels: labelSet([2]string{"env", "prod"}),
			},
			want: true,
		},
		{
			name: "label set order does not matter (Set semantics)",
			plan: certificateResourceModel{
				Labels: labelSet([2]string{"tier", "1"}, [2]string{"env", "prod"}),
			},
			prior: certificateResourceModel{
				Labels: labelSet([2]string{"env", "prod"}, [2]string{"tier", "1"}),
			},
			want: false,
		},
		{
			name: "all metadata null on both sides → unchanged",
			plan: certificateResourceModel{
				Labels: labelSet(),
			},
			prior: certificateResourceModel{
				Labels: labelSet(),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := metadataChanged(tt.plan, tt.prior); got != tt.want {
				t.Fatalf("metadataChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
