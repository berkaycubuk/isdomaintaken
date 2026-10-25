# isdomaintaken

[![skills.sh](https://skills.sh/b/berkaycubuk/isdomaintaken)](https://skills.sh/berkaycubuk/isdomaintaken)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Check whether domain names are registered. Uses [RDAP](https://www.icann.org/rdap)
(via the IANA bootstrap registry) and falls back to WHOIS for TLDs without RDAP, such as `.de` and `.io`.

```sh
go install github.com/berkaycubuk/isdomaintaken/cmd/isdomaintaken@latest

isdomaintaken example.com
isdomaintaken myproject --tlds com,dev,io,app  # bare names expand across TLDs
cat ideas.txt | isdomaintaken -t com,dev -a    # stdin, show only available
isdomaintaken foo.com -o json                  # machine-readable
```

```
DOMAIN            STATUS     EXPIRES     REGISTRAR
google.com        taken      2028-09-14  MarkMonitor Inc.
xq7zv9kpl2wm.dev  available  -           -
```

| Flag | Default | |
|---|---|---|
| `-t, --tlds` | `com` | TLDs for bare names |
| `-a, --available` | | only print available domains |
| `-c, --concurrency` | `8` | lookups in flight |
| `--timeout` | `10s` | per lookup |
| `-o, --output` | `plain` | `plain` or `json` |
| `-v` / `-q` | | verbose / quiet (stderr) |

**Exit codes:** `0` all available · `1` bad input · `2` a lookup failed · `3` at least one taken.

**Caveats:** "available" means *not registered*. Premium and registry-reserved names may
show as available but cost more or can't be bought. WHOIS verdicts are parsed from free-form text
and are best-effort. When a registry rate-limits (HTTP 429), requests to it are spaced out and retried
automatically, so large batches on strict registries like Google's (`.dev`, `.app`) are slow but complete.
The RDAP bootstrap file is cached for 24h in the user cache dir.

## Use with AI agents

A [skills.sh](https://skills.sh/berkaycubuk/isdomaintaken) skill ships with the tool. Install it and your coding agent
(Claude Code, Cursor, Codex, …) will know when to reach for `isdomaintaken`: checking a domain, buying one,
or brainstorming project names with free domains.

```sh
npx skills add berkaycubuk/isdomaintaken
```

What agents rely on: `-o json` emits a JSON array on stdout, diagnostics go to stderr only, and exit code `3`
means "something is taken", not a failure. The full agent guide is in [`skills/isdomaintaken/`](skills/isdomaintaken/SKILL.md).

## License

[MIT](LICENSE) © 2026 Berkay Çubuk
