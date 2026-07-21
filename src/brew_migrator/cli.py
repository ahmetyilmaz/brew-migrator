"""brew-migrator entrypoint — TUI-only flow.

No CLI args are required; the tool opens a charming terminal UI that
scans /Applications, matches Cask candidates, lets you review and
select, then installs.
"""
from __future__ import annotations

import os
import sys
import time
import json
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple

from rich.panel import Panel
from rich.progress import BarColumn, Progress, SpinnerColumn, TextColumn
from rich.table import Table
from rich.box import ROUNDED
from rich.live import Live

from .core.history import HistoryManager, HISTORY_PATH
from .core.brew import (
    get_cask_candidates,
    is_already_installed,
    install_homebrew_package,
    check_brew_installed,
)
from .ui.console import (
    BAD,
    BRAND,
    GOOD,
    PLAN_PATH,
    _APPLICATIONS_FOLDER,
    _candidate_css,
    _load_plan,
    _plan_value,
    _t,
    _use_colors,
    animated_boot,
    clear_screen,
    confirmation_prompt,
    console,
    display_dry_run_plan,
    display_retro_list,
    history_table,
    load_planned_rows,
    main_menu,
    match_table,
    merge_planned_rows,
    plan_table,
    preview_panel,
    questionary,
    summary_table,
    title_block,
    trust_banner,
    upsert_planned_rows,
    welcome_panel,
)

HISTORY = HistoryManager(HISTORY_PATH)


def list_app_names(folder: str = _APPLICATIONS_FOLDER) -> List[str]:
    if not os.path.isdir(folder):
        return []
    return sorted(
        os.path.splitext(app)[0]
        for app in os.listdir(folder)
        if app.endswith(".app")
    )


def match_app(app_name: str) -> List[dict]:
    return get_cask_candidates(app_name)


def _format_candidate(app_name: str, cand: dict) -> dict:
    """Normalise a candidate dict (from get_cask_candidates) into a plan row."""
    cmd = cand.get("command", [])
    if isinstance(cmd, list):
        cmd_str = " ".join(str(p) for p in cmd)
    else:
        cmd_str = str(cmd) if cmd else ""
    return {
        "app": app_name,
        "cask": cand.get("name", ""),
        "score": cand.get("score", 0),
        "tap": cand.get("tap", ""),
        "installed": cand.get("installed", ""),
        "version": cand.get("version", ""),
        "cmd": cmd_str,
    }


def _quality_label(score: int) -> str:
    """Rich-markup quality indicator based on match score."""
    if score > 90:
        return "[bold green]⭐ near perfect[/bold green]"
    elif score >= 50:
        return "[yellow]🤔 maybe[/yellow]"
    else:
        return "[red]❓ questionable[/red]"


def _quality_plain(score: int) -> str:
    """Plain-text quality label for questionary titles (no rich markup)."""
    if score > 90:
        return "⭐ near perfect"
    elif score >= 50:
        return "🤔 maybe"
    else:
        return "❓ questionable"


def interactive_scan(applications_folder: str = _APPLICATIONS_FOLDER) -> List[Tuple[str, Optional[dict]]]:
    """Live-scan apps with real-time table, then interactive checkbox selection.

    Shows a single Rich Live table that updates as each app is matched:
      score > 90  → ⭐ near perfect  (auto-selected, checkbox pre-checked)
      score 50–90 → 🤔 maybe         (checkbox unchecked)
      score < 50  → ❓ questionable   (checkbox unchecked)
      no match    → ⏭ skipped        (disabled in checkbox)

    After scanning, a questionary.checkbox() lets the user review and toggle
    selections with ↓↑ + space, then returns only the checked items.
    """
    app_names = list_app_names(applications_folder)
    if not app_names:
        console.print(trust_banner("No apps found. Override path from the main menu if needed.", kind="danger"))
        return []

    console.print(welcome_panel())
    console.print(_t(f"[bold green]{len(app_names)}[/bold green] applications found\n"))

    # Live-match state
    matched: Dict[str, Optional[dict]] = {}   # app → candidate (or None if skipped)
    quality: Dict[str, str] = {}              # app → rich-markup quality label
    score_val: Dict[str, int] = {}            # app → numeric score

    def _build_scan_table() -> Table:
        matched_count = sum(1 for v in matched.values() if v is not None)
        skipped_count = sum(1 for v in matched.values() if v is None)
        scanning_count = len(app_names) - len(matched)
        t = Table(
            title=_t("[bold]Matching Applications[/bold]"),
            box=ROUNDED,
            expand=True,
            caption=_t(f"[dim]{matched_count} matched  •  "
                       f"{skipped_count} skipped  •  "
                       f"{scanning_count} scanning[/dim]"),
        )
        t.add_column("#", style="dim", width=4)
        t.add_column("App", style="bold cyan", no_wrap=True)
        t.add_column("Cask", style="white")
        t.add_column("Score", justify="right", style="green", width=6)
        t.add_column("Quality", style="bold")
        t.add_column("Sel", justify="center", width=5)

        for i, app in enumerate(app_names, 1):
            if app in matched:
                cand = matched[app]
                if cand is None:
                    t.add_row(str(i), app, "[dim]—[/dim]", "[dim]—[/dim]", "[dim]⏭ skipped[/dim]", "[dim]—[/dim]")
                else:
                    t.add_row(
                        str(i), app,
                        cand.get("name", ""),
                        str(score_val.get(app, 0)),
                        quality.get(app, ""),
                        "[green]✓[/green]" if score_val.get(app, 0) > 90 else "○",
                    )
            else:
                t.add_row(str(i), app, "[dim]⏳[/dim]", "[dim]—[/dim]", "[dim]⏳[/dim]", "[dim]⏳[/dim]")
        return t

    # Phase 1 — live matching
    with Live(console=console, refresh_per_second=4, vertical_overflow="visible") as live:
        live.update(_build_scan_table())
        for app_name in app_names:
            candidates = match_app(app_name)
            if not candidates:
                matched[app_name] = None
                quality[app_name] = "[dim]⏭ skipped[/dim]"
                score_val[app_name] = 0
            else:
                best = candidates[0]
                matched[app_name] = best
                s = best.get("score", 0)
                score_val[app_name] = s
                quality[app_name] = _quality_label(s)
            live.update(_build_scan_table())
            time.sleep(0.02)

    console.print()  # breathing room after Live table

    # Phase 2 — interactive checkbox selection
    if questionary is None:
        # No TUI: return auto-selected (score > 90)
        return [
            (app, matched[app]) for app in app_names
            if app in matched and matched[app] is not None and score_val.get(app, 0) > 90
        ]

    choices: list = []
    for app in app_names:
        if app not in matched or matched[app] is None:
            choices.append(
                questionary.Choice(
                    title=f"{app}  [dim]⏭ no cask found[/dim]",
                    value=(app, matched.get(app)),
                    disabled=True,
                )
            )
            continue

        cand = matched[app]
        s = score_val.get(app, 0)
        checked = s > 90
        title = f"{app} → {cand.get('name', '')}  [{s} {_quality_plain(s)}]"
        choices.append(
            questionary.Choice(
                title=title,
                value=(app, cand),
                checked=checked,
            )
        )

    selected = questionary.checkbox(
        "Select apps to install:",
        choices=choices,
        qmark="✨",
        style=questionary.Style([("highlighted", "bold cyan")]),
        instruction="(↓↑ to move, space to toggle, enter to confirm)",
        validate=lambda a: len(a) > 0 or "Pick at least one app.",
    ).ask()

    if selected is None:
        # User cancelled — fall back to auto-selected
        return [
            (app, matched[app]) for app in app_names
            if app in matched and matched[app] is not None and score_val.get(app, 0) > 90
        ]
    return selected


def run_flow(folder: str = _APPLICATIONS_FOLDER) -> None:
    if not check_brew_installed():
        console.print(Panel("Homebrew is not installed or not on PATH.\nInstall from https://brew.sh and try again.", title="Homebrew required"))
        return

    folder = os.path.expanduser(folder)
    if not os.path.isdir(folder):
        console.print(Panel(f"Folder not found: {folder}\nUse another path from the main menu.", title="Folder missing"))
        return

    animated_boot(0.6)
    current_planned: List[Tuple[str, Optional[dict]]] = []
    while True:
        plan_rows = load_planned_rows()
        history_count = len(HISTORY.history)
        mode = main_menu(plan_size=len(plan_rows), history_size=history_count)

        if mode == "quit":
            console.print(Panel("See you soon!"))
            return
        if mode == "scan":
            current_planned = interactive_scan(folder)
            if current_planned:
                normed = [(app, _format_candidate(app, cand)) for app, cand in current_planned if cand and cand.get("name")]
                upsert_planned_rows(normed, PLAN_PATH)
                if confirmation_prompt("Install selected now?", action="Install", danger=False):
                    apply_selected(normed)
            continue
        if mode == "install":
            plan_rows = merge_planned_rows(load_planned_rows())
            if not plan_rows:
                console.print(Panel("Plan is empty. Scan & Plan first."))
                continue
            apply_selected(plan_rows)
            continue
        if mode == "history":
            show_history()
            continue
        if mode == "retry":
            plan_rows = merge_planned_rows(load_planned_rows())
            retry_plan = [
                (app, row)
                for app, row in plan_rows
                if isinstance(HISTORY.get(app), dict) and HISTORY.get(app).get("status") in {"failed", "skipped"}
            ]
            if not retry_plan:
                console.print(Panel("Nothing to retry."))
                continue
            apply_selected(retry_plan)
            continue


def apply_selected(planned: List[Tuple[str, Optional[dict]]]) -> bool:
    console.clear()
    console.print(Panel(_t(f"Selected installs: [green]{sum(1 for _, r in planned if r and r.get('cask'))}[/green]")))
    console.print(plan_table(planned))
    console.print()
    if not confirmation_prompt("Begin install?", action="Install", danger=False):
        console.print("Cancelled.")
        return False
    failures = []
    successes = []
    total = [p for p in planned if p[1] and p[1].get("cask")]
    if not total:
        console.print("Nothing to install.")
        return True
    with Progress(
        SpinnerColumn(),
        TextColumn("[bold yellow]{task.description}[/bold yellow]"),
        BarColumn(bar_width=None),
        TextColumn("[bold green]{task.completed}/{task.total}[/bold green]"),
        console=console,
        transient=False,
    ) as progress:
        task_id = progress.add_task("Installing cask", total=len(total))
        for app_name, row in total:
            cask = row.get("cask", "")
            if not cask:
                progress.advance(task_id)
                continue
            progress.update(task_id, description=f"Installing {app_name} → {cask}")
            selected_candidate = next((c for c in get_cask_candidates(app_name) if c.get("name") == cask), None)
            candidate = selected_candidate if selected_candidate else {
                "name": cask,
                "tap": row.get("tap", ""),
                "version": row.get("version", ""),
                "installed": row.get("installed", ""),
                "description": "",
                "command": ["brew", "install", "--cask", cask],
            }
            console.print(preview_panel(app_name, candidate))
            already = is_already_installed(cask, True)
            if already:
                HISTORY.update(app_name, "migrated", cask)
                console.print(_t(f":white_check_mark: Already installed: {cask}"), style=GOOD)
                successes.append(app_name)
                progress.advance(task_id)
                continue
            try:
                ok = install_flow(app_name, candidate, history_manager=HISTORY)
            except Exception as e:
                ok = False
            if ok:
                successes.append(app_name)
            else:
                failures.append(app_name)
            progress.advance(task_id)
    console.print()
    console.print(summary_table(successes, failures, [], []))
    return not failures


def install_flow(app_name: str, candidate: dict, *, history_manager: HistoryManager) -> bool:
    name = candidate.get("name", app_name)
    console.print(_t(f":hourglass:  Installing [bold]{name}[/bold]..."), style=BRAND)
    _, ok = install_homebrew_package(
        name,
        True,
        app_name,
        history_manager,
        dry_run=False,
        force=False,
    )
    status = _t(":white_check_mark: Installed.") if ok else _t(":x: Failed.")
    style = GOOD if ok else BAD
    console.print(status, style=style)
    return ok


def show_history(history: Dict[str, dict] = None) -> None:
    if history is None:
        history = HISTORY.history
    console.clear()
    console.print(title_block())
    choices = ["All", "Migrated", "Failed", "Skipped", "Ignored", "Back"]
    if questionary is None:
        console.print(history_table(history))
        return
    while True:
        console.print(history_table(history))
        view = questionary.select(
            "Filter history",
            choices=choices,
            qmark="🗂",
            style=questionary.Style([("highlighted", "bold cyan")]),
        ).ask()
        if view is None or view == "Back":
            break
        filtered = dict(history)
        if view != "All":
            filtered = {app: entry for app, entry in history.items() if entry.get("status") == view.lower()}
        console.clear()
        console.print(title_block())
        console.print(_t(f"Filter: [bold]{view}[/bold] — [green]{len(filtered)}[/green] entries"), style=BRAND)
        console.print(history_table(filtered))


def main() -> None:
    if not check_brew_installed():
        console.print(Panel(
            "Homebrew not found.\nInstall from https://brew.sh and restart this tool.",
            title="Homebrew required",
            style=BAD,
        ))
        return
    try:
        run_flow()
    except KeyboardInterrupt:
        console.print("\nBye!")
    except Exception as exc:
        console.print(Panel(f"{type(exc).__name__}: {exc}", title="Error", style=BAD))
        sys.exit(1)


if __name__ == "__main__":
    main()
