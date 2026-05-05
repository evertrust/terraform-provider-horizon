// tfreport converts the JUnit XML files written by `terraform test
// -junit-xml=...` into a JSON stream compatible with `go test -json`.
//
// Each tftest file already surfaces in go-e2e.html via the Go subtests
// re-emitted by tests/e2e/suite_test.go (TestE2E/<scenario>/<run_block>).
// This tool produces an additional "terraform/<file>" package namespace
// so go-test-report renders one collapsible section per .tftest.hcl —
// the perspective reviewers expect when they see terraform-* in
// tests/reports/.
//
// Usage:
//
//	go run ./tools/tfreport -dir tests/reports >> tests/reports/go-e2e.json
package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type junitReport struct {
	XMLName xml.Name     `xml:"testsuites"`
	Suites  []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	XMLName xml.Name    `xml:"testsuite"`
	Cases   []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name    string        `xml:"name,attr"`
	Time    string        `xml:"time,attr"`
	Failure *junitFailure `xml:"failure"`
	Error   *junitFailure `xml:"error"`
	Skipped *junitFailure `xml:"skipped"`
	Stdout  string        `xml:"system-out"`
	Stderr  string        `xml:"system-err"`
}

type junitFailure struct {
	Type    string `xml:"type,attr"`
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

// goTestEvent matches `go test -json` (also gotestsum's --jsonfile output).
type goTestEvent struct {
	Time    string  `json:"Time"`
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test,omitempty"`
	Output  string  `json:"Output,omitempty"`
	Elapsed float64 `json:"Elapsed,omitempty"`
}

func main() {
	dir := flag.String("dir", "tests/reports", "directory containing terraform-*.xml JUnit reports")
	pattern := flag.String("pattern", "terraform-*.xml", "glob (relative to -dir) for JUnit files to ingest")
	flag.Parse()

	matches, err := filepath.Glob(filepath.Join(*dir, *pattern))
	if err != nil {
		die("glob: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)

	for _, path := range matches {
		if err := emitFile(enc, path); err != nil {
			die("%s: %v", path, err)
		}
	}
}

// emitFile turns one JUnit XML into a self-contained go test event stream
// (start → run/output/{pass|fail|skip} per case → terminal pass|fail for
// the package).
func emitFile(enc *json.Encoder, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	suites, err := parseSuites(data)
	if err != nil {
		return err
	}

	// tests/reports/terraform-<name>.xml → package "terraform/<name>".
	// go-test-report groups by Package, so each tftest file becomes its
	// own section in the HTML output.
	base := strings.TrimPrefix(strings.TrimSuffix(filepath.Base(path), ".xml"), "terraform-")
	pkg := "terraform/" + base
	ts := time.Now().UTC().Format(time.RFC3339Nano)

	if err := enc.Encode(event(ts, "start", pkg, "", "", 0)); err != nil {
		return err
	}

	var elapsed float64
	failed := false
	for _, s := range suites {
		for _, tc := range s.Cases {
			r, err := emitCase(enc, ts, pkg, tc)
			if err != nil {
				return err
			}
			elapsed += r.elapsed
			failed = failed || r.failed
		}
	}

	final := "pass"
	if failed {
		final = "fail"
	}
	return enc.Encode(event(ts, final, pkg, "", "", elapsed))
}

type caseResult struct {
	elapsed float64
	failed  bool
}

func emitCase(enc *json.Encoder, ts, pkg string, tc junitCase) (caseResult, error) {
	if err := enc.Encode(event(ts, "run", pkg, tc.Name, "", 0)); err != nil {
		return caseResult{}, err
	}

	// Captured stdout/stderr carries terraform's run-block diagnostics;
	// route it through "output" events so it shows up in the rendered HTML
	// alongside any failure body emitted below.
	for _, body := range []string{tc.Stdout, tc.Stderr} {
		if err := emitLines(enc, ts, pkg, tc.Name, body); err != nil {
			return caseResult{}, err
		}
	}

	action, label, f := outcome(tc)
	if f != nil {
		if err := emitFailure(enc, ts, pkg, tc.Name, label, f); err != nil {
			return caseResult{}, err
		}
	}

	elapsed := parseSeconds(tc.Time)
	if err := enc.Encode(event(ts, action, pkg, tc.Name, "", elapsed)); err != nil {
		return caseResult{}, err
	}
	return caseResult{elapsed: elapsed, failed: action == "fail"}, nil
}

// outcome maps a JUnit testcase to a go test action ("pass" / "fail" /
// "skip"), the human-facing label, and the failure payload (nil for pass).
func outcome(tc junitCase) (action, label string, f *junitFailure) {
	switch {
	case tc.Failure != nil:
		return "fail", "FAIL", tc.Failure
	case tc.Error != nil:
		return "fail", "ERROR", tc.Error
	case tc.Skipped != nil:
		return "skip", "SKIP", tc.Skipped
	}
	return "pass", "", nil
}

func emitFailure(enc *json.Encoder, ts, pkg, test, label string, f *junitFailure) error {
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s: %s\n", label, test)
	if f.Type != "" || f.Message != "" {
		fmt.Fprintf(&b, "    %s: %s\n", f.Type, f.Message)
	}
	if body := strings.TrimRight(f.Body, "\n"); body != "" {
		for _, line := range strings.Split(body, "\n") {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}
	return emitLines(enc, ts, pkg, test, b.String())
}

// emitLines splits body into lines and emits one "output" event per line,
// preserving go test's line-by-line streaming convention. No-op on empty.
func emitLines(enc *json.Encoder, ts, pkg, test, body string) error {
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return nil
	}
	for _, line := range strings.Split(body, "\n") {
		if err := enc.Encode(event(ts, "output", pkg, test, line+"\n", 0)); err != nil {
			return err
		}
	}
	return nil
}

func event(ts, action, pkg, test, output string, elapsed float64) goTestEvent {
	return goTestEvent{Time: ts, Action: action, Package: pkg, Test: test, Output: output, Elapsed: elapsed}
}

// parseSuites accepts both a <testsuites>-rooted document and a bare
// <testsuite> document — terraform test 1.14 writes the latter when
// -filter is used. Mirror of suite_test.go's parseJUnitCases.
func parseSuites(data []byte) ([]junitSuite, error) {
	var report junitReport
	if err := xml.Unmarshal(data, &report); err == nil && len(report.Suites) > 0 {
		return report.Suites, nil
	}
	var single junitSuite
	if err := xml.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf("xml parse: %w", err)
	}
	return []junitSuite{single}, nil
}

func parseSeconds(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "tfreport: "+format+"\n", args...)
	os.Exit(1)
}
