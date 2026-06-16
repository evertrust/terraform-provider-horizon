package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/evertrust/horizon-go/v2/models"
)

// --- test doubles ---------------------------------------------------------

type fakeRequestClient struct {
	holderID  string
	holderErr error

	searchResults map[string]*models.RequestSearchResultsResponse
	searchErr     map[string]error
	getResponses  map[string]*models.RequestGet200Response
	getErr        map[string]error
	submitResp    *models.RequestSubmit201Response
	submitErr     error

	// recorded inputs
	searchedHolder    map[string]string // workflow -> holder id passed to search
	submittedCertID   string
	submittedPassword string
}

func (f *fakeRequestClient) certificateHolderID(_ context.Context, _ string) (string, error) {
	return f.holderID, f.holderErr
}

func (f *fakeRequestClient) search(_ context.Context, holderID, workflow string) (*models.RequestSearchResultsResponse, error) {
	if f.searchedHolder == nil {
		f.searchedHolder = map[string]string{}
	}
	f.searchedHolder[workflow] = holderID
	return f.searchResults[workflow], f.searchErr[workflow]
}

func (f *fakeRequestClient) get(_ context.Context, id string) (*models.RequestGet200Response, error) {
	return f.getResponses[id], f.getErr[id]
}

func (f *fakeRequestClient) submitRecover(_ context.Context, certID, password string) (*models.RequestSubmit201Response, error) {
	f.submittedCertID = certID
	f.submittedPassword = password
	return f.submitResp, f.submitErr
}

// --- fixture builders -----------------------------------------------------

func secret(v string) models.SecretString {
	s := models.NewSecretString()
	s.SetValue(v)
	return *s
}

func certificateWithID(id string) models.Certificate {
	c := models.NewCertificateWithDefaults()
	c.SetId(id)
	return *c
}

func emptySearchResults() *models.RequestSearchResultsResponse {
	return models.NewRequestSearchResultsResponse(false, 1, 0, []models.RequestSearchResult{})
}

func searchResultsWith(id, certID, workflow string) *models.RequestSearchResultsResponse {
	r := models.NewRequestSearchResultWithDefaults()
	r.SetId(id)
	r.SetCertificateId(certID)
	r.SetWorkflow(models.Workflow(workflow))
	r.SetStatus(models.REQUESTSTATUS_COMPLETED)
	return models.NewRequestSearchResultsResponse(false, 1, 1, []models.RequestSearchResult{*r})
}

func enrollGet(id, certID, pkcs12, password string, status models.RequestStatus) *models.RequestGet200Response {
	r := models.NewWebRAEnrollRequestOnGetResponseWithDefaults()
	r.Id = id
	r.Workflow = workflowEnroll
	r.Status = status
	r.HolderId = testHolderID
	if certID != "" {
		r.SetCertificate(certificateWithID(certID))
	}
	if pkcs12 != "" {
		r.SetPkcs12(secret(pkcs12))
	}
	if password != "" {
		r.SetPassword(secret(password))
	}
	resp := models.WebRAEnrollRequestOnGetResponseAsRequestGet200Response(r)
	return &resp
}

func recoverGet(id, certID, pkcs12, password string, status models.RequestStatus) *models.RequestGet200Response {
	r := models.NewWebRARecoverRequestOnApproveResponseWithDefaults()
	r.Id = id
	r.Workflow = workflowRecover
	r.Status = status
	r.HolderId = testHolderID
	if certID != "" {
		r.SetCertificate(certificateWithID(certID))
	}
	if pkcs12 != "" {
		r.SetPkcs12(secret(pkcs12))
	}
	if password != "" {
		r.SetPassword(secret(password))
	}
	resp := models.WebRARecoverRequestOnApproveResponseAsRequestGet200Response(r)
	return &resp
}

func recoverSubmit(id, certID, pkcs12, password string, status models.RequestStatus) *models.RequestSubmit201Response {
	r := models.NewWebRARecoverRequestOnSubmitResponseWithDefaults()
	r.Id = id
	r.Workflow = workflowRecover
	r.Status = status
	r.HolderId = testHolderID
	if certID != "" {
		r.SetCertificate(certificateWithID(certID))
	}
	if pkcs12 != "" {
		r.SetPkcs12(secret(pkcs12))
	}
	if password != "" {
		r.SetPassword(secret(password))
	}
	resp := models.WebRARecoverRequestOnSubmitResponseAsRequestSubmit201Response(r)
	return &resp
}

// search builds a fake search-result map for the two workflows.
func searchMap(enroll, recover *models.RequestSearchResultsResponse) map[string]*models.RequestSearchResultsResponse {
	return map[string]*models.RequestSearchResultsResponse{
		workflowEnroll:  enroll,
		workflowRecover: recover,
	}
}

// --- table-driven coverage of resolvePkcs12 -------------------------------

// Distinctive secret values so the no-leak assertion is meaningful.
const (
	testCertID   = "cert-1"
	testHolderID = "holder-1"
	testPkcs12   = "SECRET-P12-DATA"
	testPassword = "SECRET-PW-VALUE"
)

func TestResolvePkcs12(t *testing.T) {
	tests := []struct {
		name string
		rc   *fakeRequestClient

		// expectations
		wantErr               string // substring expected in a diagnostic summary (empty = success)
		wantSource            string
		wantWorkflow          string
		wantStatus            string
		wantRequestID         string
		wantCertID            string
		wantHolderID          string
		wantPkcs12            string
		wantPassword          string // exact expected password
		wantPasswordGenerated bool   // material.Password must equal the generated/submitted password
	}{
		// --- certificate lookup -----------------------------------------
		{
			name:    "certificate lookup failure is fatal",
			rc:      &fakeRequestClient{holderErr: fmt.Errorf("404 Not Found")},
			wantErr: "Failed to retrieve certificate",
		},

		// --- existing enroll request ------------------------------------
		{
			name: "enroll: completed request returns full contract",
			rc: &fakeRequestClient{
				searchResults: searchMap(searchResultsWith("e1", testCertID, workflowEnroll), emptySearchResults()),
				getResponses: map[string]*models.RequestGet200Response{
					"e1": enrollGet("e1", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
				},
			},
			wantSource:    sourceEnrollRequest,
			wantWorkflow:  workflowEnroll,
			wantStatus:    string(models.REQUESTSTATUS_COMPLETED),
			wantRequestID: "e1",
			wantCertID:    testCertID,
			wantPkcs12:    testPkcs12,
			wantPassword:  testPassword,
		},
		{
			name: "enroll: missing pkcs12 falls through to create",
			rc: &fakeRequestClient{
				searchResults: searchMap(searchResultsWith("e1", testCertID, workflowEnroll), emptySearchResults()),
				getResponses: map[string]*models.RequestGet200Response{
					"e1": enrollGet("e1", testCertID, "", testPassword, models.REQUESTSTATUS_COMPLETED),
				},
				submitResp: recoverSubmit("rn", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
			},
			wantSource:   sourceCreatedRecoveryRequest,
			wantPkcs12:   testPkcs12,
			wantPassword: testPassword,
		},
		{
			name: "enroll: missing password falls through to create",
			rc: &fakeRequestClient{
				searchResults: searchMap(searchResultsWith("e1", testCertID, workflowEnroll), emptySearchResults()),
				getResponses: map[string]*models.RequestGet200Response{
					"e1": enrollGet("e1", testCertID, testPkcs12, "", models.REQUESTSTATUS_COMPLETED),
				},
				submitResp: recoverSubmit("rn", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
			},
			wantSource: sourceCreatedRecoveryRequest,
		},
		{
			name: "enroll: result for a different certificate is ignored",
			rc: &fakeRequestClient{
				searchResults: searchMap(searchResultsWith("e1", "other-cert", workflowEnroll), emptySearchResults()),
				submitResp:    recoverSubmit("rn", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
			},
			wantSource: sourceCreatedRecoveryRequest,
		},
		{
			name: "enroll: get error falls through to create",
			rc: &fakeRequestClient{
				searchResults: searchMap(searchResultsWith("e1", testCertID, workflowEnroll), emptySearchResults()),
				getErr:        map[string]error{"e1": fmt.Errorf("403 Forbidden")},
				submitResp:    recoverSubmit("rn", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
			},
			wantSource: sourceCreatedRecoveryRequest,
		},
		{
			name: "enroll: newest lacks material, older completed request is reused",
			rc: &fakeRequestClient{
				searchResults: searchMap(
					multiSearchResults(
						searchResult("e-new", testCertID, workflowEnroll),
						searchResult("e-old", testCertID, workflowEnroll),
					),
					emptySearchResults(),
				),
				getResponses: map[string]*models.RequestGet200Response{
					"e-new": enrollGet("e-new", testCertID, "", "", models.REQUESTSTATUS_PENDING),
					"e-old": enrollGet("e-old", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
				},
			},
			wantSource:    sourceEnrollRequest,
			wantRequestID: "e-old",
			wantPkcs12:    testPkcs12,
			wantPassword:  testPassword,
		},
		{
			name: "recover: newest lacks material, older completed request is reused",
			rc: &fakeRequestClient{
				searchResults: searchMap(
					emptySearchResults(),
					multiSearchResults(
						searchResult("r-new", testCertID, workflowRecover),
						searchResult("r-old", testCertID, workflowRecover),
					),
				),
				getResponses: map[string]*models.RequestGet200Response{
					"r-new": recoverGet("r-new", testCertID, "", "", models.REQUESTSTATUS_PENDING),
					"r-old": recoverGet("r-old", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
				},
			},
			wantSource:    sourceRecoverRequest,
			wantRequestID: "r-old",
			wantPkcs12:    testPkcs12,
			wantPassword:  testPassword,
		},

		// --- existing recover request -----------------------------------
		{
			name: "recover: completed request returns full contract",
			rc: &fakeRequestClient{
				searchResults: searchMap(emptySearchResults(), searchResultsWith("r1", testCertID, workflowRecover)),
				getResponses: map[string]*models.RequestGet200Response{
					"r1": recoverGet("r1", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
				},
			},
			wantSource:    sourceRecoverRequest,
			wantWorkflow:  workflowRecover,
			wantStatus:    string(models.REQUESTSTATUS_COMPLETED),
			wantRequestID: "r1",
			wantCertID:    testCertID,
			wantPkcs12:    testPkcs12,
			wantPassword:  testPassword,
		},
		{
			name: "recover: missing material falls through to create",
			rc: &fakeRequestClient{
				searchResults: searchMap(emptySearchResults(), searchResultsWith("r1", testCertID, workflowRecover)),
				getResponses: map[string]*models.RequestGet200Response{
					"r1": recoverGet("r1", testCertID, "", "", models.REQUESTSTATUS_COMPLETED),
				},
				submitResp: recoverSubmit("rn", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
			},
			wantSource: sourceCreatedRecoveryRequest,
		},

		// --- created recover request ------------------------------------
		{
			name: "create: submit returns material directly",
			rc: &fakeRequestClient{
				searchResults: searchMap(emptySearchResults(), emptySearchResults()),
				submitResp:    recoverSubmit("rn", "cert-from-recover", testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
			},
			wantSource:    sourceCreatedRecoveryRequest,
			wantWorkflow:  workflowRecover,
			wantStatus:    string(models.REQUESTSTATUS_COMPLETED),
			wantRequestID: "rn",
			wantCertID:    "cert-from-recover",
			wantPkcs12:    testPkcs12,
			wantPassword:  testPassword,
		},
		{
			name: "create: submit without password uses the generated password",
			rc: &fakeRequestClient{
				searchResults: searchMap(emptySearchResults(), emptySearchResults()),
				submitResp:    recoverSubmit("rn", "cert-from-recover", testPkcs12, "", models.REQUESTSTATUS_COMPLETED),
			},
			wantSource:            sourceCreatedRecoveryRequest,
			wantCertID:            "cert-from-recover",
			wantPkcs12:            testPkcs12,
			wantPasswordGenerated: true,
		},
		{
			name: "create: submit returns only an id then GET fetches material",
			rc: &fakeRequestClient{
				searchResults: searchMap(emptySearchResults(), emptySearchResults()),
				submitResp:    recoverSubmit("rn", "", "", "", models.REQUESTSTATUS_COMPLETED),
				getResponses: map[string]*models.RequestGet200Response{
					"rn": recoverGet("rn", "cert-from-recover", testPkcs12, "", models.REQUESTSTATUS_COMPLETED),
				},
			},
			wantSource:            sourceCreatedRecoveryRequest,
			wantCertID:            "cert-from-recover",
			wantPkcs12:            testPkcs12,
			wantPasswordGenerated: true,
		},
		{
			name: "create: search errors fall through to create",
			rc: &fakeRequestClient{
				searchErr: map[string]error{
					workflowEnroll:  fmt.Errorf("400 Bad Request Invalid HQL Query"),
					workflowRecover: fmt.Errorf("400 Bad Request Invalid HQL Query"),
				},
				submitResp: recoverSubmit("rn", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
			},
			wantSource:   sourceCreatedRecoveryRequest,
			wantPkcs12:   testPkcs12,
			wantPassword: testPassword,
		},

		// --- error paths ------------------------------------------------
		{
			name: "create: pending request returns a diagnostic",
			rc: &fakeRequestClient{
				searchResults: searchMap(emptySearchResults(), emptySearchResults()),
				submitResp:    recoverSubmit("rn", testCertID, "", "", models.REQUESTSTATUS_PENDING),
			},
			wantErr: "did not complete",
		},
		{
			name: "create: denied request returns a diagnostic",
			rc: &fakeRequestClient{
				searchResults: searchMap(emptySearchResults(), emptySearchResults()),
				submitResp:    recoverSubmit("rn", testCertID, "", "", models.REQUESTSTATUS_DENIED),
			},
			wantErr: "did not complete",
		},
		{
			name: "create: non-escrowed certificate returns an escrow diagnostic",
			rc: &fakeRequestClient{
				searchResults: searchMap(emptySearchResults(), emptySearchResults()),
				submitErr:     fmt.Errorf("400 Bad Request Invalid Request: Recover request can only be made on escrowed certificate"),
			},
			wantErr: "escrow",
		},
		{
			name: "create: generic submit error returns a diagnostic",
			rc: &fakeRequestClient{
				searchResults: searchMap(emptySearchResults(), emptySearchResults()),
				submitErr:     fmt.Errorf("500 Internal Server Error"),
			},
			wantErr: "Failed to submit recovery request",
		},
		{
			name: "create: completed without material or id returns a diagnostic",
			rc: &fakeRequestClient{
				searchResults: searchMap(emptySearchResults(), emptySearchResults()),
				submitResp:    recoverSubmit("", "", "", "", models.REQUESTSTATUS_COMPLETED),
			},
			wantErr: "did not return material",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Every case has a resolvable holder unless it explicitly tests the
			// certificate-lookup failure.
			if tt.rc.holderID == "" && tt.rc.holderErr == nil {
				tt.rc.holderID = testHolderID
			}

			material, diags := resolvePkcs12(context.Background(), tt.rc, testCertID)

			// No diagnostic may ever leak secret material, in any case.
			for _, d := range diags {
				blob := d.Summary() + " " + d.Detail()
				if strings.Contains(blob, testPkcs12) || strings.Contains(blob, testPassword) {
					t.Errorf("diagnostic leaked a secret value: %q / %q", d.Summary(), d.Detail())
				}
				if tt.rc.submittedPassword != "" && strings.Contains(blob, tt.rc.submittedPassword) {
					t.Errorf("diagnostic leaked the generated password")
				}
			}

			if tt.wantErr != "" {
				if !diags.HasError() {
					t.Fatalf("expected an error diagnostic containing %q, got none", tt.wantErr)
				}
				if material != nil {
					t.Errorf("expected nil material on error, got %+v", material)
				}
				found := false
				for _, d := range diags {
					if strings.Contains(strings.ToLower(d.Summary()), strings.ToLower(tt.wantErr)) {
						found = true
					}
				}
				if !found {
					t.Errorf("expected a diagnostic summary containing %q, got %v", tt.wantErr, diags)
				}
				return
			}

			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if material == nil {
				t.Fatal("expected material, got nil")
			}
			if tt.wantSource != "" && material.Source != tt.wantSource {
				t.Errorf("source = %q, want %q", material.Source, tt.wantSource)
			}
			if tt.wantWorkflow != "" && material.RequestWorkflow != tt.wantWorkflow {
				t.Errorf("request_workflow = %q, want %q", material.RequestWorkflow, tt.wantWorkflow)
			}
			if tt.wantStatus != "" && material.RequestStatus != tt.wantStatus {
				t.Errorf("request_status = %q, want %q", material.RequestStatus, tt.wantStatus)
			}
			if tt.wantRequestID != "" && material.RequestID != tt.wantRequestID {
				t.Errorf("request_id = %q, want %q", material.RequestID, tt.wantRequestID)
			}
			if tt.wantCertID != "" && material.CertificateID != tt.wantCertID {
				t.Errorf("certificate_id = %q, want %q", material.CertificateID, tt.wantCertID)
			}
			// holder_id must always be populated on success.
			if material.HolderID != testHolderID {
				t.Errorf("holder_id = %q, want %q", material.HolderID, testHolderID)
			}
			if tt.wantHolderID != "" && material.HolderID != tt.wantHolderID {
				t.Errorf("holder_id = %q, want %q", material.HolderID, tt.wantHolderID)
			}
			if tt.wantPkcs12 != "" && material.Pkcs12 != tt.wantPkcs12 {
				t.Errorf("pkcs12 = %q, want %q", material.Pkcs12, tt.wantPkcs12)
			}
			if tt.wantPasswordGenerated {
				if tt.rc.submittedPassword == "" {
					t.Fatal("expected a generated password to have been submitted")
				}
				if material.Password != tt.rc.submittedPassword {
					t.Errorf("password = %q, want the generated password %q", material.Password, tt.rc.submittedPassword)
				}
			} else if tt.wantPassword != "" && material.Password != tt.wantPassword {
				t.Errorf("password = %q, want %q", material.Password, tt.wantPassword)
			}
			// A successful result must always carry usable material.
			if !material.hasMaterial() {
				t.Errorf("expected non-empty pkcs12 and password, got %+v", material)
			}
		})
	}
}

func TestResolvePkcs12_SearchesByHolderID(t *testing.T) {
	// When no enroll request is usable, the provider must search recovery
	// requests scoped by the resolved holder id (HRQL cannot filter by
	// certificate id), then narrow to the certificate.
	rc := &fakeRequestClient{
		holderID:      testHolderID,
		searchResults: searchMap(emptySearchResults(), searchResultsWith("r1", testCertID, workflowRecover)),
		getResponses: map[string]*models.RequestGet200Response{
			"r1": recoverGet("r1", testCertID, testPkcs12, testPassword, models.REQUESTSTATUS_COMPLETED),
		},
	}

	material, diags := resolvePkcs12(context.Background(), rc, testCertID)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if material.Source != sourceRecoverRequest {
		t.Errorf("source = %q, want %q", material.Source, sourceRecoverRequest)
	}
	if got := rc.searchedHolder[workflowEnroll]; got != testHolderID {
		t.Errorf("enroll search holder = %q, want %q", got, testHolderID)
	}
	if got := rc.searchedHolder[workflowRecover]; got != testHolderID {
		t.Errorf("recover search holder = %q, want %q", got, testHolderID)
	}
}

func TestResolvePkcs12_SubmittedCertIDAndPassword(t *testing.T) {
	// The provider must submit the recovery request with the requested
	// certificate id and a generated (non-empty) password.
	rc := &fakeRequestClient{
		searchResults: searchMap(emptySearchResults(), emptySearchResults()),
		submitResp:    recoverSubmit("rn", testCertID, testPkcs12, "", models.REQUESTSTATUS_COMPLETED),
	}

	_, diags := resolvePkcs12(context.Background(), rc, testCertID)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if rc.submittedCertID != testCertID {
		t.Errorf("submitted cert id = %q, want %q", rc.submittedCertID, testCertID)
	}
	if rc.submittedPassword == "" {
		t.Error("expected a generated password to be submitted")
	}
}

func searchResult(id, certID, workflow string) models.RequestSearchResult {
	r := models.NewRequestSearchResultWithDefaults()
	r.SetId(id)
	r.SetCertificateId(certID)
	r.SetWorkflow(models.Workflow(workflow))
	return *r
}

// multiSearchResults builds a holder-scoped search page from the given results.
func multiSearchResults(results ...models.RequestSearchResult) *models.RequestSearchResultsResponse {
	return models.NewRequestSearchResultsResponse(false, 1, int64(len(results)), results)
}

func TestUsableRequestIDs(t *testing.T) {
	// A holder-scoped page mixing certificates and workflows: only exact
	// certificate-id + workflow matches are kept, in newest-first order.
	multi := multiSearchResults(
		searchResult("other-1", "other-cert", workflowEnroll),
		searchResult("mine-newest", testCertID, workflowEnroll),
		searchResult("mine-recover", testCertID, workflowRecover),
		searchResult("mine-older", testCertID, workflowEnroll),
	)

	tests := []struct {
		name     string
		resp     *models.RequestSearchResultsResponse
		certID   string
		workflow string
		want     []string
	}{
		{name: "nil response", resp: nil, certID: testCertID, workflow: workflowEnroll, want: nil},
		{name: "empty results", resp: emptySearchResults(), certID: testCertID, workflow: workflowEnroll, want: nil},
		{name: "single match", resp: searchResultsWith("e1", testCertID, workflowEnroll), certID: testCertID, workflow: workflowEnroll, want: []string{"e1"}},
		{name: "wrong certificate", resp: searchResultsWith("e1", "other-cert", workflowEnroll), certID: testCertID, workflow: workflowEnroll, want: nil},
		{name: "wrong workflow", resp: searchResultsWith("e1", testCertID, workflowRecover), certID: testCertID, workflow: workflowEnroll, want: nil},
		{name: "keeps all matches newest-first", resp: multi, certID: testCertID, workflow: workflowEnroll, want: []string{"mine-newest", "mine-older"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := usableRequestIDs(tt.resp, tt.certID, tt.workflow)
			if len(got) != len(tt.want) {
				t.Fatalf("usableRequestIDs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("usableRequestIDs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGeneratePkcs12Password(t *testing.T) {
	pw, err := generatePkcs12Password()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pw) != passwordLength {
		t.Errorf("password length = %d, want %d", len(pw), passwordLength)
	}
	if !strings.ContainsAny(pw, passwordLowerset) {
		t.Error("password missing a lowercase character")
	}
	if !strings.ContainsAny(pw, passwordUpperset) {
		t.Error("password missing an uppercase character")
	}
	if !strings.ContainsAny(pw, passwordDigitset) {
		t.Error("password missing a digit")
	}
	if !strings.ContainsAny(pw, passwordSpecialset) {
		t.Error("password missing a special character")
	}

	pw2, err := generatePkcs12Password()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pw == pw2 {
		t.Error("expected two generated passwords to differ")
	}
}
