# isdomaintaken

Check whether domain names are registered. Uses [RDAP](https://www.icann.org/rdap)
(via the IANA bootstrap registry) and falls back to WHOIS for TLDs without RDAP, such as `.de` and `.io`.

```sh
make install                                   # or: make build → bin/isdomaintaken

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
and are best-effort. The RDAP bootstrap file is cached for 24h in the user cache dir.

## License

[MIT](LICENSE) © 2026 Berkay Çubuk
