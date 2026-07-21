from __future__ import annotations
import re
import subprocess
from typing import Dict, List, Optional, Tuple
from ..ui.console import retro_print, RED, ProgressBar


OFFICIAL_TAP_PREFIXES = (
    "homebrew/cask/",
    "homebrew/cask-versions/",
    "homebrew/cask-fonts/",
)


def _is_official_cask(name: str) -> bool:
    normalized = name.lower()
    if not normalized:
        return False
    if normalized.startswith(OFFICIAL_TAP_PREFIXES):
        return True
    if normalized.startswith("homebrew/"):
        return True
    if "/" not in normalized:
        return True
    return False


def _clean_brew_search_line(line: str) -> Optional[str]:
    line = line.strip()
    if not line or line.startswith("==>") or line.startswith("#"):
        return None
    return line


def score_candidate(app_name: str, candidate: str) -> int:
    base = app_name.lower().strip()
    candidate_lower = candidate.lower().strip()
    score = 0
    if candidate_lower == base:
        score += 50
    if candidate_lower.startswith(base + "/"):
        score += 30
    if candidate_lower.endswith("/" + base):
        score += 20
    tail = candidate_lower.split("/")[-1]
    if base == tail:
        score += 40
    elif base in tail:
        score += max(10, 20 - abs(len(base) - len(tail)))

    if _is_official_cask(candidate):
        score += 5

    score -= max(0, len(candidate_lower) - len(base)) // 4
    return score


def _run_brew(*args: str) -> Optional[str]:
    cmd = ["brew", *args]
    try:
        result = subprocess.run(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            check=False,
        )
        return result.stdout.decode("utf-8", errors="replace").strip()
    except Exception as e:
        retro_print(f"ERROR: {str(e)}", RED)
        return None


def _parse_info_into_map(info_text: str) -> Dict[str, Dict[str, str]]:
    packages: Dict[str, Dict[str, str]] = {}
    current: Optional[Dict[str, str]] = None
    current_name = ""
    for raw_line in info_text.splitlines():
        line = raw_line.rstrip("\n")
        if line.startswith("==> "):
            if current_name and current is not None:
                packages[current_name] = current
            current_name = line[5:].strip()
            current = {}
            continue
        if not current_name or current is None:
            continue
        if ":" in line:
            key, val = line.split(":", 1)
            current[key.strip()] = val.strip()
    if current_name and current is not None:
        packages[current_name] = current
    return packages


def get_cask_candidates(app_name: str) -> List[dict]:
    seen: List[str] = []
    seen_set = set()
    search_output = _run_brew("search", "--cask", app_name)
    if not search_output:
        return []

    collected: List[str] = []
    for line in search_output.splitlines():
        candidate = _clean_brew_search_line(line)
        if candidate is None or candidate in seen_set:
            continue
        seen_set.add(candidate)
        seen.append(candidate)

    official_first = [False]

    def sort_key(name: str):
        is_official = _is_official_cask(name)
        s = score_candidate(app_name, name)
        if official_first[0]:
            return (0 if is_official else 1, s)
        return (0, s)

    sorted_names = sorted(seen, key=sort_key, reverse=True)

    # Try to preserve official-first ordering while dedup-sorting by score
    official = [n for n in sorted_names if _is_official_cask(n)]
    unofficial = [n for n in sorted_names if not _is_official_cask(n)]
    official_first = official + unofficial
    official_first.sort(
        key=lambda n: score_candidate(app_name, n),
        reverse=True,
    )

    batches: List[List[str]] = [[]]
    for name in official_first:
        if len(batches[-1]) >= 8:
            batches.append([])
        batches[-1].append(name)

    merged: List[str] = []
    for idx, batch in enumerate(batches):
        b = sorted(batch, key=lambda n: score_candidate(app_name, n), reverse=True)
        if idx % 2 == 1:
            b = list(reversed(b))
        merged.extend(b)

    info_output = _run_brew("info", "--cask", *merged) if merged else ""
    info_map = _parse_info_into_map(info_output or "")

    output = []
    for name in merged:
        description = ""
        tap = ""
        version = ""
        installed = ""
        info = info_map.get(name, {})
        if info:
            description = info.get("Description", "")
            tap = info.get("Tap", "")
            version = info.get("Versions", "")
            if info.get("Installed"):
                installed = "yes"

        output.append(
            {
                "name": name,
                "description": description,
                "tap": tap,
                "version": version,
                "installed": installed,
                "score": score_candidate(app_name, name),
                "command": ["brew", "install", "--cask", name],
            }
        )

    output.sort(key=lambda item: item["score"], reverse=True)
    return output


def find_matches(app_name, search_type):
    if search_type != "cask":
        result = _run_brew("search", app_name or "")
        if not result:
            return []
        return [_clean_brew_search_line(line) for line in result.splitlines() if _clean_brew_search_line(line)]

    candidates = get_cask_candidates(app_name)
    return [c["name"] for c in candidates]


def is_already_installed(package_name, is_cask):
    cmd = [
        "brew",
        "list",
        "--cask" if is_cask else "--formula",
        package_name,
    ]
    try:
        result = subprocess.run(
            cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False
        )
        return result.returncode == 0
    except Exception:
        return False


def install_homebrew_package(
    package_name,
    is_cask,
    app_name,
    history_manager,
    dry_run=False,
    force=False,
):
    """Install a Homebrew package.

    - dry_run=True never mutates history and never runs brew install.
    - force=False by default; pass --force on the CLI to enable.
    """
    command = (
        ["brew", "install", "--cask", package_name] + (["--force"] if force else [])
        if is_cask
        else ["brew", "install", package_name, "--overwrite"]
    )

    if dry_run:
        return command, True

    try:
        with ProgressBar(f"INSTALLING {package_name}"):
            subprocess.run(
                command,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                check=True,
            )
        history_manager.update(app_name, "migrated", package_name)
        return command, True
    except subprocess.CalledProcessError as e:
        error_message = e.stderr.decode("utf-8").strip()
        history_manager.update(app_name, "failed", error_message)
        return command, False
    except Exception as e:
        error_message = str(e)
        history_manager.update(app_name, "failed", error_message)
        return command, False


def check_brew_installed():
    """Verify if Homebrew is installed."""
    try:
        return subprocess.run(["brew", "--version"], capture_output=True).returncode == 0
    except FileNotFoundError:
        return False
