import os
import sys
import time
import json
from datetime import datetime

import argparse

from .core.history import HistoryManager, HISTORY_PATH
from .core.brew import (
    find_matches,
    is_already_installed,
    install_homebrew_package,
    check_brew_installed,
    get_cask_candidates,
)
from .ui.console import (
    console,
    title_block,
    flow_badge,
    welcome_panel,
    trust_banner,
    preview_panel,
    confirmation_prompt,
    list_apps,
    ask_match,
    display_dry_run_plan,
    format_dry_run_candidates,
    display_retro_list,
    animated_boot,
    AnimatedWait,
    _use_colors,
    BRAND,
    GOOD,
    WARN,
    BAD,
    MUTED,
)

APPLICATIONS_FOLDER = "/Applications"
PLAN_PATH = os.path.join(os.path.dirname(HISTORY_PATH), "brew_migrator_plan.json")


def _build_planned_rows(apps: list[str]) -> list[tuple[str, dict]]:
    rows: list[tuple[str, dict]] = []
    for app_name in apps:
        app_candidates = get_cask_candidates(app_name)
        scored = format_dry_run_candidates(app_candidates)
        top = scored[0] if scored else {
            "cask": "(none)",
            "score": 0,
            "tap": "",
            "installed": "",
            "version": "",
            "cmd": "",
        }
        rows.append((app_name, top))
    return rows


def _update_plan(rows: list[tuple[str, dict]], plan_path: str = PLAN_PATH) -> str:
    existing: dict = {}
    if os.path.exists(plan_path):
        try:
            with open(plan_path, "r", encoding="utf-8") as f:
                existing = json.load(f) or {}
        except Exception:
            existing = {}

    existing_rows = {str(row.get("app", "")): row for row in existing.get("rows", []) if isinstance(row, dict)}
    merged: dict = dict(existing_rows)
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
    payload = {
        "generated_at": datetime.now().isoformat(),
        "applications_folder": APPLICATIONS_FOLDER,
        "rows": [merged[k] for k in sorted(merged.keys()) if merged[k].get("cask")],
    }
    os.makedirs(os.path.dirname(plan_path) or ".", exist_ok=True)
    with open(plan_path, "w", encoding="utf-8") as f:
        json.dump(payload, f, indent=2)
        f.write("\n")
    return plan_path


def run_plan(apps: list[str], plan_path: str = PLAN_PATH) -> str:
    rows = _build_planned_rows(apps)
    display_dry_run_plan("PLAN", rows, export_path=plan_path)
    _update_plan(rows, plan_path)
    print("Plan updated: {0}".format(plan_path))
    return plan_path


def _apply_plan(plan_json_path: str, history_manager: "HistoryManager", force: bool = False) -> bool:
    if not os.path.exists(plan_json_path):
        print("Plan file not found: {0}".format(plan_json_path))
        return False

    try:
        with open(plan_json_path, "r", encoding="utf-8") as f:
            plan = json.load(f)
    except Exception as e:
        print("Failed to read plan: {0}".format(e))
        return False

    rows = plan.get("rows", [])
    if not rows:
        print("Plan is empty.")
        return True

    filtered = []
    for row in rows:
        app = row.get("app")
        cask = row.get("cask")
        if not app or not cask or cask == "(none)":
            continue
        entry = history_manager.get(app)
        if entry:
            status = entry.get("status")
            if isinstance(status, str) and status.startswith("migrated"):
                continue
        filtered.append((app, cask))

    if not filtered:
        print("Nothing to apply from plan.")
        return True

    click = confirmation_prompt(
        "Apply plan with {0} install(s)?".format(len(filtered)),
        action="Apply plan",
        danger=False,
    )
    if not click:
        print("Plan apply cancelled.")
        return False

    failures = []
    for app_name, cask in filtered:
        success = False
        if is_already_installed(cask, True):
            print("Already installed: {0}".format(cask))
            history_manager.update(app_name, "migrated", cask)
            success = True
        else:
            _, success = install_homebrew_package(cask, True, app_name, history_manager, dry_run=False, force=force)
        if not success:
            failures.append(app_name)

    if failures:
        print("Applied with failures: {0}".format(", ".join(failures)))
    else:
        print("Plan applied successfully.")
    return True


def process_app(app_name, history_manager, batch_mode=False, dry_run=False, force=False):
    app_candidates = get_cask_candidates(app_name)

    if dry_run:
        planned_candidate = app_candidates[0] if app_candidates else None
        rows = format_dry_run_candidates(app_candidates[:5] if app_candidates else [])
        if rows:
            display_dry_run_plan("PLAN: {0}".format(app_name), [(app_name, rows[0])])
        else:
            display_dry_run_plan("PLAN: {0}".format(app_name), [(app_name, {
                "cask": "(none)",
                "score": 0,
                "tap": "",
                "installed": "",
                "version": "",
                "cmd": "",
            })])

        if planned_candidate and planned_candidate.get("installed") == "yes":
            print("Status: already installed, no action.")
        else:
            print("Status: would install best-matching cask.")
        return

    if batch_mode:
        with AnimatedWait("Batching"):
            time.sleep(0.2)
        selected_candidate = app_candidates[0]
        selected = selected_candidate.get("name")

        if is_already_installed(selected, True):
            print("Already installed: {0}".format(selected))
            history_manager.update(app_name, "migrated", selected)
            return

        click = confirmation_prompt(
            "Install {0}?".format(selected),
            action="Batch install",
            danger=force,
        )
        if not click:
            return

        cmd, success = install_homebrew_package(selected, True, app_name, history_manager, dry_run=False, force=force)
        if success:
            print("Installed: {0}".format(selected))
        else:
            print("Install failed: {0}".format(selected))
        return

    if not app_candidates:
        print("No matches for {0}. skipped.".format(app_name))
        history_manager.update(app_name, "skipped", "no_cask_found")
        return

    while True:
        mode, selected_name = ask_match("Select match", app_candidates)
        if mode == "quit":
            return selected_name, False
        if mode == "skip":
            print("Skipped.")
            history_manager.update(app_name, "skipped", "user_skipped")
            return selected_name, False
        if mode == "ignore":
            click = confirmation_prompt(
                "Ignore {0} permanently?".format(app_name),
                action="Ignore app",
                danger=True,
            )
            if click:
                history_manager.update(app_name, "ignored", "user_ignored")
                print("Ignored.")
            return selected_name, False
        if mode == "match" and selected_name:
            selected_candidate = app_candidates[[c.get("name") for c in app_candidates].index(selected_name)]
            selected_cask = selected_candidate.get("name", selected_name)
            click = confirmation_prompt(
                "Proceed with {0}?".format(selected_cask),
                action="Install",
                danger=force,
            )
            if not click:
                continue

            already = is_already_installed(selected_cask, True)
            if already:
                print("Already installed: {0}".format(selected_cask))
                history_manager.update(app_name, "migrated", selected_cask)
                return selected_name, True

            cmd, success = install_homebrew_package(selected_candidate.get("name", selected_candidate), True, app_name, history_manager, dry_run=dry_run, force=force)
            if success:
                history_manager.update(app_name, "migrated", selected_cask)
                print("Installed: {0}".format(selected_cask))
            else:
                print("Install failed: {0}".format(selected_cask))
            return selected_name, success


def main():
    parser = argparse.ArgumentParser(description="Migrate /Applications apps to Homebrew Casks.")
    parser.add_argument("--list-apps", action="store_true", help="List apps found in /Applications.")
    parser.add_argument("--app", type=str, help="Process a specific app by name.")
    parser.add_argument("--reset-history", action="store_true", help="Clear migration history.")
    parser.add_argument("--plan", action="store_true", default=False, help="Create or update plan without installing.")
    parser.add_argument("--apply-plan", type=str, nargs="?", const=PLAN_PATH, default=None, help="Install from saved plan. Defaults to the single plan file for this PC when this flag is provided.")
    parser.add_argument("--interactive", action="store_true", help="Direct interactive install mode.")
    parser.add_argument("--batch", action="store_true", help="Batch mode: install top match automatically.")
    parser.add_argument("--retry-skipped", action="store_true", help="Retry previously skipped apps.")
    parser.add_argument("--retry-ignored", action="store_true", help="Retry previously ignored apps.")
    parser.add_argument("--retry-failed", action="store_true", help="Retry previously failed apps.")
    parser.add_argument("--list-skipped", action="store_true", help="List skipped apps.")
    parser.add_argument("--list-ignored", action="store_true", help="List ignored apps.")
    parser.add_argument("--list-installed", action="store_true", help="List migrated apps.")
    parser.add_argument("--force", action="store_true", help="Allow brew install with --force. Enable explicitly.")
    parser.add_argument("--applications-folder", type=str, default=APPLICATIONS_FOLDER, help="Override /Applications path.")
    args = parser.parse_args()

    listing_requested = any([
        args.list_apps,
        args.list_skipped,
        args.list_ignored,
        getattr(args, "list_installed", False),
    ])

    plain = False
    if hasattr(sys.stdout, "isatty") and not sys.stdout.isatty():
        plain = True
    if os.environ.get("NO_ANSI") == "1" or os.environ.get("NO_COLOR") == "1":
        plain = True

    if not plain and not listing_requested:
        animated_boot(0.8)

    if not check_brew_installed():
        print("ERROR: HOMEBREW NOT FOUND. ABORT MISSION.")
        return

    applications_folder = args.applications_folder
    if not os.path.exists(applications_folder):
        print("ERROR: {0} NOT FOUND. Use --applications-folder to point elsewhere.".format(applications_folder))
        return
    if not plain:
        print(applications_folder)

    history_manager = HistoryManager(HISTORY_PATH)

    if args.reset_history:
        if history_manager.clear():
            print("History file '{0}' cleared.".format(HISTORY_PATH))
        else:
            print("No history file found to clear.")
        return

    app_names = sorted([
        os.path.splitext(app)[0]
        for app in os.listdir(applications_folder)
        if app.endswith(".app")
    ])

    if args.list_apps:
        list_apps("Applications in {0}".format(applications_folder), app_names)
        return

    if args.list_skipped:
        display_retro_list("Skipped apps", history_manager.get_skipped(), WARN)
        return

    if args.list_ignored:
        display_retro_list("Ignored apps", history_manager.get_ignored(), BAD)
        return

    if args.list_installed:
        display_retro_list("Migrated apps", history_manager.get_installed(), GOOD)
        return

    apps_to_process = []
    initial_state_history = history_manager.copy_history()

    for app_name in app_names:
        if args.app and app_name != args.app:
            continue

        entry = history_manager.get(app_name)
        if entry:
            status = entry.get("status")
            if isinstance(status, str) and status.startswith("migrated"):
                continue
            if status == "skipped" and not args.retry_skipped:
                continue
            if status == "ignored" and not args.retry_ignored:
                continue
            if status == "failed" and not args.retry_failed:
                continue
        apps_to_process.append(app_name)

    if args.apply_plan:
        _apply_plan(args.apply_plan, history_manager, force=args.force)
        print("History file updated. Goodbye.")
        return

    interactive_requested = args.interactive

    if not plain and not args.apply_plan:
        console.print(title_block())
        console.print(welcome_panel(len(app_names), len(history_manager.history), len(history_manager.get_ignored())))
        console.print(flow_badge(3))

    if args.plan or (not interactive_requested and not any([
        args.batch,
        args.retry_skipped,
        args.retry_ignored,
        args.retry_failed,
    ])):
        run_plan(apps_to_process, PLAN_PATH)
        print("History unchanged.")
        print("To install, run: brew-migrator --apply-plan")
        return

    if not apps_to_process:
        print("Nothing to do. All apps already have history.")
        return

    try:
        for app_name in apps_to_process:
            try:
                process_app(app_name, history_manager, batch_mode=args.batch, dry_run=False, force=args.force)
            except KeyboardInterrupt:
                raise
    except KeyboardInterrupt:
        print("\n\n!!! INTERRUPT SIGNAL RECEIVED !!!")
        print("SAVING PROGRESS BEFORE EXIT...")
    finally:
        print()
        newly_migrated, failed, skipped, ignored = history_manager.get_summary(initial_state_history)

        if newly_migrated:
            display_retro_list("Newly migrated apps", [
                "{0} -> {1}".format(app, detail)
                for app, detail in ((entry.split(" -> ")[0], entry.split(" -> ", 1)[1]) for entry in newly_migrated)
            ], GOOD)
        if failed:
            display_retro_list("Failed apps", failed, BAD)
        if skipped:
            display_retro_list("Skipped apps", skipped, WARN)
        if ignored:
            display_retro_list("Newly ignored apps", ignored, BAD)

        print("History file updated. Goodbye.")


if __name__ == "__main__":
    main()
