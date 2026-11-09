# Brew Migrator

Move applications in `/Applications` to Homebrew Casks with a keyboard-driven
terminal interface. Requires macOS and [Homebrew](https://brew.sh).

## Run

```bash
go install github.com/ahmetyilmaz/brew-migrator@latest
```

Or build locally:

```bash
go build -o bin/brew-migrator .
./bin/brew-migrator
```

Use a terminal window at least **70 columns × 20 rows**. The Velvet interface
uses lilac accents, rounded panels, emoji legends, and regular app spacing.
At 104 columns and wider, applications and the package inspector sit side by
side; narrower windows stack them. The interface adapts
to larger windows and pauses navigation and installation actions when the
window is too small. Quit remains available.

## Scan, select, review, install

1. **Scan & plan** refreshes Homebrew's installed-cask inventory, then matches
   applications. Inventory names and app artifact targets identify managed
   bundles even when their cask names differ, such as Tailscale → `tailscale-app`.
2. **Select** automatically hides verified Brew-managed apps and shows how many
   were hidden. It shows a stable app list and a separate detail panel for the
   focused app. Up/down moves the cursor without changing choices. Left/right
   cycles through its candidate packages; the panel shows the current choice,
   alternatives, description, tap, and version.
3. **Review** lists every selected app and its exact cask. Long entries wrap;
   scroll to inspect the full list. Managed applications and duplicate casks
   are excluded. Enter opens review; only **i** starts installation.
4. **Install** checks a fresh installed-cask list before each package and skips
   casks that became managed since scanning. It uses `brew install --cask --adopt`:
   Homebrew can adopt identical existing artifacts, and reports a failure if
   an existing artifact differs. No forced replacement is used.
During each installation, Velvet temporarily hands the terminal to Homebrew.
Sudo authentication uses the Mac's existing configuration: Touch ID when
available, otherwise the normal password prompt. Brew Migrator does not modify
`/etc/pam.d/sudo_local`, enable Touch ID, or install a privileged helper.
Authentication input is not copied into the log.
Live output is visible directly in the terminal. When the command finishes,
Velvet returns with its full output in the wrapped, scrollable session log.
Use Up/down or PgUp/PgDn to read logs; Home jumps to the beginning and End
shows the latest output.

The latest completed scan, candidate choices, and checkbox selections are saved
in `~/.brew_migrator_plan.json` with owner-only access. Choices are saved as
you edit them. After restarting, **Review the last plan** opens the saved plan;
installation still requires **i**, and installed casks are checked again.
An interrupted scan leaves the previous completed plan on disk. Old sessions
from versions without plan saving cannot be recovered automatically.

5. **History** records results locally in `~/.brew_migrator_history`.

If the installed inventory cannot be read, scanning stops with an error.
No match or a failed package search leaves the application unselected.

| Key | Action |
| --- | --- |
| Up/down or j/k | Move through apps, review lines, or history |
| Left/right | Change the focused app's candidate cask |
| Space | Select or deselect the focused app |
| Enter | Open the menu item or review the selected plan |
| i | Install the packages listed on the review screen |
| PgUp/PgDn, Home/End | Navigate a long app or review list |
| Esc | Return or cancel scanning |
| q or Ctrl+C | Quit the TUI; during terminal handoff, Homebrew handles keyboard input |
| Home / End in logs | First log line / follow latest output |

## Legend

| Mark | Meaning |
| --- | --- |
| `›` | Focused row |
| `☑` | Selected for the plan |
| `☐` | Available but not selected |
| `🍺` | Already managed by Homebrew; excluded from installation |
| `⏭` | No cask match |
| `⭐ strong` | Score above 90; preselected unless already managed |
| `🤔 review` | Score 50–90; inspect the package before selecting |
| `❓ uncertain` | Score below 50; inspect carefully |
| `⏭ no match` | No cask candidate was found |

The score measures name similarity, not verified publisher identity.

## Development and alpha releases

```bash
go test -race ./...
go vet ./...
go build -o bin/brew-migrator .
```

Automated tests cover inventory aliases/failures, managed-app exclusion,
selection, alternatives, review, and terminal size bounds. They use stub
Homebrew commands and do not install applications. Test the actual terminal
workflow and a real migration before relying on an alpha for a large plan.

GitHub Actions runs formatting, vet, race tests, and build checks on pushes and
pull requests. Publishing a GitHub release, including a prerelease, attaches
unsigned macOS Apple Silicon archives plus SHA-256 checksums.

The interface uses one Bubble Tea program with Lip Gloss rendering. `tui.go`
owns screens and navigation; `brew.go` owns Homebrew commands, metadata,
matching, and history; `main.go` starts the terminal program. This Go version
is not published to PyPI.

For isolated testing, `BREW_MIGRATOR_STATE_DIR` can point to a separate local
directory for both the saved plan and migration history.
