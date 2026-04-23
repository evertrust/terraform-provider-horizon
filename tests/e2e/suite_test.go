//go:build e2e

// nolint
package tests

import (
	"bytes"
	"context"
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
			if tc.Failure != nil {
				t.Fatalf("%s: %s\n%s", tc.Failure.Type, tc.Failure.Message, tc.Failure.Body)
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
