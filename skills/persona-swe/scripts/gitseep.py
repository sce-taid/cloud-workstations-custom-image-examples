#!/usr/bin/env python3

# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""GitSeep: Architectural Bedrock Reconstruction Tool.

Technical Metaphor:
- Surface (HEAD): The current state of your working directory/index.
- Strata (Commits): The layers of history between a base commit and HEAD.
- Bedrock: The specific historical commit that "owns" a path (defined in rules).
- Seepage: Moving changes from the surface down through layers until they
           are permanently deposited in their respective bedrock commits.
- Sedimentation: Syncing perfectly formed Bedrock commits out to isolated feature branches.

This tool automates the process of "backfilling" a clean architectural state
into an unstructured evolution by re-applying the final state of specific paths
to their original introductory commits. By adding Sedimentation, developers can
work exclusively on a single branch and let GitSeep autonomously manage and
sync all associated feature PR branches.
"""

# go/keep-sorted start
import argparse
import contextlib
import logging
import os
import re
import shutil
import subprocess
import sys
import termios
import tty
import uuid
from dataclasses import dataclass, field
from enum import IntEnum
from pathlib import Path
from typing import Dict, Generator, List, Optional, Set, Tuple

import yaml
# go/keep-sorted end

# --- Exit Codes ---


class ExitCode(IntEnum):
    """Standardized exit codes following common conventions."""

    OK = 0  # Success
    ERROR = 1  # General error
    USAGE = 2  # Command line usage error
    POLICY = 3  # Policy violation (e.g., --no-lithify)
    DATAERR = 65  # Data format error (e.g., YAML)
    NOINPUT = 66  # Missing input file
    UNAVAILABLE = 69  # Service unavailable / Dirty workdir
    CONFIG = 78  # Configuration error
    SIGINT = 130  # Script interrupted (Ctrl+C)


# --- Exceptions ---


class Error(Exception):
    """Base class for exceptions in this module."""


class GitError(Error):
    """Raised when a git command fails."""


class DirtyWorktreeError(GitError):
    """Raised when the working directory is not clean."""


class ConfigurationError(Error):
    """Raised when the seepage rules are invalid or missing."""


@contextlib.contextmanager
def git_work_session(original_branch: str) -> Generator[None, None, None]:
    """Ensures the repository returns to a safe state after operations.

    Args:
        original_branch: The branch name to return to on exit or error.
    """
    try:
        yield
    except KeyboardInterrupt:
        log("\nAborted by user.")
        run_git(["reset", "--hard", "HEAD"], check=False)
        run_git(["checkout", "-f", original_branch], check=False)
        sys.exit(ExitCode.SIGINT)
    except Error as e:
        log(str(e), level="error")
        run_git(["reset", "--hard", "HEAD"], check=False)
        run_git(["checkout", "-f", original_branch], check=False)
        sys.exit(ExitCode.ERROR)
    except Exception as e:
        log(f"Unexpected fatal error: {e}", level="error")
        run_git(["reset", "--hard", "HEAD"], check=False)
        run_git(["checkout", "-f", original_branch], check=False)
        sys.exit(ExitCode.ERROR)


# --- Constants & Style ---

class UI:
    """Centralized UI constants and styling."""
    DELIM = "|DELIM|"
    RESET = "\033[0m"
    BOLD = "\033[1m"
    DIM = "\033[2m"
    # Colors
    BLACK = "\033[30m"
    RED = "\033[31m"
    GREEN = "\033[32m"
    GREY = "\033[90m"
    HIGHLIGHT = "\033[7m"

    # Emojis/Icons
    ICON_BEDROCK = "🪨"
    ICON_PERCOLATE = "💧"
    ICON_SEEP = "🫧"
    ICON_LITHIFY = "💎"
    ICON_BRANCH = "🏞️"
    ICON_SUCCESS = "🏆"
    ICON_WARN = "⚠️"
    ICON_SELECTED = "✅"
    ICON_EXCLUDED = "❌"


class GitSeepFormatter(logging.Formatter):
    """Custom logging formatter for the GitSeep UI."""

    FORMATS = {
        logging.DEBUG: f"{UI.GREY}debug: %(message)s{UI.RESET}",
        logging.INFO: "%(message)s",
        logging.WARNING: f"{UI.ICON_WARN} {UI.BOLD}warning:{UI.RESET} %(message)s",
        logging.ERROR: f"{UI.RED}error: %(message)s{UI.RESET}",
        # Custom level for achievements
        25: f"{UI.ICON_SUCCESS} %(message)s"
    }

    def format(self, record):
        log_fmt = self.FORMATS.get(record.levelno, self.FORMATS[logging.INFO])
        formatter = logging.Formatter(log_fmt)
        return formatter.format(record)


# Configure global logger
logger = logging.getLogger("gitseep")
logger.setLevel(logging.INFO)
handler = logging.StreamHandler(sys.stdout)
handler.setFormatter(GitSeepFormatter())
logger.addHandler(handler)


def log(msg: str = "", level: str = "info"):
    """Standardized logger for the GitSeep CLI."""
    if level == "error":
        logger.error(msg)
    elif level == "warn":
        logger.warning(msg)
    elif level == "success":
        logger.log(25, msg)
    else:
        logger.info(msg)


RULES_FILENAME = ".gitseep.yaml"


# --- Data Classes ---


@dataclass
class SeepageContext:
    """Carries the state and configuration for the seepage session."""

    repo_root: Path
    original_head: str
    current_branch: str
    target_branch: str
    parent_of_strata: str
    strata: List[str]
    resolved_rules: Dict[str, List[str]]
    path_to_bedrock: Dict[str, str]
    date_to_branch: Dict[str, str]
    all_files: bool
    auto_approve: bool
    no_lithify: bool
    stage_only: bool
    dry_run: bool


@dataclass
class SeepageSummary:
    """Collects statistics and results for the final report."""

    strata_processed: int = 0
    bedrock_files: int = 0
    percolate_files: int = 0
    seep_files: int = 0
    lithified_files: Dict[str, List[str]] = field(default_factory=dict)
    percolate_paths: Set[str] = field(default_factory=set)
    seep_paths: Set[str] = field(default_factory=set)
    sedimented_branches: Set[str] = field(default_factory=set)
    parity_passed: bool = False


# --- Git Helpers ---


def run_git(args: List[str], check: bool = True, env_vars: Optional[Dict[str, str]] = None) -> subprocess.CompletedProcess:
    """Executes a git command and returns the result.

    Args:
        args: List of git command arguments.
        check: Whether to raise an exception on non-zero exit.
        env_vars: Optional dictionary of environment variables to inject.

    Returns:
        The subprocess.CompletedProcess object.

    Raises:
        GitError: If the command fails and check is True.
    """
    env = os.environ.copy()
    if env_vars:
        env.update(env_vars)

    try:
        return subprocess.run(
            ["git"] + args,
            capture_output=True,
            text=True,
            check=check,
            env=env,
        )
    except subprocess.CalledProcessError as e:
        error_msg = f"Error executing git {' '.join(args)}: {e.stderr.strip()}"
        raise GitError(error_msg) from e


def get_git_root() -> Path:
    """Returns the absolute path to the git repository root."""
    try:
        return Path(
            run_git(["rev-parse", "--show-toplevel"]).stdout.strip()
        ).resolve()
    except GitError:
        log("Fatal: Not a git repository.", level="error")
        sys.exit(ExitCode.ERROR)


def get_single_key() -> str:
    """Reads a single keypress from the terminal without waiting for Enter.

    Returns:
        The string representation of the key pressed.
    """
    fd = sys.stdin.fileno()
    old_settings = termios.tcgetattr(fd)
    try:
        tty.setraw(sys.stdin.fileno())
        ch = sys.stdin.read(1)
        if ch == "\x1b":  # Escape sequence (e.g., arrow keys, page up/down)
            ch += sys.stdin.read(2)
            # Handle vt100 extended keys that end with ~
            if ch[-1] in ["1", "2", "3", "4", "5", "6"]:
                ch += sys.stdin.read(1)
    finally:
        termios.tcsetattr(fd, termios.TCSADRAIN, old_settings)
    return ch.lower()


# --- UI Helpers ---


def color_hash(h: str) -> str:
    """Returns a short, bolded, deterministic color-coded version of a git hash."""
    # Deterministic color based on the hash string
    colors = [31, 32, 33, 34, 35, 36]  # Red, Green, Yellow, Blue, Magenta, Cyan
    try:
        # Convert first 7 chars of hex hash to int and modulo the colors list
        color_code = colors[int(h[:7], 16) % len(colors)]
    except ValueError:
        # Fallback for unit tests that pass non-hex strings like "hash" or "ORIGINAL_HEAD"
        color_code = 32 # Green
    return f"{UI.BOLD}\033[{color_code}m{h[:7]}{UI.RESET}"


def strip_ansi(text: str) -> str:
    """Removes ANSI escape codes from a string."""
    ansi_escape = re.compile(r"\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])")
    return ansi_escape.sub("", text)


def _print_preflight_briefing(ctx: SeepageContext):
    """Prints a summary of the strata range and target branch before starting.

    Args:
        ctx: The active SeepageContext.
    """
    log("\n--- Pre-flight seepage briefing ---")
    log(f"Target branch: {ctx.target_branch}")
    log(f"Surface hash:  {color_hash(ctx.original_head)}")
    log("Architectural bedrock assignments are listed under their commits.")
    log("Press [Q] at any prompt to abort without making changes.")
    log("-" * 75)
    hdr = (
        f"{'Stratum':<7} {'[Commit]':<10} {'(Author Date)':<27} - {'Message'}"
    )
    log(hdr)
    log("-" * 75)

    for i, h in enumerate(reversed(ctx.strata)):
        log_out = run_git(["log", "-1", "--format=%ai|%s", h]).stdout.strip()
        commit_info = log_out.split("|", 1)
        date = commit_info[0]
        msg = commit_info[1]

        layer_num = len(ctx.strata) - i
        layer_str = f"{layer_num}/{len(ctx.strata)}"
        h_color = color_hash(h)
        log(f"{layer_str:<7} {h_color:<19} ({date}) - {msg}")
        if h in ctx.resolved_rules:
            for owned_path in sorted(ctx.resolved_rules[h]):
                log(f"            ⮑  {owned_path}")

    log("-" * 75)


class HistorySelector:
    """Manages the interactive selection UI for Phase 2."""

    def __init__(
        self,
        ctx: SeepageContext,
        proposed_schedule: Dict[str, Dict[str, List[str]]],
        sources: Dict[str, Set[str]]
    ):
        self.ctx = ctx
        self.proposed_schedule = proposed_schedule
        self.sources = sources
        self.items = self._flatten_schedule()
        self.excluded_paths: Set[str] = set()
        self.cursor_idx = 0
        self.scroll_offset = 0
        self.page_size = max(1, (shutil.get_terminal_size().lines - 12) // 2)

    def _flatten_schedule(self) -> List[Tuple[str, str, str]]:
        """Flattens the schedule into a list of (path, src_h, bedrock_h) for UI display."""
        flat_list = []
        for bedrock_h, sources_map in self.proposed_schedule.items():
            for src_h, files in sources_map.items():
                for f in sorted(files):
                    flat_list.append((f, src_h, bedrock_h))
        # Sort by path for predictable UI
        return sorted(flat_list, key=lambda x: x[0])

    def run(self) -> Dict[str, Dict[str, List[str]]]:
        """Runs the interactive selection loop. Returns the final schedule."""
        if self.ctx.auto_approve or not self.items:
            return self.proposed_schedule

        while True:
            self._draw_ui()
            key = get_single_key()

            if key in ["\x1b[a", "k"]:  # Up
                self.cursor_idx = max(0, self.cursor_idx - 1)
            elif key in ["\x1b[b", "j"]:  # Down
                self.cursor_idx = min(len(self.items) - 1, self.cursor_idx + 1)
            elif key in ["\x1b[5~", "\x1b[v"]:  # Page Up
                self.cursor_idx = max(0, self.cursor_idx - self.page_size)
            elif key in ["\x1b[6~", "\x1b[u"]:  # Page Down
                self.cursor_idx = min(len(self.items) - 1, self.cursor_idx + self.page_size)
            elif key in ["\x1b[1~", "\x1b[h"]:  # Home
                self.cursor_idx = 0
            elif key in ["\x1b[4~", "\x1b[f"]:  # End
                self.cursor_idx = len(self.items) - 1
            elif key in [" ", "\r", "x"]:  # Toggle
                path = self.items[self.cursor_idx][0]
                if path in self.excluded_paths:
                    self.excluded_paths.remove(path)
                else:
                    self.excluded_paths.add(path)
            elif key == "v" or key == "d":  # Diff
                path, src, bedrock = self.items[self.cursor_idx]
                self._show_diff(path, src, bedrock)
            elif key == "c":  # Continue
                break
            elif key == "q":  # Abort
                log("\nAborted.")
                sys.exit(ExitCode.OK)

        return self._build_final_schedule()

    def _draw_ui(self):
        """Renders the selection list to the terminal."""
        # Clear screen and move cursor to top-left to redraw UI
        sys.stdout.write("\033[2J\033[H")

        # Dynamically recalculate page size
        self.page_size = max(1, (shutil.get_terminal_size().lines - 12) // 2)

        # Handle scrolling
        if self.cursor_idx < self.scroll_offset:
            self.scroll_offset = self.cursor_idx
        elif self.cursor_idx >= self.scroll_offset + self.page_size:
            self.scroll_offset = self.cursor_idx - self.page_size + 1

        print("\n--- Phase 2: Selective exclusion ---")
        print("Use [UP/DOWN] to navigate, [SPACE/ENTER/X] to toggle selection.")
        print(f"Legend: [{UI.ICON_SELECTED}] = Selected for migration, [{UI.ICON_EXCLUDED}] = Excluded from migration.")
        print("Use [V/D] to view migration diffs.")
        print("Press [C] to finalize selection and continue.")
        print("Press [Q] or [Ctrl+C] to abort any time without action.")
        print("-" * 80)

        for i in range(self.scroll_offset, min(len(self.items), self.scroll_offset + self.page_size)):
            path, src_h, bedrock_h = self.items[i]

            # Icon
            status_icon = UI.ICON_SELECTED if path not in self.excluded_paths else UI.ICON_EXCLUDED

            # Lithification label
            lith_label = ""
            if len(self.sources[path]) > 1:
                lith_label = f" {UI.GREY}{UI.ICON_LITHIFY} Lithify{UI.RESET}"

            # Highlighting
            prefix = f"{UI.HIGHLIGHT}>{UI.RESET} " if i == self.cursor_idx else "  "

            # Directional label
            src_idx = self.ctx.strata.index(src_h)
            bed_idx = self.ctx.strata.index(bedrock_h)
            if src_idx < bed_idx:
                dir_str = "↑ Seep Up"
                dir_icon = UI.ICON_SEEP
            else:
                dir_str = "↓ Percolate Down"
                dir_icon = UI.ICON_PERCOLATE

            # Filename in black as per mandate
            print(f"{prefix}[{status_icon}] {UI.BLACK}{path}{UI.RESET}{lith_label}")
            print(f"    ⮑  {dir_icon} {dir_str} to Bedrock {color_hash(bedrock_h)} from stratum commit: {color_hash(src_h)}")
        print("-" * 80)
        selected_count = len(self.items) - len(self.excluded_paths)
        print(f"Item {self.cursor_idx + 1} of {len(self.items)} ({selected_count}/{len(self.items)} selected for migration)\n")
        sys.stdout.flush()
    def _show_diff(self, path: str, src_h: str, bedrock_h: str):
        """Displays a diff between the current path and its proposed bedrock state."""
        log(f"\n--- Diff for {path} ---")
        log(f"From: {color_hash(src_h)} (stratum source)")
        log(f"To:   {color_hash(bedrock_h)} (proposed bedrock target)")

        try:
            # We diff the file as it exists in the 'src' commit vs the 'original_head'
            # (which represents the final state we want to seep down).
            diff_res = run_git(["diff", f"{src_h}:{path}", f"{self.ctx.original_head}:{path}"])
            if not diff_res.stdout.strip():
                log("No changes (path state matches bedrock target).", level="info")
            else:
                log(diff_res.stdout)
        except GitError:
            log("Could not generate diff (path might not exist in source commit).", level="warn")

        log("\nPress any key to return to selection UI...")
        get_single_key()

    def _build_final_schedule(self) -> Dict[str, Dict[str, List[str]]]:
        """Constructs a new schedule excluding the user-selected items."""
        new_schedule: Dict[str, Dict[str, List[str]]] = {}
        for bedrock_h, sources_map in self.proposed_schedule.items():
            new_sources: Dict[str, List[str]] = {}
            for src_h, files in sources_map.items():
                filtered_files = [f for f in files if f not in self.excluded_paths]
                if filtered_files:
                    new_sources[src_h] = filtered_files
            if new_sources:
                new_schedule[bedrock_h] = new_sources
        return new_schedule


# --- Seepage Logic ---


def resolve_commit_by_date(date_str: str) -> str:
    """Finds a unique commit hash matching an author date string.

    Args:
        date_str: An author date string (e.g., from 'git log --format=%ai').

    Returns:
        The full 40-character commit hash.

    Raises:
        GitError: If no commit is found.
    """
    try:
        res = run_git(["log", "--format=%H %ai"])
        for line in res.stdout.strip().splitlines():
            if line.endswith(date_str) or date_str in line:
                return line.split(" ")[0]

        # Fallback for YYYY-MM-DD format (first commit on or after day)
        if len(date_str) == 10:
            res = run_git(["log", "--after", f"{date_str} 00:00:00", "--reverse", "--format=%H %ai"])
            lines = res.stdout.strip().splitlines()
            if lines:
                return lines[0].split()[0]

        raise GitError(f"No commits found for date {date_str} in current branch history.")
    except Exception as e:
        raise GitError(f"Failed to resolve commit for date {date_str}: {e}")

def force_path_state(path: Path | str, commit: str):
    """Hard-resets a path to its exact state in a specific commit.

    Args:
        path: The file or directory path.
        commit: The commit hash to restore from.
    """
    p = Path(path)
    if p.exists():
        if p.is_dir():
            shutil.rmtree(p)
        else:
            p.unlink()

    # If the path exists in the target commit, check it out
    ls = run_git(["ls-tree", "-d", commit, "--", str(p)], check=False)
    if ls.stdout.strip() or run_git(["ls-tree", commit, "--", str(p)], check=False).stdout.strip():
        run_git(["checkout", commit, "--", str(p)])


def _initialize_seepage_context(
    rules_dict: Dict[str, List[str]],
    target_branch: str,
    *,  # Enforce keyword-only arguments for all following flags
    all_files: bool,
    auto_approve: bool,
    no_lithify: bool,
    stage_only: bool,
    dry_run: bool,
    base_commit: Optional[str],
) -> SeepageContext:
    """Validates the repository state and prepares the seepage context.

    Args:
        rules_dict: Mapping of date strings to lists of paths.
        target_branch: The branch to update with refactored history.
        all_files: Whether to list all files in bedrock commits.
        auto_approve: Whether to skip non-critical user confirmations.
        no_lithify: Whether to prevent multi-stratum squashing and error instead.
        stage_only: Whether to exit after preparing the temp branch.
        dry_run: Whether to simulate without modifying history.
        base_commit: Optional starting commit for the refactor range.

    Returns:
        A populated SeepageContext object.
    """
    repo_root = get_git_root()
    os.chdir(repo_root)

    status = run_git(["status", "--porcelain"])
    if status.stdout.strip():
        raise DirtyWorktreeError("Working directory is not clean.")

    original_head = run_git(["rev-parse", "HEAD"]).stdout.strip()
    current_branch = (
        run_git(["rev-parse", "--abbrev-ref", "HEAD"]).stdout.strip()
    )

    # 1. Resolve Bedrocks
    resolved_rules: Dict[str, List[str]] = {}
    path_to_bedrock: Dict[str, str] = {}
    date_to_branch: Dict[str, str] = {}

    for key, rule_val in rules_dict.items():
        if isinstance(rule_val, dict):
            paths = rule_val.get("paths", [])
            branch = rule_val.get("branch")
        else:
            paths = rule_val
            branch = None

        # Key can be a date or a hash or a branch
        # If it's YYYY-MM-DD, resolve it
        if re.match(r"\d{4}-\d{2}-\d{2}", key):
            h = resolve_commit_by_date(key)
            date_to_branch[key] = branch if branch else f"sediment/{key}" # Default Sedimentation branch
        else:
            h = run_git(["rev-parse", key]).stdout.strip()

        resolved_rules[h] = paths
        for p in paths:
            path_to_bedrock[p] = h

    # 2. Identify the full range of strata (from parent of oldest bedrock to HEAD)
    # Sort bedrocks by commit time
    sorted_hashes = sorted(
        resolved_rules.keys(),
        key=lambda x: int(
            run_git(["log", "-1", "--format=%at", x]).stdout.strip()
        ),
    )

    oldest_bedrock = sorted_hashes[0]

    # If a base commit was provided, use its parent as the anchor
    # Otherwise use the parent of the oldest bedrock
    start_point = base_commit if base_commit else oldest_bedrock
    parent_of_strata = run_git(["rev-parse", f"{start_point}^"]).stdout.strip()

    # Get the linear history from parent to original_head
    strata_res = run_git(
        ["log", "--format=%H", "--reverse", f"{parent_of_strata}..{original_head}"]
    )
    strata = strata_res.stdout.strip().splitlines()

    return SeepageContext(
        repo_root=repo_root,
        original_head=original_head,
        current_branch=current_branch,
        target_branch=target_branch,
        parent_of_strata=parent_of_strata,
        strata=strata,
        resolved_rules=resolved_rules,
        path_to_bedrock=path_to_bedrock,
        date_to_branch=date_to_branch,
        all_files=all_files,
        auto_approve=auto_approve,
        no_lithify=no_lithify,
        stage_only=stage_only,
        dry_run=dry_run,
    )


def _calculate_percolation_schedule(
    ctx: SeepageContext,
) -> Tuple[Dict[str, Dict[str, List[str]]], Dict[str, Set[str]]]:
    """Determines which files from which strata should be moved to which bedrock.

    Returns:
        A tuple of (schedule, sources).
        - schedule: {bedrock_h: {source_h: [files]}}
        - sources: {file_path: {set of source_hashes where it changed}}
    """
    # schedule[bedrock_hash][source_hash] = [files]
    schedule: Dict[str, Dict[str, List[str]]] = {}
    # sources[file_path] = {source_hashes}
    sources: Dict[str, Set[str]] = {}

    # Optimized O(1) discovery using batched log
    log_res = run_git(
        [
            "log",
            f"--format=[%H]",
            "--name-only",
            f"{ctx.parent_of_strata}..{ctx.original_head}",
        ]
    )

    current_hash = None
    for line in log_res.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        if line.startswith("[") and line.endswith("]"):
            current_hash = line[1:-1]
            continue

        file_path = line
        # Does this file belong to a bedrock?
        # Use "Most Specific Path Wins" (longest prefix)
        best_match = None
        for rule_path in ctx.path_to_bedrock:
            if file_path == rule_path or file_path.startswith(f"{rule_path.rstrip('/')}/"):
                if best_match is None or len(rule_path) > len(best_match):
                    best_match = rule_path

        if best_match:
            bedrock_h = ctx.path_to_bedrock[best_match]
            if bedrock_h not in schedule:
                schedule[bedrock_h] = {}
            if current_hash not in schedule[bedrock_h]:
                schedule[bedrock_h][current_hash] = []

            schedule[bedrock_h][current_hash].append(file_path)

            if file_path not in sources:
                sources[file_path] = set()
            sources[file_path].add(current_hash)

    return schedule, sources


def _perform_read_only_review(
    ctx: SeepageContext,
    schedule: Dict[str, Dict[str, List[str]]],
    sources: Dict[str, Set[str]],
):
    """Displays a detailed review of the proposed percolation actions."""
    log("\n--- Phase 1: Stratigraphy review (read-only) ---")

    for i, commit_hash in enumerate(reversed(ctx.strata)):
        actual_idx = len(ctx.strata) - 1 - i
        log_out = run_git(["log", "-1", "--format=%s", commit_hash]).stdout.strip()
        layer_str = f"{actual_idx + 1}/{len(ctx.strata)}"
        log(f"\n{layer_str} {color_hash(commit_hash)} - {log_out}")

        # What's happening in this commit?
        actions: List[str] = []

        # Bedrock establishment?
        if commit_hash in ctx.resolved_rules:
            for owned_path in sorted(ctx.resolved_rules[commit_hash]):
                # Aggregate sources for all files belonging to this owned_path
                sources_for_path: Set[str] = set()
                for file_path, file_sources in sources.items():
                    if file_path == owned_path or file_path.startswith(f"{owned_path.rstrip('/')}/"):
                        # Check if this owned_path is indeed the best match for the file
                        # (to avoid overlapping rule issues in reporting)
                        best_match = None
                        for rule_path in ctx.path_to_bedrock:
                            if file_path == rule_path or file_path.startswith(f"{rule_path.rstrip('/')}/"):
                                if best_match is None or len(rule_path) > len(best_match):
                                    best_match = rule_path
                        if best_match == owned_path:
                            sources_for_path.update(file_sources)

                if not sources_for_path:
                    if ctx.all_files:
                        actions.append(f"- {UI.BLACK}{owned_path}{UI.RESET} {UI.ICON_BEDROCK} (Bedrock - No changes found in range)")
                    continue

                # If the path changed in OTHER commits, it's being seeped/percolated
                foreign_sources = sources_for_path - {commit_hash}

                if foreign_sources:
                    # If source index > bedrock index (actual_idx), source is newer -> Percolate Down
                    perc_count = len([h for h in foreign_sources if ctx.strata.index(h) > actual_idx])
                    seep_count = len(foreign_sources) - perc_count
                    details = []
                    if seep_count: details.append(f"↑ Seep Up from {seep_count} older strata")
                    if perc_count: details.append(f"↓ Percolate Down from {perc_count} newer strata")

                    lith_warn = f" {UI.GREY}{UI.ICON_LITHIFY} Lithify{UI.RESET}" if len(sources_for_path) > 1 else ""
                    actions.append(f"- {UI.BLACK}{owned_path}{UI.RESET} {UI.ICON_BEDROCK} (Establishing via {', '.join(details)}){lith_warn}")
                elif ctx.all_files:
                    actions.append(f"- {UI.BLACK}{owned_path}{UI.RESET} {UI.ICON_BEDROCK} (Established natively)")

                # Also list individual files that natively establish here
                if ctx.all_files:
                    native_files = schedule.get(commit_hash, {}).get(commit_hash, [])
                    for f in sorted(native_files):
                        if f == owned_path or f.startswith(f"{owned_path.rstrip('/')}/"):
                            actions.append(f"- {UI.BLACK}{f}{UI.RESET} {UI.ICON_BEDROCK} (Established natively)")

        # Foreign Seepage? (This commit provides changes to OTHER bedrocks)
        for bedrock_h, sources_map in schedule.items():
            if bedrock_h == commit_hash:
                continue
            if commit_hash in sources_map:
                if ctx.strata.index(commit_hash) > ctx.strata.index(bedrock_h):
                    direction = "Percolate Down"
                    icon = UI.ICON_PERCOLATE
                else:
                    direction = "Seep Up"
                    icon = UI.ICON_SEEP
                for f in sorted(sources_map[commit_hash]):
                    actions.append(f"- {UI.BLACK}{f}{UI.RESET} {icon} ({direction} to Bedrock {color_hash(bedrock_h)})")


        for action in sorted(actions):
            log(action)

    if not ctx.auto_approve:
        log("\nProceed to Phase 2? [Y/n/a] (a=Accept all and execute)", level="info")
        choice = get_single_key()
        if choice == "q":
            log("Aborted.")
            sys.exit(ExitCode.OK)
        if choice == "a":
            ctx.auto_approve = True
        elif choice == "n":
            log("Review aborted.")
            sys.exit(ExitCode.OK)


def _process_single_stratum(
    ctx: SeepageContext,
    summary: SeepageSummary,
    commit_hash: str,
    index: int,
    schedule: Dict[str, Dict[str, List[str]]],
    sources: Dict[str, Set[str]],
):
    """Handles metadata extraction and file state reconstruction for a single stratum."""
    log_res = run_git(
        [
            "log",
            "-1",
            f"--format=%an{UI.DELIM}%ae{UI.DELIM}%ad{UI.DELIM}%cn{UI.DELIM}%ce{UI.DELIM}%cd{UI.DELIM}%B",
            commit_hash,
        ]
    )
    parts = log_res.stdout.strip().split(UI.DELIM)
    an, ae, ad, cn, ce, cd, body = parts

    # Custom environment for identity preservation
    env = {
        "GIT_AUTHOR_NAME": an,
        "GIT_AUTHOR_EMAIL": ae,
        "GIT_AUTHOR_DATE": ad,
        "GIT_COMMITTER_NAME": cn,
        "GIT_COMMITTER_EMAIL": ce,
        "GIT_COMMITTER_DATE": cd,
    }

    log(f"\n{index}/{len(ctx.strata)} {color_hash(commit_hash)} - {body.splitlines()[0]}")
    summary.strata_processed += 1

    # Reconstruction Phase:
    # 1. Reset index and working tree to this stratum's original baseline.
    # This brings in all normal modifications for files NOT governed by seepage rules.
    run_git(["read-tree", "-u", "--reset", commit_hash])

    # 2. Iterate through all owned paths and enforce their correct state.
    # We must sort by length so that the most specific paths are processed first
    # (though force_path_state just uses paths, sorting helps conceptual ordering)
    for owned_path in sorted(ctx.path_to_bedrock.keys(), key=len, reverse=True):
        bedrock_commit = ctx.path_to_bedrock[owned_path]

        if commit_hash == bedrock_commit:
            _handle_bedrock_path(ctx, summary, commit_hash, owned_path, schedule, sources)
        else:
            # Revert the owned path to the state it had in the rebuilt HEAD.
            # This effectively ERASES any changes made to this path in this stratum,
            # transferring them to the bedrock commit instead.
            force_path_state(owned_path, "HEAD")

    # 3. Finalize the stratum commit
    run_git(["commit", "-a", "--allow-empty", "-m", body], check=True, env_vars=env)


def _handle_bedrock_path(
    ctx: SeepageContext,
    summary: SeepageSummary,
    commit_hash: str,
    owned_path: str,
    schedule: Dict[str, Dict[str, List[str]]],
    sources: Dict[str, Set[str]],
):
    """Ensures a bedrock-owned path has the correct (final) state in its bedrock commit."""
    # 1. Gather all files that should be migrated to this bedrock from ALL strata
    sources_map = schedule.get(commit_hash, {})
    all_landing_files: Set[str] = set()
    for src_h, files in sources_map.items():
        for f in files:
            # Does this file belong to the current owned_path?
            if f == owned_path or f.startswith(f"{owned_path.rstrip('/')}/"):
                all_landing_files.add(f)

                # Stats
                src_idx = ctx.strata.index(src_h)
                bed_idx = ctx.strata.index(commit_hash)

                if src_h == commit_hash:
                    summary.bedrock_files += 1
                elif src_idx < bed_idx:
                    log(f"  [{UI.ICON_SEEP}] ↑ Seep Up {UI.BLACK}{f}{UI.RESET} to Bedrock from {color_hash(src_h)}")
                    summary.seep_files += 1
                    summary.seep_paths.add(f)
                else:
                    log(f"  [{UI.ICON_PERCOLATE}] ↓ Percolate Down {UI.BLACK}{f}{UI.RESET} to Bedrock from {color_hash(src_h)}")
                    summary.percolate_files += 1
                    summary.percolate_paths.add(f)

                if len(sources.get(f, set())) > 1:
                    if ctx.no_lithify:
                        raise ConfigurationError(
                            f"Policy Violation: Lithification detected for '{f}'. "
                            "Multiple historical versions exist in the range, "
                            "but --no-lithify is enabled."
                        )
                    if f not in summary.lithified_files:
                        summary.lithified_files[f] = []
                    if src_h not in summary.lithified_files[f]:
                        summary.lithified_files[f].append(src_h)


    # 2. Perform the hard-state projection
    # Force the path to its final state from original_head
    log(f"  [{UI.ICON_BEDROCK}] Capturing sediment for '{UI.BLACK}{owned_path}{UI.RESET}'")
    force_path_state(owned_path, ctx.original_head)


def _perform_sedimentation(ctx: SeepageContext, summary: SeepageSummary):
    """Syncs the final state of bedrock commits to their respective feature branches."""
    if not ctx.date_to_branch:
        return

    log("\n--- Phase 4: Sedimentation (autonomous branch sync) ---")
    for date_str, branch_name in ctx.date_to_branch.items():
        # Find the bedrock commit for this date
        bedrock_h = resolve_commit_by_date(date_str)

        # We find the NEWLY created commit that corresponds to this bedrock_h
        # We can do this by searching the current work branch log for the message
        log_res = run_git(["log", "-1", "--format=%B", bedrock_h])
        msg_header = log_res.stdout.strip().splitlines()[0]

        # Find the hash on our NEW branch with this message
        new_res = run_git(["log", "--format=%H", f"--grep={re.escape(msg_header)}", "-1"])
        new_h = new_res.stdout.strip()

        if new_h:
            log(f"  [{UI.ICON_BRANCH}] Syncing {UI.BOLD}{branch_name}{UI.RESET} to {color_hash(new_h)}")
            run_git(["branch", "-f", branch_name, new_h])
            summary.sedimented_branches.add(branch_name)


def _finalize_seepage(ctx: SeepageContext, summary: SeepageSummary, temp_branch_name: str):
    """Verifies parity and prints the final achievement report."""
    # 1. Parity Check
    current_state = run_git(["rev-parse", "HEAD"]).stdout.strip()
    diff_res = run_git(["diff", ctx.original_head, current_state])
    summary.parity_passed = not diff_res.stdout.strip()

    # 2. Summary Report
    log("\n" + "=" * 50)
    log("      seepage final summary report")
    if ctx.dry_run:
        log("             (dry run simulation)")
    log("=" * 50)

    log(f"{'strata processed':<20} {summary.strata_processed}")
    log(f"{'bedrock files':<20} {summary.bedrock_files}   {UI.ICON_BEDROCK}")

    if summary.seep_files:
        log(f"{'seep actions':<20} {summary.seep_files}   {UI.ICON_SEEP}")
        for p in sorted(summary.seep_paths): log(f"  ⮑  {p}")

    if summary.percolate_files:
        log(f"{'percolate actions':<20} {summary.percolate_files}   {UI.ICON_PERCOLATE}")
        for p in sorted(summary.percolate_paths): log(f"  ⮑  {p}")

    if summary.lithified_files:
        log(f"{'lithification':<20} {len(summary.lithified_files)}   {UI.ICON_LITHIFY}")
        for p, hashes in sorted(summary.lithified_files.items()):
            hashes_str = ", ".join(color_hash(h) for h in hashes)
            log(f"  ⮑  {p} [{hashes_str}]")

    if summary.sedimented_branches:
        log(f"{'sedimented branches':<20} {len(summary.sedimented_branches)}   {UI.ICON_BRANCH}")
        for b in sorted(summary.sedimented_branches): log(f"  ⮑  {b}")

    log("-" * 50)
    if summary.parity_passed:
        log(f"achievement unlocked: perfect stratigraphy", level="success")
        log(f"history matches original state {color_hash(ctx.original_head)}")
    else:
        log("warning: stratigraphy mismatch", level="warn")
        log("The reconstructed history deviates from the original state.")

    if ctx.dry_run:
        log("\ndry run complete. no changes were made.")
    log("=" * 50 + "\n")

    if ctx.dry_run or ctx.stage_only:
        if ctx.stage_only:
            log("\n--- Stage-only mode ---")
            log(f"Refactored history prepared on: {temp_branch_name}")
            log(f"To finalize: git checkout {ctx.target_branch} && "
                f"git reset --hard {temp_branch_name}")
        return

    # Update target branch pointer and cleanup
    run_git(["checkout", "-B", ctx.target_branch, temp_branch_name])
    run_git(["branch", "-D", temp_branch_name])
    log(f"\nBranch '{ctx.target_branch}' has been updated.")


def seep_history(
    rules_dict: Dict[str, List[str]],
    target_branch: str,
    *,  # Enforce keyword-only arguments
    all_files: bool = False,
    auto_approve: bool = False,
    no_lithify: bool = False,
    stage_only: bool = False,
    dry_run: bool = False,
    base_commit: Optional[str] = None,
):
    """Refactors git history by percolating changes to bedrock commits.

    Args:
        rules_dict: Mapping of date strings to lists of paths.
        target_branch: The branch to update with refactored history.
        all_files: Whether to list all files in bedrock commits.
        auto_approve: Whether to skip non-critical user confirmations.
        no_lithify: Whether to prevent multi-stratum squashing and error instead.
        stage_only: Whether to exit after preparing the temp branch.
        dry_run: Whether to simulate without modifying history.
        base_commit: Optional starting commit for the refactor range.
    """
    ctx = _initialize_seepage_context(
        rules_dict,
        target_branch,
        all_files=all_files,
        auto_approve=auto_approve,
        no_lithify=no_lithify,
        stage_only=stage_only,
        dry_run=dry_run,
        base_commit=base_commit,
    )

    with git_work_session(original_branch=ctx.current_branch):
        _print_preflight_briefing(ctx)

        # Calculation: Build the initial raw proposal
        proposed_schedule, sources = _calculate_percolation_schedule(ctx)

        if ctx.dry_run:
            # Simulate 'Accept All' and 'Auto Approve' for dry run stats
            ctx.auto_approve = True
            schedule = proposed_schedule
            summary = SeepageSummary()
            summary.parity_passed = True # Assume parity for simulation

            # Calculate stats from schedule
            for bedrock_hash, sources_map in schedule.items():
                for src_hash, files in sources_map.items():
                    bedrock_idx = ctx.strata.index(bedrock_hash)
                    src_idx = ctx.strata.index(src_hash)

                    for f in files:
                        file_sources = sources[f]
                        if len(file_sources) > 1:
                            if f not in summary.lithified_files:
                                summary.lithified_files[f] = []
                            if src_hash not in summary.lithified_files[f]:
                                summary.lithified_files[f].append(src_hash)

                        if src_hash == bedrock_hash:
                            summary.bedrock_files += 1
                        elif src_idx < bedrock_idx:
                            summary.seep_files += 1
                            summary.seep_paths.add(f)
                        else:
                            summary.percolate_files += 1
                            summary.percolate_paths.add(f)

            # Mock sedimentation branches
            for date_str, branch_name in ctx.date_to_branch.items():
                summary.sedimented_branches.add(branch_name)

            _finalize_seepage(ctx, summary, "dry-run-branch")
            return

        # Phase 1: Read-Only Review
        _perform_read_only_review(ctx, proposed_schedule, sources)

        # Phase 2: Selection (Interactive Exclusion)
        selector = HistorySelector(ctx, proposed_schedule, sources)
        schedule = selector.run()

        # Phase 3: Execution (History Reconstruction)
        log("\n--- Phase 3: Execution (history reconstruction) ---")
        temp_branch_name = f"gitseep-work-{uuid.uuid4().hex[:8]}"
        run_git(["checkout", "-b", temp_branch_name, ctx.parent_of_strata])

        summary = SeepageSummary()

        for i, commit_hash in enumerate(ctx.strata):
            _process_single_stratum(
                ctx, summary, commit_hash, i + 1, schedule, sources
            )

        # Phase 4: Sedimentation (Run before final report to include stats)
        _perform_sedimentation(ctx, summary)

        _finalize_seepage(ctx, summary, temp_branch_name)


def main():
    """Main entry point for the GitSeep CLI."""
    parser = argparse.ArgumentParser(
        description="GitSeep: Geological History Percolation"
    )
    # go/keep-sorted start
    parser.add_argument(
        "--all",
        dest="all_files",
        action="store_true",
        help="List all files in bedrock strata, not just changes",
    )
    parser.add_argument(
        "-y",
        "--auto-approve",
        action="store_true",
        help="Skip interactive confirmations",
    )
    parser.add_argument(
        "--base",
        help="Base commit to start refactor from (defaults to oldest bedrock)",
    )
    parser.add_argument(
        "-B", "--branch", help="Target branch name (defaults to current)"
    )
    parser.add_argument(
        "-n",
        "--dry-run",
        action="store_true",
        help="Simulate the refactor without modifying history",
    )
    parser.add_argument(
        "--no-lithify",
        action="store_true",
        help="Prevent squashing multiple historical versions of a file",
    )
    parser.add_argument(
        "--rules", help="Path to YAML seepage rules file"
    )
    parser.add_argument(
        "--stage-only",
        action="store_true",
        help="Leave result on temporary branch for inspection",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Enable debug logging",
    )
    # go/keep-sorted end
    args = parser.parse_args()

    if args.verbose:
        logger.setLevel(logging.DEBUG)

    try:
        current_branch = (
            run_git(["rev-parse", "--abbrev-ref", "HEAD"]).stdout.strip()
        )
    except GitError:
        log("Could not determine current branch.", level="error")
        sys.exit(ExitCode.ERROR)

    target_branch = args.branch or current_branch

    # Load rules from local or script directory
    rules_path = args.rules
    if not rules_path:
        local_rules = Path.cwd() / RULES_FILENAME
        script_rules = Path(__file__).parent / RULES_FILENAME
        if local_rules.exists():
            rules_path = local_rules
        elif script_rules.exists():
            rules_path = script_rules

    if not rules_path or not Path(rules_path).exists():
        msg = f"Seepage rules not found. Please create {RULES_FILENAME}."
        log(msg, level="error")
        sys.exit(ExitCode.NOINPUT)

    with open(rules_path, "r", encoding="utf-8") as f:
        try:
            rules_dict = yaml.safe_load(f)
        except yaml.YAMLError as e:
            log(f"Error parsing YAML: {e}", level="error")
            sys.exit(ExitCode.DATAERR)

    if not isinstance(rules_dict, dict):
        log("Rules must be a mapping of strata dates to path lists.", level="error")
        sys.exit(ExitCode.DATAERR)

    try:
        seep_history(
            rules_dict,
            target_branch=target_branch,
            all_files=args.all_files,
            auto_approve=args.auto_approve,
            no_lithify=args.no_lithify,
            stage_only=args.stage_only,
            dry_run=args.dry_run,
            base_commit=args.base,
        )
    except Error as e:
        log(str(e), level="error")
        sys.exit(ExitCode.ERROR)
    except Exception as e:
        log(f"Unexpected fatal error: {e}", level="error")
        sys.exit(ExitCode.ERROR)


if __name__ == "__main__":
    main()
