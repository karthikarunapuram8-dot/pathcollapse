# Suppress cold-start confidence INFO note in non-TTY and --quiet runs

<!--
Draft issue body. Open with:
    gh issue create --repo karthikarunapuram8-dot/pathcollapse \
      --title "Suppress cold-start confidence INFO note in non-TTY and --quiet runs" \
      --body-file .github/ISSUE_DRAFTS/01-suppress-coldstart-note.md
-->

## Problem

When `--confidence=on` (the default) runs without a local snapshot history
at `~/.pathcollapse/snapshots.db`, the CLI emits a single stderr line on
every invocation:

```
INFO: confidence: no snapshot history at ~/.pathcollapse/snapshots.db — T(e) using cold-start prior (see docs/confidence.md §4.4)
```

This is intentional for interactive first-time users — it explains the
degradation and points at the docs. But in CI pipelines, `jq`-style
pipelines, and any automated invocation of `breakpoints` or `report`, it's
noise.

## Proposal

Suppress the cold-start INFO note in either of these conditions:

1. **Non-TTY stderr.** Detect via `golang.org/x/term.IsTerminal(int(os.Stderr.Fd()))`. If stderr is not a terminal (CI, pipes, redirects), skip the note.
2. **Explicit `--quiet` flag.** Added to `breakpoints` and `report`, suppresses all INFO-level stderr lines (the cold-start note and the built-in-fixture note today).

Either condition alone suppresses. Real errors (unreadable DB, invalid flag values, etc.) are never suppressed.

## Scope

- `cmd/pathcollapse/subcmd/confidence_flag.go` — update `noteColdStart` to check both conditions.
- `cmd/pathcollapse/subcmd/breakpoints.go` + `report.go` — add `--quiet` flag, plumb into a shared context.
- Test: capture stderr in a non-TTY buffer (already how tests work) and confirm no output when `--quiet` or when stderr is not a terminal.

## Out of scope

- Changing the default behavior for interactive use. Humans still get the note.
- Structured logging. If verbosity control grows beyond two levels, revisit.

## References

- Confidence algorithm: [docs/confidence.md](../../docs/confidence.md) §4.4
- Sample CLI transcript: [docs/assets/breakpoints-confidence.txt](../../docs/assets/breakpoints-confidence.txt)
