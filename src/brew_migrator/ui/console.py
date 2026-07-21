"""Spectacular TUI layer built on rich + questionary + alive-progress.

This module is the primary user-facing surface. It keeps backwards-compatible
shims for the original retro API referenced from core/* (history.py, brew.py).
"""
from __future__ import annotations

import json
import os
import shutil
import sys
import time
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Sequence, Tuple

from rich.console import Console
from rich.panel import Panel
from rich.table import Table
from rich.text import Text
from rich.markdown import Markdown
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn
from rich.align import Align
from rich.box import (
    HEAVY,
    HEAVY_EDGE,
    MINIMAL,
    SIMPLE_HEAD,
    SQUARE,
    DOUBLE,
    ROUNDED,
)

try:
    import questionary
except Exception:  # pragma: no cover
    questionary = None

_APPLICATIONS_FOLDER = "/Applications"
HISTORY_PATH = Path.home() / ".brew_migrator_history"
PLAN_PATH = Path.home() / ".brew_migrator_plan.json"


# ---------------------------------------------------------------------------
# Color / output helpers
# ---------------------------------------------------------------------------

def _use_colors() -> bool:
    if os.environ.get("NO_COLOR") == "1" or os.environ.get("NO_ANSI") == "1":
        return False
    if hasattr(sys.stdout, "isatty") and not sys.stdout.isatty():
        return False
    return True


console = Console(force_terminal=_use_colors(), color_system="standard")
err_console = Console(stderr=True, force_terminal=_use_colors(), color_system="standard")

BRAND = "bold cyan"
GOOD = "bold green"
WARN = "bold yellow"
BAD = "bold red"
MUTED = "dim"
RED = "bold red"


def _t(text: str) -> Text:
    return Text.from_markup(text)


def retro_print(text: str, style: str = "", err: bool = True) -> None:
    """Compatibility shim for core/*."""
    target = err_console if err else console
    if not _use_colors():
        target.print(text)
        return
    style_name = (style or "").lower()
    if "red" in style_name:
        target.print(Text(text, style="bold red"))
    else:
        target.print(text)


class ProgressBar:
    """Compatibility shim used by core/brew.py."""

    def __init__(self, label: str) -> None:
        self.label = label

    def __enter__(self) -> "ProgressBar":
        console.print(f"[bold cyan]{self.label}[/bold cyan]")
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        if exc is None:
            console.print(f"[bold green]done[/bold green] {self.label}")
        else:
            console.print(f"[bold red]failed[/bold red] {self.label}")


def boot_spinner(label: str = "Brew Migrator", duration: float = 0.6) -> None:
    frames = [
        f"{label} ●   ",
        f"{label} ●●  ",
        f"{label} ●●● ",
        f"{label} ●●●●",
    ]
    delay = max(duration / max(len(frames), 1), 0.03)
    for frame in frames:
        sys.stdout.write(f"\r{frame}")
        sys.stdout.flush()
        time.sleep(delay)
    sys.stdout.write("\r")
    sys.stdout.flush()


def clear_screen() -> None:
    os.system("cls" if os.name == "nt" else "clear")


# ---------------------------------------------------------------------------
# Rich panels / tables
# ---------------------------------------------------------------------------

def title_block(version: str = "v0.0.3") -> Panel:
    body = Text.assemble(
        ("Homebrew App Migrator ", "bold white"),
        (version, "green"),
        ("\n", ""),
        ("Mac  ", "dim"),
        ("→ ", "dim"),
        ("Homebrew Cask ", "bold cyan"),
        ("× ", ""),
        ("Charm TUI ", "bold magenta"),
    )
    return Panel(
        Align.center(body, vertical="middle"),
        title=_t("[bold cyan]BREW MIGRATOR[/bold cyan]"),
        subtitle=_t("[dim]scan → match → select → install[/dim]"),
        border_style="cyan",
        box=HEAVY_EDGE,
        padding=(0, 3),
    )


def welcome_panel() -> Panel:
    md = "\n".join(
        [
            "Welcome to **Brew Migrator**.",
            "",
            "- Scan **/Applications** for `.app` bundles",
            "- Match every app to Homebrew Casks",
            "- Autoselect top matches, review in bulk, then install",
            "- Browse history and retry failures",
            "",
            "*All paths are local. History is kept on disk.*",
        ]
    )
    return Panel(
        Markdown(md),
        title=_t("[bold white]Hi![/bold white]"),
        subtitle=_t("[dim]this tool only works on macOS with Homebrew[/dim]"),
        border_style="magenta",
        box=DOUBLE,
        padding=(1, 2),
    )


def animated_boot(seconds: float = 0.8) -> None:
    with Progress(
        SpinnerColumn(),
        TextColumn("[bold yellow]{task.description}[/bold yellow]"),
        BarColumn(bar_width=None),
        console=console,
        transient=True,
    ) as progress:
        task_id = progress.add_task("Loading workspace", total=1)
        time.sleep(max(0.05, seconds * 0.35))
        progress.advance(task_id)
        time.sleep(max(0.05, seconds * 0.35))
        progress.advance(task_id)
        time.sleep(max(0.05, seconds * 0.35))


def trust_banner(text: str, kind: str = "default") -> Panel:
    styles = {
        "force": "bold red",
        "danger": "bold yellow",
        "default": "bold magenta",
    }
    style = styles.get(kind, "bold magenta")
    return Panel(
        _t(f"⚠️  {text}"),
        title=_t("[bold white]Trust action[/bold white]"),
        border_style=style,
        box=SQUARE,
        padding=(0, 1),
    )


def preview_panel(app: str, candidate: dict, force: bool = False, dry_run: bool = False) -> Panel:
    tap = candidate.get("tap", "")
    desc = candidate.get("description", "")
    version = candidate.get("version", "")
    installed = candidate.get("installed", "")
    official = tap.lower().startswith("homebrew/")
    official_md = ":white_check_mark:" if official else ":x:"

    command = candidate.get("command", ["brew", "install", "--cask", candidate.get("name", app)])
    cmd = " ".join(command)

    body = Text.from_markup(
        "\n".join(
            [
                f":robot: **{app}** → :package: **{candidate.get('name', app)}**",
                f":test_tube: Version: [green]{version or 'unknown'}[/green]",
                f":movie_camera: Tap: [magenta]{tap or 'unknown'}[/magenta]",
                f":house_with_garden: Official cask: [green]{official_md}[/green]",
                f":construction_worker: Already installed: {'[green]yes[/green]' if installed else '[red]no[/red]'}",
                f":shield: Force install: {'[red]yes[/red]' if force else '[green]no[/green]'}",
                f":notebook: Dry run: {'[green]yes[/green]' if dry_run else '[red]no[/red]'}",
                "",
                f":computer: **Command**\n[cyan]{cmd}[/cyan]",
            ]
        ),
    )
    return Panel(
        body,
        title=_t("[bold white]Action preview[/bold white]"),
        subtitle=_t("[dim]Read carefully before confirming.[/dim]"),
        border_style="yellow",
        box=SQUARE,
        padding=(0, 2),
    )


def confirmation_prompt(message: str, action: str = "Confirm", danger: bool = False) -> bool:
    if questionary is None:
        choice = input(f"{action}: {message} [y/N]: ").strip().lower()
        return choice in ("y", "yes")
    style = questionary.Style(
        [
            ("question", "bold yellow"),
            ("answer", "bold green"),
            ("highlighted", "bold cyan"),
        ]
    )
    q = questionary.confirm(
        message,
        default=False,
        style=style,
        qmark="⚠️",
    )
    if q is None:
        return False
    return bool(q.ask())


def list_apps(title: str, items: Sequence[str], count: Optional[int] = None) -> None:
    if count is None:
        count = len(items)
    table = Table(title=title, box=SIMPLE_HEAD)
    table.add_column("#", justify="right", style="dim", width=5)
    table.add_column("App", style="cyan", no_wrap=True)
    for idx, item in enumerate(items, 1):
        table.add_row(str(idx), item)
    caption = Text.from_markup(f":mag_right: Found [green]{count}[/green] apps")
    console.print(table)
    console.print(caption)


def match_table(title: str, matches: Sequence[dict], start_idx: int = 0, page_size: int = 6) -> Panel:
    end_idx = min(start_idx + page_size, len(matches))
    table = Table(box=SIMPLE_HEAD, row_styles=["on black", ""])
    table.add_column("#", style="dim", width=4)
    table.add_column("Cask", style="cyan", no_wrap=True)
    table.add_column("Score", justify="right", style="green", width=7)
    table.add_column("Tap", style="magenta")
    table.add_column("Version", style="yellow")
    table.add_column("Installed", justify="center")
    table.add_column("Description", style="dim")

    for idx in range(start_idx, end_idx):
        match = matches[idx]
        official = match.get("tap", "").lower().startswith("homebrew/")
        desc = match.get("description", "")
        if len(desc) > 55:
            desc = desc[:55] + "…"
        table.add_row(
            str(idx + 1),
            Text(match.get("name", ""), style="green" if official else "white"),
            str(match.get("score", "")),
            match.get("tap", "") or "—",
            match.get("version", "") or "—",
            Text("yes" if match.get("installed") else "no", style=GOOD if match.get("installed") else BAD),
            desc,
        )
    return Panel(table, title=title, border_style="green", box=ROUNDED)


def format_dry_run_candidates(
    app_candidates: Sequence[dict],
) -> List[dict]:
    scored: List[dict] = []
    for c in app_candidates:
        if not isinstance(c, dict):
            continue
        cm = c.get("command")
        if isinstance(cm, list):
            cmd = " ".join(str(part) for part in cm)
        else:
            cmd = str(c.get("command", ""))
        scored.append(
            {
                "cask": c.get("name", ""),
                "score": c.get("score", 0),
                "tap": c.get("tap", ""),
                "installed": c.get("installed", ""),
                "version": c.get("version", ""),
                "cmd": cmd,
            }
        )
    scored.sort(key=lambda row: row.get("score", 0), reverse=True)
    return scored


def display_dry_run_plan(
    title: str,
    planned_rows: Sequence[Tuple[str, dict]],
    export_path: Optional[str] = None,
) -> None:
    if not planned_rows:
        console.print(_t(f":warning: {title}: (None found)"), style=WARN)
        return
    table = Table(title=title, box=HEAVY, expand=True)
    table.add_column("App", style="bold cyan", no_wrap=True)
    table.add_column("Proposed Cask", style="white", no_wrap=True)
    table.add_column("Score", justify="right", style=GOOD, width=8)
    table.add_column("Official", justify="center")
    table.add_column("Installed", justify="center")
    table.add_column("Version", style="yellow")
    table.add_column("Command", style="dim")
    for app, row in planned_rows:
        official = "yes" if row.get("tap", "").lower().startswith(
            ("homebrew/cask/", "homebrew/cask-versions/", "homebrew/cask-fonts/")
        ) else "no"
        table.add_row(
            app,
            row.get("cask", "(none)"),
            str(row.get("score", "")),
            official,
            row.get("installed", ""),
            row.get("version", ""),
            row.get("cmd", ""),
        )
    console.print(table)
    if export_path:
        console.print(_t(f":memo: Plan exported to: {export_path}"), style=MUTED)


def display_retro_list(title: str, items: Iterable[str], color: str = BRAND) -> None:
    sorted_items = sorted(items)
    term_width = getattr(console, "size", None)
    col_min = 24
    term_width = term_width.width if term_width else shutil.get_terminal_size((80, 20)).columns
    num_cols = max(1, term_width // col_min)
    rows: List[List[str]] = [[] for _ in range(num_cols)]
    for idx, item in enumerate(sorted_items):
        rows[idx % num_cols].append(f"▸ {item}")
    num_rows = max((len(col) for col in rows), default=0)
    for row_idx in range(num_rows):
        line = ""
        for col in rows:
            if row_idx < len(col):
                line += col[row_idx].ljust(col_min)
        console.print(line.rstrip(), style=color)


class AnimatedWait:
    def __init__(self, label: str = "Working") -> None:
        self.label = label
        self._progress = None

    def __enter__(self):
        if questionary is not None:
            console.print(f"[bold yellow]:hourglass:  {self.label}[/bold yellow]")
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        if exc_type is None:
            console.print(f"[bold green]:white_check_mark:  {self.label} complete.[/bold green]")
        else:
            console.print(f"[bold red]:warning:  {self.label} ended with an issue.[/bold red]")
        return False


# ---------------------------------------------------------------------------
# Plan persistence
# ---------------------------------------------------------------------------

def _load_plan(plan_path: Path = PLAN_PATH) -> Dict[str, Any]:
    if not plan_path.exists():
        return {}
    try:
        return json.loads(plan_path.read_text(encoding="utf-8")) or {}
    except Exception:
        return {}


def _save_plan(payload: Dict[str, Any], plan_path: Path = PLAN_PATH) -> None:
    plan_path.parent.mkdir(parents=True, exist_ok=True)
    plan_path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")


def load_planned_rows(plan_path: Path = PLAN_PATH) -> List[Tuple[str, dict]]:
    plan = _load_plan(plan_path)
    rows = []
    for row in plan.get("rows", []):
        app = row.get("app", "")
        if app:
            rows.append((app, row))
    return rows


def merge_planned_rows(rows: List[Tuple[str, dict]], plan_path: Path = PLAN_PATH) -> List[Tuple[str, dict]]:
    existing = {str(r.get("app", "")): r for r in _load_plan(plan_path).get("rows", []) if isinstance(r, dict)}
    merged = dict(existing)
    for app, row in rows:
        merged[app] = {
            "app": app,
            "cask": row.get("cask", ""),
            "score": row.get("score", 0),
            "tap": row.get("tap", ""),
            "installed": row.get("installed", ""),
            "version": row.get("version", ""),
            "cmd": row.get("cmd", ""),
        }
    return [(app, merged[app]) for app in sorted(merged.keys()) if merged[app].get("cask")]


def upsert_planned_rows(rows: List[Tuple[str, dict]], plan_path: Path = PLAN_PATH) -> None:
    payload = {
        "generated_at": datetime.now().isoformat(),
        "applications_folder": _APPLICATIONS_FOLDER,
        "rows": [r for _, r in merge_planned_rows(rows, plan_path)],
    }
    _save_plan(payload, plan_path)


# ---------------------------------------------------------------------------
# TUI helpers
# ---------------------------------------------------------------------------

def _candidate_css(row: dict) -> str:
    official = row.get("tap", "").lower().startswith(
        ("homebrew/cask/", "homebrew/cask-versions/", "homebrew/cask-fonts/")
    )
    installed = row.get("installed") == "yes"
    score = row.get("score", 0)
    if installed:
        return "[green]● installed[/green]"
    if score >= 40 and official:
        return "[bold green]● top[/bold green]"
    if official:
        return "[green]● official[/green]"
    return "[yellow]● unofficial[/yellow]"


def _plan_value(app: str, row: Optional[dict]) -> str:
    if not row:
        return ""
    return row.get("cask", "(none)")


def _plan_checkbox_label(app: str, row: Optional[dict]) -> str:
    if not row:
        return f"{app} [dim red](no match)[/dim red]"
    label = row.get("cask", "(none)")
    css = _candidate_css(row)
    return f"{app}  →  {label}  {css}"


def main_menu(plan_size: int = 0, history_size: int = 0) -> str:
    console.clear()
    console.print(Align.center(Text("🍺  Brew Migrator", style="bold cyan")))
    console.print(Align.center(Text("scan  →  match  →  select  →  install", style=MUTED)))
    console.print()
    choices = [
        questionary.Choice(
            title=f"🔍  Scan & Plan  {'(' + str(plan_size) + ' ready)' if plan_size else '(fresh)'}",
            value="scan",
        ),
        questionary.Choice(
            title=f"🚀  Install Selected Plan  {'(' + str(plan_size) + ')' if plan_size else '(no plan yet)'}",
            value="install",
            disabled=(not plan_size),
        ),
        questionary.Choice(
            title=f"👀  View History  {'(' + str(history_size) + ')' if history_size else '(empty)'}",
            value="history",
            disabled=(not history_size),
        ),
        questionary.Choice(
            title=f"♻️  Retry Failed / Skipped",
            value="retry",
        ),
        questionary.Choice(
            title="🚪  Quit",
            value="quit",
        ),
    ]
    style = questionary.Style(
        [
            ("question", "bold yellow"),
            ("answer", "bold green"),
            ("highlighted", "bold cyan"),
        ]
    )
    if questionary is None:
        while True:
            print("\n".join(f"{i+1}. {c.title}" for i, c in enumerate(choices)))
            picked = input("Choose: ").strip()
            if picked.isdigit() and 1 <= int(picked) <= len(choices):
                return choices[int(picked) - 1].value
            continue
    return questionary.select(
        "What next?",
        choices=choices,
        style=style,
        qmark="✨",
        use_shortcuts=True,
        use_jk_keys=True,
        instruction="(up/down/enter)",
    ).ask() or "quit"


# ---------------------------------------------------------------------------
# History / plan tables used by the TUI
# ---------------------------------------------------------------------------

def history_table(history: Dict[str, dict]) -> Table:
    table = Table(title="Migration History", box=HEAVY_EDGE, expand=True)
    table.add_column("App", style="bold cyan", no_wrap=True)
    table.add_column("Status", style="green", no_wrap=True)
    table.add_column("Detail", style="white", no_wrap=False)
    table.add_column("Updated", style="magenta", no_wrap=True)
    for app in sorted(history.keys()):
        entry = history[app]
        status = entry.get("status", "")
        style = GOOD if status.startswith("migrated") else (BAD if status == "failed" else WARN)
        table.add_row(
            app,
            Text(status, style=style),
            entry.get("detail", ""),
            entry.get("timestamp", ""),
        )
    return table


def plan_table(planned: List[Tuple[str, dict]]) -> Table:
    table = Table(title="Plan", box=HEAVY_EDGE, expand=True)
    table.add_column("App", style="bold cyan", no_wrap=True)
    table.add_column("Match", style="white", no_wrap=True)
    table.add_column("Score", justify="right", style=GOOD, width=7)
    table.add_column("Tap", style="magenta")
    table.add_column("Version", style="yellow")
    table.add_column("Installed", justify="center")
    table.add_column("Status", style=WARN)
    for app, row in planned:
        official = row.get("tap", "").lower().startswith(("homebrew/cask/", "homebrew/cask-versions/", "homebrew/cask-fonts/"))
        status = "[green]ready[/green]"
        if row.get("installed"):
            status = "[green]installed[/green]"
        elif not row.get("cask"):
            status = "[red]no match[/red]"
        table.add_row(
            app,
            row.get("cask", "(none)"),
            str(row.get("score", "")),
            row.get("tap", "") or "—",
            row.get("version", "") or "—",
            "yes" if row.get("installed") else "no",
            status,
        )
    return table


def summary_table(newly_migrated, failed, skipped, ignored) -> Table:
    table = Table(title="Session Summary", box=HEAVY_EDGE, expand=True)
    table.add_column("Category", style="bold cyan", no_wrap=True)
    table.add_column("Count", justify="right", style=GOOD, width=6)
    table.add_column("Items", style="white", no_wrap=False)
    for label, items in [
        ("Migrated", newly_migrated),
        ("Failed", failed),
        ("Skipped", skipped),
        ("Ignored", ignored),
    ]:
        text_color = GOOD if label == "Migrated" else (BAD if label == "Failed" else WARN)
        table.add_row(
            Text(label, style=text_color),
            Text(str(len(items)), style=text_color),
            Text(", ".join(items[:8]) if items else "—"),
        )
    return table
