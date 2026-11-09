# Terminal Alpha Implementation Plan

**Goal:** A stable terminal workflow with understandable choices and correct managed-app detection.
**Architecture:** One Bubble Tea model; JSON-backed Homebrew inventory; bounded rendering.
**Tech stack:** Go, Bubble Tea, Lip Gloss, Homebrew JSON.

Work in the existing checkout, preserving the user's build/run location.

- [x] Add failing regression tests in brew_test.go and tui_test.go: installed
  apps are never selected, app artifact aliases resolve, cursor movement never
  changes selections, left/right changes only the focused candidate, and views
  fit 80x24, 160x50, 40x12, and tiny terminals.
- [x] Fix brew.go inventory loading and candidate metadata. Refresh per scan,
  report failures, use JSON for installed and search info, and recheck before install.
- [x] Replace nested terminal programs in tui.go with selection and review states.
  Keep stable row labels, textual status/legend, visible candidate choices,
  full scrollable review, and explicit install action. Bound every screen.
- [x] Run regression suite and race detector, vet, formatting, cross builds,
  PTY resize/navigation smoke tests, and read-only local installed detection.
- [ ] Review final diff, update README and release notes, commit, push, publish
  next alpha with macOS archives/checksums, and verify Actions/release assets.

Review focus: narrow/short terminals; installed inventory failure; aliases whose
bundle differs from token; mixed managed/unmanaged candidates; cancelled scans;
plan list longer than the viewport. Tests own each of these cases.

Validation ledger: regression tests reproduced the original managed-app and
navigation bugs, malformed inventory, hidden small-window install actions,
truncated review identifiers, unsafe review resize, and missing adoption flag.
Each now passes. Read-only inventory checks confirmed Pearcleaner, Tailscale,
and Raspberry Pi Imager are excluded. PTY checks covered scan, navigation,
alternatives, review, resize, and quit without invoking installation.
Independent review findings were fixed with regression coverage.

Ruling: terminal minimum is 72x24, with a resize notice and blocked action keys
below it; this preserves the legend, choices, review controls, and visible errors.
Homebrew adoption is used to handle existing identical application artifacts
without forced replacement. Real installation remains untested in this task.
