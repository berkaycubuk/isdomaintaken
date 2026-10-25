package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// CmdResult captures the output and exit status of a CLI command run in a test.
type CmdResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// RunTest builds the isdomaintaken binary (if needed) and runs it with args,
// returning captured stdout, stderr, and exit code.
// The binary is expected at bin/isdomaintaken relative to the project root.
func RunTest(t *testing.T, args ...string) CmdResult {
	t.Helper()

	bin := findBinary(t)
	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", bin, err)
		}
	}

	return CmdResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

func findBinary(t *testing.T) string {
	t.Helper()

	candidates := []string{
		filepath.Join("bin", "isdomaintaken"),
		filepath.Join("..", "bin", "isdomaintaken"),
		"isdomaintaken",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	// Fall back to go run
	t.Skip("binary not found; build with 'make build' first (skipping integration test)")
	return ""
}

// AssertGolden compares data against a golden file at testdata/<name>.golden.
// If the ACTUAL_GOLDEN env var is set, the golden file is updated instead.
func AssertGolden(t *testing.T, name string, data []byte) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}

	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v", path, err)
	}

	if !bytes.Equal(data, expected) {
		t.Errorf("output mismatch (-want +got):\n%s", diff(expected, data))
	}
}

// ExecuteCommand runs a cobra command directly in-process, capturing stdout and stderr.
// Useful for unit-testing individual commands without building the binary.
func ExecuteCommand(root *cobra.Command, args ...string) (stdout, stderr string, err error) {
	bout := new(bytes.Buffer)
	berr := new(bytes.Buffer)
	root.SetOut(bout)
	root.SetErr(berr)
	root.SetArgs(args)

	err = root.Execute()
	return bout.String(), berr.String(), err
}

func diff(a, b []byte) string {
	alines := bytes.Split(a, []byte("\n"))
	blines := bytes.Split(b, []byte("\n"))
	var buf bytes.Buffer
	max := len(alines)
	if len(blines) > max {
		max = len(blines)
	}
	for i := 0; i < max; i++ {
		var al, bl string
		if i < len(alines) {
			al = string(alines[i])
		}
		if i < len(blines) {
			bl = string(blines[i])
		}
		if al != bl {
			buf.WriteString("- " + al + "\n")
			buf.WriteString("+ " + bl + "\n")
		}
	}
	return buf.String()
}
