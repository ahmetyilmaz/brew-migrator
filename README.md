# Homebrew App Migrator

[![PyPI version](https://img.shields.io/pypi/v/brew-migrator.svg)](https://pypi.org/project/brew-migrator/)
[![Python versions](https://img.shields.io/pypi/pyversions/brew-migrator.svg)](https://pypi.org/project/brew-migrator/)
[![License](https://img.shields.io/github/license/ahmetyilmaz/brew-migrator.svg)](https://github.com/ahmetyilmaz/brew-migrator/blob/main/LICENSE)

A charm-style terminal tool to migrate manually installed macOS applications from `/Applications` to Homebrew Casks.

## Why this tool?

If you've ever bought a new Mac and spent hours manually downloading `.dmg` files, dragging them to `/Applications`, and trying to remember everything you had installed, this tool is for you.

**Homebrew** is the gold standard for managing software on macOS, but it only works if your apps are installed as "Casks." Most people have a mix of apps they downloaded from websites and apps installed via Homebrew.

### The Power of the Brewfile

Once your apps are migrated to Homebrew Casks using this tool, you can run:

```bash
brew bundle dump
```

This generates a `Brewfile`. This single file is like a "save game" for your entire Mac setup. On a new machine, you just run `brew bundle` and **every single app** is automatically installed for you in minutes.

## Features

- **Plan by default**: `brew-migrator` now creates a repeatable plan first, installs second.
- **Single plan per PC**: one plan file in the same folder as history; reruns merge, not overwrite.
- **Resume from history**: skipped/ignored/failed apps continue later; new apps get matched on reruns.
- **Charm-style UI**: animated welcome walkthrough, flow badges, rich tables, confirmation prompts for trust actions.
- **Interactive selection**: inspect candidates, force-install safely, ignore permanently, quit gracefully.
- **Official tap preference**: prefers `homebrew/cask` candidates when available.
- **Migration history**: remembers migrated/skipped/ignored/failed so later runs avoid double-work.

## Installation

### Via PyPI

PyPI currently serves the upstream package. To get this fork's changes, install from this repo instead.

### From this fork (recommended)

```bash
git clone https://github.com/ahmetyilmaz/brew-migrator.git
cd brew-migrator
python3 -m venv .venv
source .venv/bin/activate
pip install -U pip
pip install -e .
brew-migrator --help
```

Or install directly without cloning:

```bash
pip install git+https://github.com/ahmetyilmaz/brew-migrator.git
```

## Requirements

- macOS with [Homebrew](https://brew.sh/) installed
- Python 3.10+
- `pip` that can install `rich`, `questionary`, and `alive-progress`

## Usage

After installation, you can run the tool using the `brew-migrator` command:

```bash
brew-migrator [OPTIONS]
```

### Options

- `--plan`: Create or update the plan without installing.
- `--apply-plan <path>`: Install from a saved plan file.
- `--interactive`: Go straight to interactive installs for this run.
- `--batch`: Install the best match for every eligible app automatically.
- `--app "AppName"`: Process one app only.
- `--list-apps`: List apps found in `/Applications`.
- `--list-installed`: List migrated apps.
- `--list-skipped`: List skipped apps.
- `--list-ignored`: List ignored apps.
- `--retry-skipped`: Retry skipped apps.
- `--retry-ignored`: Retry ignored apps.
- `--retry-failed`: Retry previously failed apps.
- `--reset-history`: Clear migration history.
- `--force`: Allow `brew install --cask --force`. Explicit opt-in.
- `--applications-folder <path>`: Use a custom apps folder.

### Examples

**Default plan workflow:**

```bash
brew-migrator
```

This creates/updates a plan, prints the match table, and stores it in one place.

**Install from plan later:**

```bash
brew-migrator --apply-plan
```

This uses the single plan file saved for this machine. You can still pass an explicit path if needed.

**List apps:**

```bash
brew-migrator --list-apps
```

**Run batch install:**

```bash
brew-migrator --batch
```

## How it Works

1. **Plan**: the tool reads `/Applications`, finds cask candidates, scores matches, and updates a single plan file for this machine.
2. **Review**: reruns are safe — history skips already migrated items; unmatched/new apps are picked up automatically.
3. **Apply**: install with `--apply-plan` or use `--interactive` to confirm one-by-one.
4. **Remember**: migrated/skipped/ignored/failed states are persisted so future runs resume properly.

## Plan behavior
- Default mode is plan-first.
- One plan per PC is maintained automatically: reruns merge updated matches instead of starting over.
- History is **never** modified in plan mode.
- If installations fail after applying a plan, rerun `brew-migrator --apply-plan`; migrated apps are skipped automatically and only remaining items retry.
- New apps installed outside the tool appear on the next `brew-migrator` run and are matched against the current plan/history state.

## Force install opt-in
- By default, installs use standard cask install.
- Use `--force` only when you need `--force` semantics from Homebrew.
- The UI confirms before any force action.

## Project Structure

```text
.
├── LICENSE
├── README.md
├── pyproject.toml
└── src
    └── brew_migrator
        ├── cli.py
        ├── core
        │   ├── brew.py
        │   └── history.py
        └── ui
            └── console.py
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
