package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// Format represents the output format for a command.
type Format string

const (
	FormatJSON  Format = "json"
	FormatPlain Format = "plain"
)

// Output writes data to stdout. It respects the output format:
// plain mode prints with fmt, json mode prints with json.MarshalIndent.
type Output struct {
	w      io.Writer
	format Format
}

// NewOutput returns an Output writing to w in the given format.
func NewOutput(w io.Writer, format Format) *Output {
	return &Output{w: w, format: format}
}

// Stdout returns an Output writing to the command's output writer,
// respecting the --output flag value. In production this is os.Stdout.
// In tests, cmd.SetOut() redirects it to a buffer.
func Stdout(cmd *cobra.Command, format string) *Output {
	f := FormatPlain
	if format == "json" {
		f = FormatJSON
	}
	return NewOutput(cmd.OutOrStdout(), f)
}

// IsJSON returns true when the output format is JSON.
func (o *Output) IsJSON() bool {
	return o.format == FormatJSON
}

// Print writes v to the output. In JSON mode, v is marshaled with indentation.
// In plain mode, fmt.Fprintln is used for strings; other types fall back to JSON.
func (o *Output) Print(v interface{}) error {
	if o.format == FormatJSON {
		return o.printJSON(v)
	}
	switch val := v.(type) {
	case string:
		_, err := fmt.Fprintln(o.w, val)
		return err
	case []byte:
		_, err := o.w.Write(val)
		return err
	default:
		return o.printJSON(v)
	}
}

func (o *Output) printJSON(v interface{}) error {
	enc := json.NewEncoder(o.w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// PrintTable writes a table to the output. Headers and rows are tab-separated
// in plain mode, and a JSON array of objects in JSON mode.
func (o *Output) PrintTable(headers []string, rows [][]interface{}) error {
	if o.format == FormatJSON {
		return o.printTableJSON(headers, rows)
	}
	w := tabwriter.NewWriter(o.w, 0, 0, 2, ' ', 0)
	for _, h := range headers {
		fmt.Fprintf(w, "%s\t", h)
	}
	fmt.Fprintln(w)
	for _, row := range rows {
		for _, cell := range row {
			fmt.Fprintf(w, "%v\t", cell)
		}
		fmt.Fprintln(w)
	}
	return w.Flush()
}

func (o *Output) printTableJSON(headers []string, rows [][]interface{}) error {
	out := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		obj := make(map[string]interface{})
		for j, h := range headers {
			if j < len(row) {
				obj[h] = row[j]
			}
		}
		out[i] = obj
	}
	return o.printJSON(out)
}
