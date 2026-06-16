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
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

const stagedProviderVersion = "0.0.1"

// TestE2E is the Go entry point for the e2e suite. It delegates to E2ESuite,
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
	s.reportsDir = filepath.Join(s.repoRoot, "tests", "reports")
	s.Require().NoError(os.MkdirAll(s.reportsDir, 0o755))

	// Copy tftests/ into a per-suite temp dir so parallel `mise run e2e`
	// invocations don't race on the same .terraform/ and lock file.
	srcTftests := filepath.Join(cwd, "tftests")
	s.tftestsDir = filepath.Join(t.TempDir(), "tftests")
	s.Require().NoError(os.CopyFS(s.tftestsDir, os.DirFS(srcTftests)))

	s.binDir = t.TempDir()
	binPath := filepath.Join(s.binDir, "terraform-provider-horizon")

	t.Log("building provider binary")
	build := exec.CommandContext(s.ctx, "go", "build", "-buildvcs=false", "-o", binPath, ".")
	build.Dir = s.repoRoot
	out, err := build.CombinedOutput()
	s.Require().NoErrorf(err, "go build: %s", out)

	localMirrorDir := filepath.Join(s.binDir, "tf-mirror")
	s.stageProviderInDir(binPath, localMirrorDir)
	s.stageProviderInMirror(binPath)

	cliConfigPath := filepath.Join(s.binDir, ".terraformrc")
	cliConfig := `provider_installation {
  dev_overrides {
    "registry.terraform.io/evertrust/horizon" = "` + s.binDir + `"
  }
  filesystem_mirror {
    path    = "` + localMirrorDir + `"
    include = ["registry.terraform.io/evertrust/horizon"]
  }
`
	excluded := []string{`"registry.terraform.io/evertrust/horizon"`}
	if _, err := os.Stat("/opt/tf-mirror"); err == nil {
		cliConfig += `  filesystem_mirror {
    path    = "/opt/tf-mirror"
    include = ["registry.terraform.io/hashicorp/*"]
  }
`
		excluded = append(excluded, `"registry.terraform.io/hashicorp/*"`)
	}
	cliConfig += `  direct {
    exclude = [` + strings.Join(excluded, ", ") + `]
  }
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

func (s *E2ESuite) stageProviderInMirror(binPath string) {
	const mirrorRoot = "/opt/tf-mirror"
	if _, err := os.Stat(mirrorRoot); err != nil {
		return
	}
	s.stageProviderInDir(binPath, mirrorRoot)
}

func (s *E2ESuite) stageProviderInDir(binPath, mirrorRoot string) {
	target := runtime.GOOS + "_" + runtime.GOARCH
	stagedDir := filepath.Join(
		mirrorRoot, "registry.terraform.io", "evertrust", "horizon",
		stagedProviderVersion, target,
	)
	s.Require().NoError(os.MkdirAll(stagedDir, 0o755), "create mirror staging dir")

	dst := filepath.Join(stagedDir, "terraform-provider-horizon_v"+stagedProviderVersion)
	src, err := os.Open(binPath)
	s.Require().NoError(err, "open built provider binary")
	defer src.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	s.Require().NoError(err, "create staged provider binary")
	defer out.Close()

	_, err = io.Copy(out, src)
	s.Require().NoError(err, "copy provider binary into mirror")
}

func (s *E2ESuite) TestCentralized() {
	s.runTftestFile("certificate_centralized.tftest.hcl")
}

func (s *E2ESuite) TestDecentralized() {
	s.runTftestFile("certificate_decentralized.tftest.hcl")
}

func (s *E2ESuite) TestRenew() {
	s.runTftestFile("certificate_renew.tftest.hcl")
}

func (s *E2ESuite) TestNoDrift() {
	s.runTftestFile("certificate_no_drift.tftest.hcl")
}

func (s *E2ESuite) TestMetadata() {
	s.runTftestFile("certificate_metadata.tftest.hcl")
}

func (s *E2ESuite) TestTrustChain() {
	s.runTftestFile("certificate_trust_chain.tftest.hcl")
}

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
		"HORIZON_ESCROW_PROFILE="+CentralizedProfile,
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

func (s *E2ESuite) tfCLIConfigPath() string {
	return filepath.Join(s.binDir, ".terraformrc")
}

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

var runMarkerRE = regexp.MustCompile(`^\s*run\s+"([^"]+)"\.\.\.`)

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

type goTestEvent struct {
	Action  string  `json:"Action"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

type goTestResult struct {
	Name    string
	Outcome string // "pass", "fail", or "skip"
	Elapsed float64
	Output  string
}

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
