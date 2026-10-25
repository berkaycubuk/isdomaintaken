package main

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/berkaycubuk/isdomaintaken/internal/check"
	"github.com/berkaycubuk/isdomaintaken/internal/cli"
)

// CodeTaken is returned when every lookup succeeded but at least one domain is registered.
const CodeTaken cli.ExitCode = 3

var (
	outputFormat string
	verbose      bool
	quiet        bool

	// Overridable in tests.
	bootstrapURL = check.IANABootstrapURL
	newWhois     = check.NewWhois
	cacheFile    = cachePath

	// version is set at build time with -ldflags "-X main.version=...".
	version = "dev"
)

func newRootCmd() *cobra.Command {
	var (
		tlds          []string
		concurrency   int
		timeout       time.Duration
		availableOnly bool
	)

	cmd := &cobra.Command{
		Use:   "isdomaintaken [domain|name ...]",
		Short: "Check whether domain names are registered",
		Long: `Check whether domain names are registered, using RDAP with a WHOIS
fallback for TLDs that have no RDAP service.

Arguments containing a dot are checked as-is. Bare names are expanded across
--tlds. With no arguments (or "-"), names are read from stdin, one per line.

Exit codes: 0 all available, 1 bad input, 2 a lookup failed, 3 something is taken.

Note: "available" means not registered. Premium or registry-reserved names can
still show as available but cost more, or not be purchasable at all.`,
		Example: `  isdomaintaken example.com
  isdomaintaken myproject --tlds com,dev,io,app
  cat ideas.txt | isdomaintaken --tlds com,dev --available
  isdomaintaken foo.de -o json`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cli.Stdout(cmd, outputFormat)
			log := cli.Stderr(verbose, quiet)

			inputs, err := collectInputs(cmd, args)
			if err != nil {
				return err
			}
			if len(inputs) == 0 {
				return cmd.Help()
			}
			domains, err := check.Expand(inputs, tlds)
			if err != nil {
				return cli.Exitf(cli.CodeError, "%v", err)
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			client := &http.Client{Timeout: timeout}
			boot, err := check.LoadBootstrap(ctx, client, bootstrapURL, cacheFile(), 24*time.Hour)
			if err != nil {
				return cli.WrapFatal(err, "cannot load RDAP bootstrap")
			}

			c := &check.Checker{HTTP: client, Bootstrap: boot, Whois: newWhois(timeout), Timeout: timeout}
			c.OnThrottle = func(server string, every time.Duration) {
				if u, err := url.Parse(server); err == nil {
					server = u.Host
				}
				log.Info("%s is rate-limiting; slowing to one request every %s", server, every)
			}
			log.Debug("checking %d domain(s), concurrency %d", len(domains), concurrency)
			results := c.CheckAll(ctx, domains, concurrency)

			var taken, failed int
			shown := make([]check.Result, 0, len(results))
			for _, r := range results {
				switch r.Status {
				case check.StatusTaken:
					taken++
				case check.StatusUnknown:
					failed++
					log.Warn("%s: %s", r.Domain, r.Error)
				}
				log.Debug("%s: %s via %s", r.Domain, r.Status, r.Source)
				if !availableOnly || r.Status == check.StatusAvailable {
					shown = append(shown, r)
				}
			}

			if err := printResults(out, shown); err != nil {
				return cli.WrapFatal(err, "write output")
			}

			switch {
			case failed > 0:
				return cli.Exitf(cli.CodeFatal, "%d of %d lookup(s) failed", failed, len(results))
			case taken > 0:
				return &cli.Error{Code: CodeTaken}
			}
			return nil
		},
	}

	cli.RegisterOutputFlags(cmd, &outputFormat)
	cli.RegisterLogFlags(cmd, &verbose, &quiet)
	cmd.Flags().StringSliceVarP(&tlds, "tlds", "t", []string{"com"}, "TLDs to try for bare names (comma-separated)")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "c", 8, "Maximum lookups in flight")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "Timeout per lookup")
	cmd.Flags().BoolVarP(&availableOnly, "available", "a", false, "Only print available domains")

	cmd.Version = version
	return cmd
}

// collectInputs returns args, or stdin lines when args are empty or "-".
// An interactive terminal on stdin with no args yields nothing (shows help).
func collectInputs(cmd *cobra.Command, args []string) ([]string, error) {
	if len(args) > 0 && !(len(args) == 1 && args[0] == "-") {
		return args, nil
	}
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok && len(args) == 0 {
		if st, err := f.Stat(); err == nil && st.Mode()&os.ModeCharDevice != 0 {
			return nil, nil
		}
	}
	return readLines(in)
}

func readLines(r io.Reader) ([]string, error) {
	var lines []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, strings.Fields(line)...)
	}
	if err := sc.Err(); err != nil {
		return nil, cli.WrapFatal(err, "read stdin")
	}
	return lines, nil
}

func printResults(out *cli.Output, results []check.Result) error {
	if out.IsJSON() {
		return out.Print(results)
	}
	rows := make([][]interface{}, len(results))
	for i, r := range results {
		rows[i] = []interface{}{r.Domain, r.Status, dash(r.Expires), dash(r.Registrar)}
	}
	return out.PrintTable([]string{"DOMAIN", "STATUS", "EXPIRES", "REGISTRAR"}, rows)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func cachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "isdomaintaken", "rdap-dns.json")
}
