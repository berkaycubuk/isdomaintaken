---
name: isdomaintaken
description: Check whether domain names are registered or available using the isdomaintaken CLI (RDAP with WHOIS fallback, JSON output). Use when the user asks if a domain is taken or free, wants to buy or register a domain, is naming a project, product, startup, app, or brand and needs a matching domain, wants to brainstorm names with available .com/.dev/.io/.app domains, or asks who owns a domain or when it expires. Triggers include "is example.com taken", "is this domain available", "check domain availability", "find me an available domain", "name ideas with a free .com", "when does this domain expire", "domain for my project".
---

# isdomaintaken

`isdomaintaken` checks whether domain names are registered. It queries the registry's RDAP server (found via IANA's bootstrap registry) and falls back to WHOIS for TLDs without RDAP, such as `.de` and `.io`. It checks many names at once, and its JSON output is meant for agents.

Use it instead of web-searching or scraping registrar sites: it asks the registries directly, takes about a second, and works the same way every time.

## Install

Check first, then install if missing (requires Go):

```sh
command -v isdomaintaken || go install github.com/berkaycubuk/isdomaintaken/cmd/isdomaintaken@latest
```

If Go is not available, tell the user and link <https://github.com/berkaycubuk/isdomaintaken>.

## Usage

**Always pass `-o json` when calling from an agent.** The plain table is for humans.

```sh
isdomaintaken example.com other.dev -o json              # exact domains
isdomaintaken myproject -t com,dev,io,app -o json        # bare name → one domain per TLD
printf 'acme\nnimbus\n' | isdomaintaken -t com,dev -o json  # many names via stdin
isdomaintaken myproject -t com,dev,io -a -o json         # only list available ones
```

- Arguments containing a dot are checked as-is. URLs and `www.` are stripped; Unicode names are converted to punycode.
- Bare names (no dot) are expanded across `--tlds` / `-t` (default `com`).
- With no arguments, or `-`, names are read from stdin: one or more per line, `#` comments allowed.
- Order of results matches input order. Duplicates are dropped.

| Flag | Default | Meaning |
|---|---|---|
| `-o, --output` | `plain` | `json` for agents |
| `-t, --tlds` | `com` | TLDs for bare names, comma-separated |
| `-a, --available` | off | print only available domains |
| `-c, --concurrency` | `8` | lookups in flight |
| `--timeout` | `10s` | per-lookup timeout |
| `-q` / `-v` | off | quiet / verbose diagnostics on stderr |

## Output

stdout is a JSON array. stderr carries only warnings and errors.

```json
[
  {
    "domain": "google.com",
    "status": "taken",
    "source": "rdap",
    "registrar": "MarkMonitor Inc.",
    "created": "1997-09-15",
    "expires": "2028-09-14"
  },
  {
    "domain": "xq7zv9kpl2wm.dev",
    "status": "available",
    "source": "rdap"
  }
]
```

- `status`: `available`, `taken`, or `unknown`. `unknown` comes with an `error` field.
- `source`: `rdap` or `whois`.
- `registrar`, `created`, `expires` (`YYYY-MM-DD`): only for taken domains, only when RDAP provides them. WHOIS results do not include them.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | every domain is available |
| `1` | bad input (invalid domain, bad flag) |
| `2` | at least one lookup failed (`status: unknown`) |
| `3` | all lookups succeeded and at least one domain is taken |

**Exit code 3 is not a failure.** It just means something is taken. Read stdout either way; the JSON is complete for codes 0, 2, and 3.

## Workflows

**"Is X taken?"** Run it on the exact domain and answer directly. For taken domains, mention the registrar and expiry when present.

**Naming a project or brand.** Brainstorm 10–30 short candidate names, then check them all in one call across the TLDs the user cares about (ask, or default to `com,dev,io,app`). Use `-a` to keep the output small, and present the available options grouped by name. One batched call beats many single calls.

```sh
printf '%s\n' lumen ferro quilt tandem | isdomaintaken -t com,dev,io,app -a -o json
```

**`unknown` results.** These are usually rate limits or timeouts. Retry just those domains once with `-c 2`. If they still fail, report them as unknown rather than guessing.

## Caveats to tell the user

- "available" means **not registered**. Premium or registry-reserved names can show as available but cost much more, or not be for sale at all. Before the user buys, tell them to confirm the price at a registrar.
- WHOIS verdicts (non-RDAP TLDs) are parsed from free-form text and are best-effort.
- Availability changes. A result reflects the moment of the query.
- Don't hammer registries. Keep batches reasonable (hundreds, not tens of thousands) and leave `--concurrency` at its default.
