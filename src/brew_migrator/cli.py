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
    _plan_checkbox_label,
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


def interactive_scan(applications_folder: str = _APPLICATIONS_FOLDER) -> List[Tuple[str, Optional[dict]]]:
    app_names = list_app_names(applications_folder)
    if not app_names:
        console.print(trust_banner("No apps found. Override path from the main menu if needed.", kind="danger"))
        return []
    console.print(welcome_panel())
    console.print(f"Apps found: [bold green]{len(app_names)}[/bold green]\n")
    chosen: Dict[str, Optional[dict]] = {}
    with console.status("Scanning applications...", spinner="bouncingBall"):
        time.sleep(0.2)

    for app_name in app_names:
        candidates = match_app(app_name)
        selected: Optional[dict] = None
        if candidates:
            selected = candidates[0]
            if questionary is None:
                console.clear()
                console.print(title_block())
                console.print(Panel.from_markup(f"**{app_name}**\nFound {len(candidates)} candidate(s)."))
                console.print(match_table(app_name, candidates))
                console.print(preview_panel(app_name, selected))
                choice = input("Action: [1]install top  [2]pick another  [3]skip  [4]ignore  [q]uit ").strip().lower()
                if choice.startswith("2"):
                    idx = input("Pick candidate #: ").strip()
                    if idx.isdigit():
                        i = int(idx) - 1
                        if 0 <= i < len(candidates):
                            selected = candidates[i]
                elif choice.startswith("3"):
                    selected = None
                elif choice.startswith("4"):
                    if confirmation_prompt("Ignore this app permanently?", action="Ignore", danger=True):
                        HISTORY.update(app_name, "ignored", "user_ignored")
                    chosen[app_name] = None
                    continue
                elif choice.startswith("q"):
                    return [(app, chosen.get(app)) for app in chosen]
            else:
                actions = [
                    f"✅ Install top match ({selected.get('name', app_name)})",
                    "🔽 Pick another match",
                    "⏭ Skip app",
                    "🚫 Ignore app",
                    "🚪 Quit",
                ]
                console.clear()
                console.print(title_block())
                console.print(Panel.from_markup(f"**{app_name}**\nFound [green]{len(candidates)}[/green] candidate(s)."))
                console.print(match_table(app_name, candidates))
                console.print(preview_panel(app_name, selected))
                action = questionary.select(
                    "What to do?",
                    choices=actions,
                    qmark="🤖",
                    style=questionary.Style([("highlighted", "bold cyan")]),
                ).ask()
                if action is None:
                    return [(app, chosen.get(app)) for app in chosen]
                if "Pick another match" in action:
                    pick = questionary.select(
                        "Pick candidate:",
                        choices=[
                            questionary.Choice(
                                title=f"{c.get('name', '?')}  {_candidate_css(c)}  [{c.get('tap','')}]",
                                value=c,
                            )
                            for c in candidates
                        ],
                        qmark="🔽",
                    ).ask()
                    if pick:
                        selected = pick
                elif "Skip app" in action:
                    selected = None
                elif "Ignore app" in action:
                    if confirmation_prompt("Ignore this app permanently?", action="Ignore", danger=True):
                        HISTORY.update(app_name, "ignored", "user_ignored")
                    chosen[app_name] = None
                    continue
                elif "Quit" in action:
                    return [(app, chosen.get(app)) for app in chosen]
        else:
            console.clear()
            console.print(Panel.from_markup(f"**{app_name}**\nNo matches found."))
            if questionary is not None:
                action = questionary.select(
                    "No matches. What to do?",
                    choices=["Skip app", "Ignore app", "Quit"],
                    qmark="🤖",
                    style=questionary.Style([("highlighted", "bold cyan")]),
                ).ask()
                if action is None:
                    return [(app, chosen.get(app)) for app in chosen]
                if "Ignore app" in action:
                    if confirmation_prompt("Ignore this app permanently?", action="Ignore", danger=True):
                        HISTORY.update(app_name, "ignored", "user_ignored")
                    chosen[app_name] = None
                    continue
                if "Quit" in action:
                    return [(app, chosen.get(app)) for app in chosen]
            selected = None

        chosen[app_name] = selected
        time.sleep(0.05)
    return [(app, chosen.get(app)) for app in sorted(chosen.keys())]


def review_and_select(planned: List[Tuple[str, Optional[dict]]]) -> List[Tuple[str, Optional[dict]]]:
    if not planned:
        return planned
    console.clear()
    console.print(title_block())
    console.print(Panel("Review your selections.\nAutoselected top matches where available.\nDeselect anything you don't want installed."))
    if questionary is None:
        return planned
    checkbox_choices = [
        questionary.Choice(
            title=_plan_checkbox_label(app, row),
            value=(app, row),
            checked=row is not None and row.get("cask"),
        )
        for app, row in planned
    ]
    if not checkbox_choices:
        return planned
    selected = questionary.checkbox(
        "Selected installs",
        choices=checkbox_choices,
        qmark="✨",
        style=questionary.Style([("highlighted", "bold cyan")]),
        instruction="(space to toggle, enter to confirm)",
        validate=lambda a: len(a) > 0 or "Pick at least one item.",
    ).ask()
    if selected is None:
        return planned
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
            to_save = [(app, row) for app, row in current_planned if row and row.get("cask")]
            upsert_planned_rows(to_save, PLAN_PATH)
            current_planned = review_and_select(current_planned)
            to_save = current_planned
            upsert_planned_rows(to_save, PLAN_PATH)
            if confirmation_prompt("Install selected now?", action="Install", danger=False):
                apply_selected(current_planned)
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
    console.print(Panel.from_markup(f"Selected installs: [green]{sum(1 for _, r in planned if r and r.get('cask'))}[/green]"))
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
