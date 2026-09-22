package provider

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"net"
	"net/url"
	"testing"

	"github.com/evertrust/horizon-go/v2/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestUsesChallenge(t *testing.T) {
	tests := []struct {
		name string
		data certificateResourceModel
		want bool
	}{
		{name: "neither", data: certificateResourceModel{}, want: false},
		{name: "request_challenge false", data: certificateResourceModel{RequestChallenge: types.BoolValue(false)}, want: false},
		{name: "request_challenge true", data: certificateResourceModel{RequestChallenge: types.BoolValue(true)}, want: true},
		{name: "challenge set", data: certificateResourceModel{Challenge: types.StringValue("otp")}, want: true},
		// A challenge coming from another resource is unknown at plan time but
		// will hold a value at apply time.
		{name: "challenge unknown", data: certificateResourceModel{Challenge: types.StringUnknown()}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usesChallenge(tt.data); got != tt.want {
				t.Fatalf("usesChallenge = %v, want %v", got, tt.want)
			}
		})
	}
}

func labelsSet(t *testing.T, label, value string) types.Set {
	t.Helper()
	elemType := types.ObjectType{AttrTypes: map[string]attr.Type{"label": types.StringType, "value": types.StringType}}
	obj, diags := types.ObjectValue(elemType.AttrTypes, map[string]attr.Value{
		"label": types.StringValue(label),
		"value": types.StringValue(value),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	set, diags := types.SetValue(elemType, []attr.Value{obj})
	if diags.HasError() {
		t.Fatal(diags)
	}
	return set
}

func summaries(diags []diag.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Summary())
	}
	return out
}

func TestValidateChallengeConfig(t *testing.T) {
	tests := []struct {
		name         string
		data         certificateResourceModel
		wantErrors   []string
		wantWarnings []string
	}{
		{
			// Existing flows must be unaffected. Without a challenge, no
			// diagnostic is reported for any combination of the other attributes.
			name: "no challenge: nothing reported",
			data: certificateResourceModel{
				Password:        types.StringValue("secret"),
				Pkcs12WriteOnly: types.BoolValue(true),
				Owner:           types.StringValue("alice"),
			},
		},
		{
			name: "centralized with a provided challenge",
			data: certificateResourceModel{Challenge: types.StringValue("otp")},
		},
		{
			name: "centralized with a requested challenge and ownership",
			data: certificateResourceModel{RequestChallenge: types.BoolValue(true), Owner: types.StringValue("alice")},
		},
		{
			name:       "challenge and request_challenge",
			data:       certificateResourceModel{Challenge: types.StringValue("otp"), RequestChallenge: types.BoolValue(true)},
			wantErrors: []string{"request_challenge conflicts with challenge"},
		},
		{
			name:       "password with a challenge",
			data:       certificateResourceModel{Challenge: types.StringValue("otp"), Password: types.StringValue("secret")},
			wantErrors: []string{"password cannot be set when enrolling with a challenge"},
		},
		{
			name:       "centralized pkcs12_write_only would lose the key",
			data:       certificateResourceModel{RequestChallenge: types.BoolValue(true), Pkcs12WriteOnly: types.BoolValue(true)},
			wantErrors: []string{"pkcs12_write_only would lose the private key of a challenge enrollment"},
		},
		{
			name: "decentralized pkcs12_write_only is harmless",
			data: certificateResourceModel{
				RequestChallenge: types.BoolValue(true),
				Csr:              types.StringValue("csr"),
				Pkcs12WriteOnly:  types.BoolValue(true),
			},
		},
		{
			name: "ownership is not applied with a provided challenge",
			data: certificateResourceModel{
				Challenge:    types.StringValue("otp"),
				Owner:        types.StringValue("alice"),
				Team:         types.StringValue("backend"),
				ContactEmail: types.StringValue("alice@example.org"),
				Labels:       labelsSet(t, "env", "prod"),
			},
			wantWarnings: []string{
				"owner is not applied at enrollment when challenge is provided.",
				"team is not applied at enrollment when challenge is provided.",
				"contact_email is not applied at enrollment when challenge is provided.",
				"labels are not applied at enrollment when challenge is provided.",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateChallengeConfig(tt.data)
			if got := summaries(diags.Errors()); !equalStrings(got, tt.wantErrors) {
				t.Errorf("errors = %v, want %v", got, tt.wantErrors)
			}
			if got := summaries(diags.Warnings()); !equalStrings(got, tt.wantWarnings) {
				t.Errorf("warnings = %v, want %v", got, tt.wantWarnings)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func testCSR(t *testing.T, subject pkix.Name, template x509.CertificateRequest) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template.Subject = subject
	der, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
}

func TestIdentityFromCSR(t *testing.T) {
	t.Run("subject and SANs", func(t *testing.T) {
		uri, _ := url.Parse("spiffe://example.org/web")
		csr := testCSR(t,
			pkix.Name{
				CommonName:         "web.example.org",
				Organization:       []string{"Example"},
				OrganizationalUnit: []string{"Ops", "Web"},
				Country:            []string{"FR"},
			},
			x509.CertificateRequest{
				DNSNames:       []string{"web.example.org", "www.example.org"},
				IPAddresses:    []net.IP{net.ParseIP("10.0.0.1")},
				EmailAddresses: []string{"ops@example.org"},
				URIs:           []*url.URL{uri},
			},
		)

		subject, sans, diags := identityFromCSR(csr)
		if len(diags) > 0 {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		gotSubject := map[string]string{}
		for _, el := range subject {
			gotSubject[el.Element] = *el.Value.Get()
		}
		wantSubject := map[string]string{
			"cn.1": "web.example.org",
			"o.1":  "Example",
			// Same-type attributes are indexed in CSR order.
			"ou.1": "Ops",
			"ou.2": "Web",
			"c.1":  "FR",
		}
		if len(gotSubject) != len(wantSubject) {
			t.Fatalf("subject = %v, want %v", gotSubject, wantSubject)
		}
		for element, want := range wantSubject {
			if gotSubject[element] != want {
				t.Errorf("subject[%s] = %q, want %q", element, gotSubject[element], want)
			}
		}

		gotSans := map[string][]string{}
		for _, el := range sans {
			gotSans[*el.Type.Get()] = el.Value
		}
		wantSans := map[string][]string{
			"DNSNAME":    {"web.example.org", "www.example.org"},
			"IPADDRESS":  {"10.0.0.1"},
			"RFC822NAME": {"ops@example.org"},
			"URI":        {"spiffe://example.org/web"},
		}
		if len(gotSans) != len(wantSans) {
			t.Fatalf("sans = %v, want %v", gotSans, wantSans)
		}
		for sanType, want := range wantSans {
			if !equalStrings(gotSans[sanType], want) {
				t.Errorf("sans[%s] = %v, want %v", sanType, gotSans[sanType], want)
			}
		}
	})

	t.Run("no SAN yields no SAN element", func(t *testing.T) {
		_, sans, diags := identityFromCSR(testCSR(t, pkix.Name{CommonName: "web.example.org"}, x509.CertificateRequest{}))
		if len(diags) > 0 || len(sans) != 0 {
			t.Fatalf("sans = %v, diags = %v; want none", sans, diags)
		}
	})

	t.Run("attribute Horizon has no element for is skipped with a warning", func(t *testing.T) {
		csr := testCSR(t, pkix.Name{
			CommonName: "web.example.org",
			// businessCategory
			ExtraNames: []pkix.AttributeTypeAndValue{{Type: asn1.ObjectIdentifier{2, 5, 4, 15}, Value: "Private"}},
		}, x509.CertificateRequest{})

		subject, _, diags := identityFromCSR(csr)
		if diags.HasError() || diags.WarningsCount() != 1 {
			t.Fatalf("diags = %v, want exactly one warning", diags)
		}
		for _, el := range subject {
			if *el.Value.Get() == "Private" {
				t.Fatalf("unknown attribute must not be sent: %v", subject)
			}
		}
	})

	t.Run("not PEM", func(t *testing.T) {
		if _, _, diags := identityFromCSR("not a csr"); !diags.HasError() {
			t.Fatal("expected an error")
		}
	})

	t.Run("PEM but not a CSR", func(t *testing.T) {
		notCSR := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: []byte("garbage")}))
		if _, _, diags := identityFromCSR(notCSR); !diags.HasError() {
			t.Fatal("expected an error")
		}
	})
}

func subjectSet(t *testing.T, cn string) types.Set {
	t.Helper()
	elemType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"element": types.StringType, "type": types.StringType, "value": types.StringType,
	}}
	obj, diags := types.ObjectValue(elemType.AttrTypes, map[string]attr.Value{
		"element": types.StringValue("cn.1"),
		"type":    types.StringValue("CN"),
		"value":   types.StringValue(cn),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	set, diags := types.SetValue(elemType, []attr.Value{obj})
	if diags.HasError() {
		t.Fatal(diags)
	}
	return set
}

// identityModel returns a model whose subject and sans are typed null sets, as
// the framework hands them over when they are not configured.
func identityModel() certificateResourceModel {
	subjectType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"element": types.StringType, "type": types.StringType, "value": types.StringType,
	}}
	sanType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"type": types.StringType, "value": types.ListType{ElemType: types.StringType},
	}}
	return certificateResourceModel{Subject: types.SetNull(subjectType), Sans: types.SetNull(sanType)}
}

func TestChallengeIdentity(t *testing.T) {
	ctx := context.Background()
	csr := testCSR(t, pkix.Name{CommonName: "from-csr.example.org"}, x509.CertificateRequest{})

	t.Run("configured subject wins over the CSR", func(t *testing.T) {
		data := identityModel()
		data.Csr = types.StringValue(csr)
		data.Subject = subjectSet(t, "configured.example.org")
		subject, _, diags := challengeIdentity(ctx, data)
		if diags.HasError() || len(subject) != 1 || *subject[0].Value.Get() != "configured.example.org" {
			t.Fatalf("subject = %v, diags = %v", subject, diags)
		}
	})

	t.Run("CSR identity is the default", func(t *testing.T) {
		data := identityModel()
		data.Csr = types.StringValue(csr)
		subject, _, diags := challengeIdentity(ctx, data)
		if diags.HasError() || len(subject) != 1 || *subject[0].Value.Get() != "from-csr.example.org" {
			t.Fatalf("subject = %v, diags = %v", subject, diags)
		}
	})

	t.Run("centralized without identity sends none", func(t *testing.T) {
		subject, sans, diags := challengeIdentity(ctx, identityModel())
		if diags.HasError() || len(subject) != 0 || len(sans) != 0 {
			t.Fatalf("subject = %v, sans = %v, diags = %v", subject, sans, diags)
		}
	})
}

func TestChallengeSubmitTemplate(t *testing.T) {
	cn := models.IndexedDNElement{Element: "cn.1"}
	cn.SetValue("web.example.org")

	t.Run("centralized sends the key type and no CSR", func(t *testing.T) {
		tmpl := challengeSubmitTemplate(certificateResourceModel{KeyType: types.StringValue("rsa-2048")}, []models.IndexedDNElement{cn}, nil)
		if tmpl.Csr != nil || tmpl.KeyType == nil || *tmpl.KeyType != "rsa-2048" {
			t.Fatalf("csr = %v, keyType = %v", tmpl.Csr, tmpl.KeyType)
		}
		if len(tmpl.Subject) != 1 || tmpl.Sans != nil {
			t.Fatalf("subject = %v, sans = %v", tmpl.Subject, tmpl.Sans)
		}
	})

	t.Run("decentralized sends the CSR and no key type", func(t *testing.T) {
		// csr and keyType are mutually exclusive on the endpoint.
		tmpl := challengeSubmitTemplate(certificateResourceModel{
			Csr:     types.StringValue("csr-pem"),
			KeyType: types.StringValue("rsa-2048"),
		}, nil, nil)
		if tmpl.Csr == nil || *tmpl.Csr != "csr-pem" || tmpl.KeyType != nil {
			t.Fatalf("csr = %v, keyType = %v", tmpl.Csr, tmpl.KeyType)
		}
	})

	t.Run("unknown key type is left to the profile default", func(t *testing.T) {
		tmpl := challengeSubmitTemplate(certificateResourceModel{KeyType: types.StringUnknown()}, nil, nil)
		if tmpl.KeyType != nil {
			t.Fatalf("keyType = %v, want nil", *tmpl.KeyType)
		}
	})
}

func TestTemplateDefinesIdentity(t *testing.T) {
	if templateDefinesIdentity(nil) {
		t.Fatal("nil template must not define an identity")
	}
	empty := &models.WebRAEnrollRequestOnTemplateResponse{}
	if templateDefinesIdentity(empty) {
		t.Fatal("empty template must not define an identity")
	}
	withSubject := &models.WebRAEnrollRequestOnTemplateResponse{}
	withSubject.Template.Subject = []models.IndexedDNElementResponse{{Element: "cn.1"}}
	if !templateDefinesIdentity(withSubject) {
		t.Fatal("a template with a subject defines an identity")
	}
}

func submitResponse(status models.RequestStatus, challenge string, cert *models.Certificate) *models.RequestSubmit201Response {
	r := &models.WebRAEnrollRequestOnSubmitResponse{Id: "req-1", Status: status}
	if challenge != "" {
		r.SetPassword(secret(challenge))
	}
	if cert != nil {
		r.SetCertificate(*cert)
	}
	return &models.RequestSubmit201Response{WebRAEnrollRequestOnSubmitResponse: r}
}

func TestExtractIssuedChallenge(t *testing.T) {
	t.Run("approved request carrying a challenge", func(t *testing.T) {
		issued, diags := extractIssuedChallenge(submitResponse(models.REQUESTSTATUS_APPROVED, "otp", nil))
		if diags.HasError() || issued == nil || issued.challenge != "otp" || issued.requestID != "req-1" {
			t.Fatalf("issued = %+v, diags = %v", issued, diags)
		}
	})

	errorCases := []struct {
		name string
		resp *models.RequestSubmit201Response
		want string
	}{
		{name: "nil response", resp: nil, want: "Unexpected response type"},
		{name: "other response type", resp: &models.RequestSubmit201Response{}, want: "Unexpected response type"},
		{
			// The profile enrolled directly, so it is not in Challenge mode.
			name: "certificate instead of a challenge",
			resp: submitResponse(models.REQUESTSTATUS_COMPLETED, "p12-password", &models.Certificate{Id: "cert-1"}),
			want: "Profile is not in Challenge mode",
		},
		{
			name: "pending approval",
			resp: submitResponse(models.REQUESTSTATUS_PENDING, "", nil),
			want: "Challenge request was not approved",
		},
		{
			name: "approved without a challenge",
			resp: submitResponse(models.REQUESTSTATUS_APPROVED, "", nil),
			want: "No challenge in the approved request",
		},
	}
	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			issued, diags := extractIssuedChallenge(tt.resp)
			if issued != nil || !containsErrorSummary(diags, tt.want) {
				t.Fatalf("issued = %+v, diags = %v; want error %q", issued, diags, tt.want)
			}
		})
	}
}
