# Brew Migrator 🍺

Port your `/Applications` to Homebrew Casks with a **Charm-powered TUI**.

```
  scan  →  match  →  select  →  install
```

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), [Lip Gloss](https://github.com/charmbracelet/lipgloss), and [Huh](https://github.com/charmbracelet/huh).

## Features

- **Live scanning** — real-time table updates as apps are matched to casks
- **Smart auto-selection** — score >90 → ⭐ near perfect pre-checked, 50–90 → 🤔 maybe unchecked, <50 → ❓ questionable unchecked
- **Keyboard-driven** — `↓↑` navigate, `space` toggle, `enter` confirm
- **Progress bars** — animated installation with gradient progress
- **History** — persistent JSON-backed migration history
- **All Charm** — unified design system with lipgloss styling, spinners, borders, and huh confirmations

## Quick start

```bash
# Install
go install github.com/ahmetyilmaz/brew-migrator@latest

# Or build from source
git clone https://github.com/ahmetyilmaz/brew-migrator.git
cd brew-migrator
go build -o bin/brew-migrator .
./bin/brew-migrator
```

**Requires:** macOS + [Homebrew](https://brew.sh)

## How it works

1. **Scan** — discovers every `.app` bundle in `/Applications`
2. **Match** — runs `brew search --cask` for each app, scores candidates by name similarity
3. **Select** — presents an interactive checkbox list; `↓↑` to move, `space` to toggle
4. **Install** — `brew install --cask` with a live progress bar
5. **History** — browsable record of every migration attempt

## Visual design

| Screen | Description |
|--------|-------------|
| Menu   | Styled menu with cursor, live plan/history counts |
| Scan   | Live-updating table, spinner, status badges |
| Select | Checkbox list with quality emoji indicators |
| Install | Gradient progress bar, real-time result log |
| Summary | Success/fail breakdown |
| History | Table of past migrations, sorted newest first |

## Thresholds

| Score | Label | Auto-selected |
|-------|-------|:---:|
| >90   | ⭐ near perfect | ✅ |
| 50–90 | 🤔 maybe | ❌ |
| <50   | ❓ questionable | ❌ |
| no match | ⏭ skipped | ❌ (disabled) |

## Architecture

```
main.go      — program entry, tea.NewProgram
tui.go       — all TUI screens (menu, scan, select, install, summary, history)
styles.go    — lipgloss colour palette and reusable style definitions
brew.go      — Homebrew ops, app scanner, fuzzy matching, scoring, history
```

## Tech stack

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — Elm-architecture TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — table, spinner, progress, viewport, help components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — terminal styling
- [Huh](https://github.com/charmbracelet/huh) — confirmation dialogs
