from __future__ import annotations

import os
import sys
import time
from typing import Dict, Iterable, List, Optional, Sequence, Tuple

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
)

try:
    import questionary
except Exception:  # pragma: no cover
    questionary = None

_APPLICATIONS_FOLDER = "/Applications"


def _use_colors() -> bool:
    if os.environ.get("NO_COLOR") == "1" or os.environ.get("NO_ANSI") == "1":
        return False
    if hasattr(sys.stdout, "isatty") and not sys.stdout.isatty():
        return False
    return True


console = Console(force_terminal=_use_colors(), color_system="standard")
err_console = Console(stderr=True, force_terminal=_use_colors(), color_system="standard")


# Backwards-compatible shims for the original retro API still referenced
# from core/* (history.py, brew.py). The new rich/questionary UI is the
# primary surface, but these names are kept so core helpers don't break
# when a history file fails to load or a brew command raises.
RED = "bold red"


def retro_print(text: str, style: str = "", err: bool = True) -> None:
    """Minimal retro-style print used by core/* for warnings/errors."""
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
    """Tiny context manager that prints a start/finish line. Replaces the
    original progress indicator used by the brew install path.
    """

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

BRAND = "bold cyan"
GOOD = "bold green"
WARN = "bold yellow"
BAD = "bold red"
MUTED = "dim"


def _t(text: str) -> Text:
    return Text.from_markup(text)


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
    # Safe: no user input reaches this call.
    os.system("cls" if os.name == "nt" else "clear")


def title_block(version: str = "v0.0.3") -> Panel:
    body = Text.assemble(
        ("Homebrew App Migrator ", "bold white"),
        (version, "green"),
        ("\n", ""),
        ("Mac  ", "dim"),
        ("→ ", "dim"),
        ("Homebrew Cask ", "bold cyan"),
        ("× ", ""),
        ("Charm UI ", "bold magenta"),
    )
    return Panel(
        Align.center(body, vertical="middle"),
        title=_t("[bold cyan]BREW MIGRATOR[/bold cyan]"),
        subtitle=_t("[dim]charm.land-inspired terminal experience[/dim]"),
        border_style="cyan",
        box=HEAVY_EDGE,
        padding=(0, 3),
    )


def flow_badge(step: int) -> Panel:
    lines = [
        _build_step(0, step, "Scan", ":satellite: Reading /Applications"),
        _build_step(1, step, "Match", ":mag_right: Finding Cask candidates"),
        _build_step(2, step, "Rank", ":chart_increasing: Scoring + official-tap filter"),
        _build_step(3, step, "Decide", ":ticket: Confirm, review, or skip each app"),
        _build_step(4, step, "Finish", ":sparkles: Generate session summary"),
    ]
    body = "\n".join(lines)
    return Panel(
        Text.from_markup(body, style=BRAND),
        title=_t("[bold white]Progress[/bold white]"),
        border_style="blue",
        box=MINIMAL,
        padding=(0, 1),
    )


def _build_step(index: int, current: int, label: str, detail: str) -> str:
    if index < current:
        state = ":white_check_mark:"
        style = GOOD
    elif index == current:
        state = ":star2:"
        style = BRAND
    else:
        state = ":white_circle:"
        style = MUTED
    return f"{state} [{style}]{label}[/] [dim]{detail}[/dim]\n"


def welcome_panel(app_count: int, history_count: int, ignore_count: int) -> Panel:
    md = "\n".join(
        [
            "**Walkthrough**",
            "1. **Scan** — reads `" + _APPLICATIONS_FOLDER + "` and lists apps left to process",
            "2. **Match** — calls Homebrew, prefers `homebrew/cask` when available",
            "3. **Decide** — choose the best match, preview the command, and confirm destructive actions",
            f"4. **History** — remembers migrated/skipped/failed so reruns skip real apps automatically",
            "",
            f"**This run** — apps: {app_count} · entries: {history_count + ignore_count}",
        ]
    )
    return Panel(
        Markdown(md),
        title=_t("[bold white]Welcome, friend.[/bold white]"),
        subtitle=_t("[dim]You stayed on the safe path.[/dim]"),
        border_style="magenta",
        box=DOUBLE,
        padding=(1, 2),
    )


def animated_boot(seconds: float = 0.8) -> None:
    first = 0.4 * seconds
    second = 0.35 * seconds
    third = 0.1 * seconds
    last = 0.15 * seconds
    boot_spinner("Brew Migrator", duration=max(first, 0.3))

    with Progress(
        SpinnerColumn(),
        TextColumn("[bold yellow]{task.description}[/bold yellow]"),
        BarColumn(bar_width=None),
        console=console,
        transient=True,
    ) as progress:
        task_id = progress.add_task("Preparing workspace", total=1)
        time.sleep(first)
        try:
            progress.advance(task_id, 0.02)
        except TypeError:  # compitible fallback for older rich APIs
            progress.advance(task_id)
        time.sleep(second)
        try:
            progress.advance(task_id, 0.05)
        except TypeError:
            progress.advance(task_id)
        time.sleep(third)
        try:
            progress.advance(task_id, 0.93)
        except TypeError:
            progress.advance(task_id)
        time.sleep(last)


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
        box=HEAVY,
        padding=(0, 2),
    )


def confirmation_prompt(message: str, action: str = "Confirm", danger: bool = False) -> bool:
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
    return q.ask() or False


def list_apps(title: str, items: Sequence[str], count: Optional[int] = None) -> None:
    if count is None:
        count = len(items)
    table = Table(title=title, box=SIMPLE_HEAD)
    table.add_column("#", justify="right", style="dim", width=5)
    table.add_column("App", style="cyan", no_wrap=True)
    for idx, item in enumerate(items, 1):
        table.add_row(str(idx), item)
    caption = Text.from_markup(f":mag_right: Found [green]{count}[/green] apps found")
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
        desc = (desc[:55] + "…") if len(desc) > 55 else desc
        table.add_row(
            str(idx + 1),
            Text(match.get("name", ""), style="green" if official else "white"),
            str(match.get("score", "")),
            match.get("tap", "") or "—",
            match.get("version", "") or "—",
            Text("yes" if match.get("installed") else "no", style=GOOD if match.get("installed") else BAD),
            desc,
        )
    return Panel(table, title=title, border_style="green", box=HEAVY_EDGE)


def format_dry_run_candidates(
    app_candidates: Sequence[dict],
) -> List[dict]:
    """Mapcask candidate dicts from ``core.brew`` into the plan-table shape
    used by ``cli._build_planned_rows`` and ``cli.process_app``."""
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


def ask_match(title: str, matches: Sequence[dict], start_idx: int = 0) -> Tuple[str, Optional[str]]:
    action = "match"
    while True:
        selection = None
        clear_screen()
        console.print(title_block())
        console.print(flow_badge(2))
        console.print(match_table(title, matches, start_idx=start_idx))
        pager_choices: List[str] = []
        if start_idx > 0:
            pager_choices.append("⏮ Previous page")
        if start_idx + 6 < len(matches):
            pager_choices.append("Next page ⏭")
        pager_choices += ["View details", "Skip app", "Ignore app", "Quit"]

        if questionary is None:
            choice = input("Choice (1..n, p, n, s, i, q): ").strip().lower()
            if choice in ("q",):
                return "quit", None
            if choice in ("s",):
                return "skip", None
            if choice in ("i",):
                return "ignore", None
            if choice.isdigit():
                idx = int(choice) - 1
                if 0 <= idx < len(matches):
                    selection = matches[idx].get("name", "")
            if choice in ("p",):
                start_idx = max(0, start_idx - 6)
                continue
            if choice in ("n",):
                start_idx = min(len(matches) - 1, start_idx + 6)
                continue
        else:
            chosen = questionary.select(
                "Choose an action",
                choices=pager_choices,
                style=questionary.Style([("highlighted", "bold cyan")]),
                qmark="✨",
            ).ask()
            if chosen is None:
                return "quit", None
            if chosen.startswith("⏮"):
                start_idx = max(0, start_idx - 6)
                continue
            if chosen.startswith("Next"):
                start_idx = min(max(len(matches) - 6, 0), start_idx + 6)
                continue
            if chosen in {"Skip app"}:
                return "skip", None
            if chosen in {"Ignore app"}:
                return "ignore", None
            if chosen == "Quit":
                return "quit", None
            if chosen == "View details":
                selection = questionary.select(
                    "Pick a match to inspect",
                    choices=[m.get("name", "") for m in matches],
                    style=questionary.Style([("highlighted", "bold cyan")]),
                    qmark="🔽",
                ).ask()
            else:
                selection = chosen
        if selection:
            return action, selection


def display_dry_run_plan(title: str, planned_rows: Sequence[Tuple[str, dict]], export_path: Optional[str] = None) -> None:
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
        official = "yes" if row.get("tap", "").lower().startswith(("homebrew/cask/", "homebrew/cask-versions/", "homebrew/cask-fonts/")) else "no"
        table.add_row(
            app,
            row.get("cask", ""),
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
                line += (col[row_idx].ljust(col_min))
        console.print(line.rstrip(), style=color)


class AnimatedWait:
    def __init__(self, label: str = "Working") -> None:
        self.label = label
        self._progress = None

    def __enter__(self):
        self._progress = Progress(
            SpinnerColumn(),
            TextColumn("[bold yellow]{task.description}[/bold yellow]"),
            BarColumn(bar_width=None),
            console=console,
            transient=True,
        )
        self._progress.add_task(self.label, total=1)
        self._progress.start()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        if self._progress:
            self._progress.stop()
            if exc_type is None:
                console.print(_t(f":white_check_mark: {self.label} complete."), style=GOOD)
            else:
                console.print(_t(f":warning: {self.label} ended with an issue."), style=WARN)
        return False


import shutil
