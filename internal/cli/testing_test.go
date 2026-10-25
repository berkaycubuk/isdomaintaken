package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestExecuteCommand(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := Stdout(cmd, "plain")
			return out.Print("hello world")
		},
	}

	stdout, stderr, err := ExecuteCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got %q", stderr)
	}
	if stdout != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", stdout)
	}
}

func TestExecuteCommandJSON(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := Stdout(cmd, "json")
			return out.Print(map[string]string{"key": "value"})
		},
	}

	stdout, stderr, err := ExecuteCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got %q", stderr)
	}
	expected := "{\n  \"key\": \"value\"\n}\n"
	if stdout != expected {
		t.Errorf("expected %q, got %q", expected, stdout)
	}
}

func TestPrintTable(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := Stdout(cmd, "plain")
			return out.PrintTable(
				[]string{"ID", "NAME"},
				[][]interface{}{
					{1, "foo"},
					{2, "bar"},
				},
			)
		},
	}

	stdout, _, err := ExecuteCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == "" {
		t.Error("expected table output, got empty")
	}
}

func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		code ExitCode
	}{
		{"error", Exitf(CodeError, "bad input"), CodeError},
		{"fatal", Exitf(CodeFatal, "disk full"), CodeFatal},
		{"wrap", Wrap(Exitf(CodeError, "inner"), "outer"), CodeError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("expected code %d, got %d", tt.code, tt.err.Code)
			}
		})
	}
}
