//go:build e2e

// nolint
package tests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

// TestE2E is the Go entry point for the e2e suite. It delegates to E2ESuite,
// which builds the provider, spins up Horizon once via testcontainers, then
// runs one Test method per .tftest.hcl file. Each Terraform `run` block is
// surfaced as a Go sub-test via JUnit parsing so reports show a separate line
// per scenario instead of a single big TestE2E.
func TestE2E(t *testing.T) {
	suite.Run(t, new(E2ESuite))
}

type E2ESuite struct {
	suite.Suite
	ctx        context.Context
	repoRoot   string
	tftestsDir string
	reportsDir string
	binDir     string
	instances  *HorizonTestInstances
	tfEnv      []string
}

func (s *E2ESuite) SetupSuite() {
	s.ctx = context.Background()
	t := s.T()

	cwd, err := os.Getwd()
	s.Require().NoError(err)
	s.repoRoot = filepath.Clean(filepath.Join(cwd, "..", ".."))
	s.tftestsDir = filepath.Join(cwd, "tftests")
	s.reportsDir = filepath.Join(s.repoRoot, "tests", "reports")
	s.Require().NoError(os.MkdirAll(s.reportsDir, 0o755))

	s.binDir = t.TempDir()
	binPath := filepath.Join(s.binDir, "terraform-provider-horizon")

	t.Log("building provider binary")
	build := exec.CommandContext(s.ctx, "go", "build", "-buildvcs=false", "-o", binPath, ".")
	build.Dir = s.repoRoot
	out, err := build.CombinedOutput()
	s.Require().NoErrorf(err, "go build: %s", out)

	cliConfigPath := filepath.Join(s.binDir, ".terraformrc")
	cliConfig := `provider_installation {
  dev_overrides {
    "registry.terraform.io/evertrust/horizon" = "` + s.binDir + `"
  }
  direct {}
}
`
	s.Require().NoError(os.WriteFile(cliConfigPath, []byte(cliConfig), 0o644))

	t.Log("starting Horizon instance")
	instances, err := UpHorizonInstance(s.ctx, t)
	s.Require().NoError(err)
	s.instances = instances

	s.tfEnv = append(os.Environ(),
		"TF_CLI_CONFIG_FILE="+cliConfigPath,
		"TF_VAR_endpoint="+instances.Nginx.HttpUrl,
		"TF_VAR_username="+AdminUsername,
		"TF_VAR_password="+AdminPassword,
		"TF_VAR_centralized_profile="+CentralizedProfile,
		"TF_VAR_decentralized_profile="+DecentralizedProfile,
	)

	var initOut bytes.Buffer
	initSinks := io.MultiWriter(&testWriter{t: t}, &initOut)
	t.Log("running terraform init (installs tls provider + child modules)")
	tfInit := exec.CommandContext(s.ctx, "terraform", "init", "-input=false")
	tfInit.Dir = s.tftestsDir
	tfInit.Env = s.tfEnv
	tfInit.Stdout = initSinks
	tfInit.Stderr = initSinks
	s.Require().NoError(tfInit.Run(), "terraform init")

	// Proof that the locally built provider binary is used, not the registry.
	// The warning only appears when dev_overrides is actually honored.
	s.Require().Contains(
		initOut.String(),
		"Provider development overrides are in effect",
		"dev_overrides warning missing — Terraform may have used the registry provider instead of %s",
		s.binDir,
	)
	s.Require().Contains(
		initOut.String(),
		s.binDir,
		"dev_overrides warning present but does not reference %s",
		s.binDir,
	)
}

func (s *E2ESuite) TearDownSuite() {
	if s.instances != nil {
		if err := DownHorizonInstance(s.ctx, *s.instances); err != nil {
			s.T().Logf("DownHorizonInstance: %v", err)
		}
	}
}

// TestCentralized runs every centralized-enrollment run block as a sub-test.
func (s *E2ESuite) TestCentralized() {
	s.runTftestFile("certificate_centralized.tftest.hcl")
}

// TestDecentralized runs every decentralized-enrollment run block as a sub-test.
func (s *E2ESuite) TestDecentralized() {
	s.runTftestFile("certificate_decentralized.tftest.hcl")
}

// TestRenew exercises the renewal behavior: centralized certificates are
// renewed in place via the WebRA renew endpoint when they enter their
// renew_before window; decentralized ones go through destroy/create
// (RequiresReplace) since a renew with the same CSR would reuse the key.
func (s *E2ESuite) TestRenew() {
	s.runTftestFile("certificate_renew.tftest.hcl")
}

// TestAcceptance runs the Terraform-plugin-testing acceptance suite under
// tests/ against the Horizon instance spun up in SetupSuite. Those tests are
// gated by TF_ACC and HORIZON_* env vars — they skip in any plain
// `go test ./...` invocation. Running them here is what actually exercises
// them end-to-end.
//
// Each TestAcc* function is surfaced as its own Go sub-test via t.Run so the
// final report shows one line per acceptance test, with its own
// outcome/output slice, instead of one opaque TestAcceptance entry.
func (s *E2ESuite) TestAcceptance() {
	t := s.T()
	acceptanceDir := filepath.Join(s.repoRoot, "tests")

	cliConfigPath := s.tfCLIConfigPath()

	env := append(os.Environ(),
		"TF_ACC=1",
		"TF_CLI_CONFIG_FILE="+cliConfigPath,
		"HORIZON_ENDPOINT="+s.instances.Nginx.HttpUrl,
		"HORIZON_USERNAME="+AdminUsername,
		"HORIZON_PASSWORD="+AdminPassword,
		"HORIZON_PROFILE="+CentralizedProfile,
		"HORIZON_DECENTRALIZED_PROFILE="+DecentralizedProfile,
	)

	t.Log("running acceptance tests under ./tests/ (go test -json)")
	cmd := exec.CommandContext(s.ctx,
		"go", "test",
		"-json",
		"-count=1",
		"-timeout", "15m",
		"-run", "^TestAcc",
		".",
	)
	cmd.Dir = acceptanceDir
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	s.Require().NoError(err, "stdout pipe")
	cmd.Stderr = &testWriter{t: t}

	s.Require().NoError(cmd.Start(), "go test -json: start")

	results := parseGoTestJSON(stdout)

	// Wait() must run after the stream is fully drained; parseGoTestJSON
	// returns only once stdout is closed.
	runErr := cmd.Wait()

	if len(results) == 0 {
		s.Require().NoError(runErr, "acceptance tests failed with no per-test events captured")
		t.Fatal("acceptance tests produced no test events — check the logs above")
	}

	for _, r := range results {
		r := r
		t.Run(r.Name, func(t *testing.T) {
			for _, line := range strings.Split(strings.TrimRight(r.Output, "\n"), "\n") {
				t.Log(line)
			}
			switch r.Outcome {
			case "fail":
				t.Fatalf("%s: FAIL (%.2fs)", r.Name, r.Elapsed)
			case "skip":
				t.Skipf("%s: SKIP", r.Name)
			}
		})
	}
}

// tfCLIConfigPath returns the TF_CLI_CONFIG_FILE written by SetupSuite that
// pins Terraform to the locally built provider binary via dev_overrides.
func (s *E2ESuite) tfCLIConfigPath() string {
	return filepath.Join(s.binDir, ".terraformrc")
}

// runTftestFile invokes `terraform test -filter=<file>`, then parses the JUnit
// report (for pass/fail status) and the captured stdout (for per-run output)
// so that each Terraform run block surfaces as its own Go sub-test with only
// its relevant log slice — not the whole file dumped under the parent test.
func (s *E2ESuite) runTftestFile(file string) {
	t := s.T()
	base := strings.TrimSuffix(file, ".tftest.hcl")
	junitPath := filepath.Join(s.reportsDir, "terraform-"+base+".xml")

	t.Logf("running terraform test -filter=%s", file)
	var captured bytes.Buffer
	cmd := exec.CommandContext(
		s.ctx,
		"terraform", "test",
		"-verbose",
		"-no-color",
		"-filter="+file,
		"-junit-xml="+junitPath,
	)
	cmd.Dir = s.tftestsDir
	cmd.Env = s.tfEnv
	cmd.Stdout = &captured
	cmd.Stderr = &captured
	// Ignore the exit code — per-run failures are surfaced via JUnit below.
	_ = cmd.Run()

	cases, err := parseJUnitCases(junitPath)
	if err != nil || len(cases) == 0 {
		// Surface the raw output so the test failure is diagnosable.
		t.Log(captured.String())
	}
	s.Require().NoErrorf(err, "parse junit %s", junitPath)
	s.Require().NotEmptyf(cases, "no test cases parsed from %s", junitPath)

	outputs := splitTerraformOutputByRun(captured.String())

	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if body, ok := outputs[tc.Name]; ok {
				for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
					t.Log(line)
				}
			}
			// terraform test emits <error> for runtime errors (e.g. provider
			// returned 4xx) and <failure> for assertion failures. Both must
			// fail the Go sub-test. <skipped> marks cascaded skips.
			if f := tc.Failure; f != nil {
				t.Fatalf("%s: %s\n%s", f.Type, f.Message, f.Body)
			}
			if e := tc.Error; e != nil {
				t.Fatalf("%s: %s\n%s", e.Type, e.Message, e.Body)
			}
			if s := tc.Skipped; s != nil {
				t.Skipf("%s: %s", s.Message, s.Body)
			}
		})
	}
}

// runMarkerRE matches the `  run "NAME"...` line that terraform test -verbose
// emits at the start of each run block.
var runMarkerRE = regexp.MustCompile(`^\s*run\s+"([^"]+)"\.\.\.`)

// splitTerraformOutputByRun splits the text produced by `terraform test
// -verbose` into per-run chunks keyed by run name. Lines before the first run
// marker are discarded; lines after the last run marker belong to it.
func splitTerraformOutputByRun(raw string) map[string]string {
	out := make(map[string]string)
	var currentName string
	var buf strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		if m := runMarkerRE.FindStringSubmatch(line); m != nil {
			if currentName != "" {
				out[currentName] = buf.String()
			}
			currentName = m[1]
			buf.Reset()
		}
		if currentName != "" {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	if currentName != "" {
		out[currentName] = buf.String()
	}
	return out
}

type junitReport struct {
	XMLName    xml.Name     `xml:"testsuites"`
	TestSuites []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	TestCases []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name    string        `xml:"name,attr"`
	Failure *junitFailure `xml:"failure"`
	Error   *junitFailure `xml:"error"`
	Skipped *junitFailure `xml:"skipped"`
}

type junitFailure struct {
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

// parseJUnitCases accepts both a <testsuites>-rooted document and a bare
// <testsuite> document (terraform test 1.14 writes the latter when a single
// file is filtered).
func parseJUnitCases(path string) ([]junitCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report junitReport
	if err := xml.Unmarshal(data, &report); err == nil && len(report.TestSuites) > 0 {
		var out []junitCase
		for _, ts := range report.TestSuites {
			out = append(out, ts.TestCases...)
		}
		return out, nil
	}
	var suite junitSuite
	if err := xml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("junit xml parse: %w", err)
	}
	return suite.TestCases, nil
}

// goTestEvent is the schema of a single line emitted by `go test -json`.
type goTestEvent struct {
	Action  string  `json:"Action"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

// goTestResult aggregates the per-test events of a single TestAcc function.
type goTestResult struct {
	Name    string
	Outcome string // "pass", "fail", or "skip"
	Elapsed float64
	Output  string
}

// parseGoTestJSON consumes a `go test -json` stream and returns one
// goTestResult per top-level test function. Sub-tests (TestAccX/step) have
// their output folded into the parent's Output but do not surface as their
// own entries. Output events that are not tied to a specific test (e.g. the
// final "PASS" summary) are discarded.
func parseGoTestJSON(r io.Reader) []goTestResult {
	type buf struct {
		out     strings.Builder
		outcome string
		elapsed float64
	}
	buffers := map[string]*buf{}
	order := []string{}

	getOrCreate := func(name string) *buf {
		if b, ok := buffers[name]; ok {
			return b
		}
		b := &buf{}
		buffers[name] = b
		order = append(order, name)
		return b
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		var ev goTestEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Test == "" {
			continue
		}
		topLevel := strings.SplitN(ev.Test, "/", 2)[0]
		b := getOrCreate(topLevel)
		switch ev.Action {
		case "output":
			b.out.WriteString(ev.Output)
		case "pass", "fail", "skip":
			// Only the top-level test's terminal event decides the outcome
			// we report; sub-test outcomes are already reflected in the
			// captured output.
			if ev.Test == topLevel {
				b.outcome = ev.Action
				b.elapsed = ev.Elapsed
			}
		}
	}

	results := make([]goTestResult, 0, len(order))
	for _, name := range order {
		b := buffers[name]
		if b.outcome == "" {
			// Never saw a terminal event (process killed, etc.) — treat as fail.
			b.outcome = "fail"
		}
		results = append(results, goTestResult{
			Name:    name,
			Outcome: b.outcome,
			Elapsed: b.elapsed,
			Output:  b.out.String(),
		})
	}
	return results
}

// testWriter forwards byte chunks to t.Log, one line at a time.
type testWriter struct {
	t   *testing.T
	mu  sync.Mutex
	buf strings.Builder
}

func (w *testWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	for {
		s := w.buf.String()
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			break
		}
		w.t.Log(strings.TrimRight(s[:idx], "\r"))
		w.buf.Reset()
		w.buf.WriteString(s[idx+1:])
	}
	return len(p), nil
}
