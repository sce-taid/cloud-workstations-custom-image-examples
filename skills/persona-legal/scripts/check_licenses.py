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

"""Check and apply license headers to tracked files."""

# go/keep-sorted start
from pathlib import Path
from typing import Dict, List, Optional, Tuple
import argparse
import datetime
import re
import subprocess
import sys
# go/keep-sorted end

# --- Constants & Style ---


class UI:
    """Centralized UI constants and styling."""

    RESET = "\033[0m"
    BOLD = "\033[1m"
    RED = "\033[31m"
    GREEN = "\033[32m"
    YELLOW = "\033[33m"
    ICON_SUCCESS = "✅"
    ICON_WARN = "⚠️"


def log(msg: str = "", level: str = "info") -> None:
    """Standardized logger for the license checker."""
    if level == "error":
        print(f"{UI.RED}error: {msg}{UI.RESET}", file=sys.stderr)
    elif level == "warn":
        print(f"{UI.ICON_WARN} {UI.BOLD}warning:{UI.RESET} {msg}")
    elif level == "success":
        print(f"{UI.ICON_SUCCESS} {msg}")
    else:
        print(msg)


LICENSES: Dict[str, str] = {
    "Apache-2.0": """\
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.""",
    "MIT": """\
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.""",
    "GPL-2.0-only": """\
This program is free software; you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation; version 2 of the License.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.""",
}


class HeaderFormatter:
    """Handles formatting of license headers for different file types."""

    @staticmethod
    def format(text: str, ext: str) -> str:
        """Wraps text in comment markers based on file extension."""
        if ext in ["sh", "bash", "bats", "py", "desktop"]:
            return (
                "\n".join([f"# {line}" if line else "#" for line in text.splitlines()])
                + "\n\n"
            )
        if ext in ["html", "md"]:
            return f"<!--\n{text}\n-->\n\n"
        if ext in ["css", "ts", "js"]:
            formatted = "/**\n"
            formatted += "\n".join(
                [f" * {line}" if line else " *" for line in text.splitlines()]
            )
            formatted += "\n */\n\n"
            return formatted
        return text


def get_original_copyright_year(content: str, holder: str, current_year: int) -> int:
    """Extracts the earliest copyright year for a specific holder."""
    pattern = re.compile(
        rf"Copyright\s+([0-9]{{4}})(?:-[0-9]{{4}})?\s+{re.escape(holder)}",
        re.IGNORECASE | re.VERBOSE,
    )
    match = pattern.search(content)
    return int(match.group(1)) if match else current_year


def check_foreign_holders(content: str, target_holder: str) -> Optional[str]:
    """Detects if a different copyright holder exists in the file."""
    pattern = re.compile(
        r"Copyright\s+[0-9]{4}(?:-[0-9]{4})?\s+(.*?)(?:\n|$)",
        re.IGNORECASE | re.VERBOSE,
    )
    for match in pattern.finditer(content):
        found_holder = match.group(1).strip()
        if target_holder.lower() not in found_holder.lower():
            return found_holder
    return None


def strip_redundant_headers(content: str) -> str:
    """Removes short or malformed headers before re-applying the full one."""
    content = re.sub(
        r"/\*\*\s*\n\s*\* Copyright.*?\n\s*\*/(?:\s*\n)?",
        "",
        content,
        flags=re.DOTALL | re.VERBOSE,
    )
    content = re.sub(
        r"<!--\s*\n\s*Copyright.*?\n\s*-->(?:\s*\n)?",
        "",
        content,
        flags=re.DOTALL | re.VERBOSE,
    )
    return content


def get_tracked_files(filter_regex: str) -> List[str]:
    """Returns a list of git-tracked files matching the filter regex."""
    try:
        result = subprocess.run(
            ["git", "ls-files"], stdout=subprocess.PIPE, text=True, check=True
        )
        files = result.stdout.splitlines()
        filter_pattern = re.compile(filter_regex, re.IGNORECASE)
        return [f for f in files if filter_pattern.search(f)]
    except subprocess.CalledProcessError as e:
        log(f"Error running git ls-files: {e}", level="error")
        sys.exit(1)


def process_file_content(
    content: str, ext: str, current_year: int, holder: str, target_license: str
) -> Tuple[str, bool, bool, Optional[str]]:
    """Processes the file content to ensure license compliance."""
    original_content = content
    license_added = False
    year_updated = False

    different_holder = check_foreign_holders(original_content, holder)
    start_year = get_original_copyright_year(original_content, holder, current_year)
    content = strip_redundant_headers(content)

    # 1. Check for full license string
    license_text = LICENSES.get(target_license)
    if license_text and license_text.splitlines()[0] not in content:
        license_added = True
        effective_year = (
            f"{start_year}-{current_year}"
            if start_year < current_year
            else str(current_year)
        )
        full_header_text = f"Copyright {effective_year} {holder}\n\n{license_text}"
        formatted_header = HeaderFormatter.format(full_header_text, ext)

        if ext in ["sh", "bash", "bats", "py"] and content.startswith("#!"):
            lines = content.splitlines(True)
            content = lines[0] + "\n" + formatted_header + "".join(lines[1:])
        else:
            content = formatted_header + content

    # 2. Update copyright year to range if in the past
    year_pattern = re.compile(
        rf"Copyright\s+([0-9]{{4}})(?:-[0-9]{{4}})?\s+{re.escape(holder)}",
        re.IGNORECASE | re.VERBOSE,
    )

    def year_replacer(match: re.Match) -> str:
        nonlocal year_updated
        match_start_year = int(match.group(1))
        if match_start_year < current_year:
            year_updated = True
            return f"Copyright {match_start_year}-{current_year} {holder}"
        return match.group(0)

    content = year_pattern.sub(year_replacer, content)

    # 3. Final cleanup and spacing
    content = re.sub(r"^\n+", "", content)
    end_markers = [
        r"limitations\s+under\s+the\s+License\.",
        r"DEALINGS\s+IN\s+THE\s+SOFTWARE\.",
        r"02110-1301,\s+USA\.",
    ]
    end_pattern = r"(" + r"|".join(end_markers) + r")"

    if ext in ["sh", "bash", "bats", "py", "desktop"]:
        content = re.sub(r"^(#!.*?\n)\n*(# Copyright)", r"\1\n\2", content)
        content = re.sub(rf"(# {end_pattern})\n+(?!$)", r"\1\n\n", content)
    elif ext in ["html", "md"]:
        content = re.sub(rf"({end_pattern}\n-->)\n+(?!$)", r"\1\n\n", content)
    elif ext in ["css", "ts", "js"]:
        content = re.sub(rf"({end_pattern}\n\s*\*/)\n+(?!$)", r"\1\n\n", content)

    return content, license_added, year_updated, different_holder


def main() -> None:
    """Main entry point for the license checker CLI."""
    parser = argparse.ArgumentParser(description="Check and apply license headers.")
    parser.add_argument(
        "--exclude",
        default="node_modules/ dist/ licenses/ check_licenses.py",
        help="Space-separated list of strings to exclude.",
    )
    parser.add_argument(
        "--filter",
        default=r"\.(ts|js|sh|css|html|md|bash|bats|py|desktop)$",
        help="Regex to filter files.",
    )
    parser.add_argument("--holder", default="Google LLC", help="Copyright holder name.")
    parser.add_argument(
        "--license", default="Apache-2.0", help="SPDX license identifier to enforce."
    )
    args = parser.parse_args()

    exclude_list = args.exclude.split()
    current_year = datetime.datetime.now().year

    files_to_check = get_tracked_files(args.filter)
    modified_files: List[str] = []

    for file_str in files_to_check:
        file_path = Path(file_str)
        if any(ex in str(file_path) for ex in exclude_list):
            continue

        if not file_path.exists() or file_path.is_symlink():
            continue

        try:
            original_content = file_path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue

        ext = file_path.suffix.lstrip(".").lower()
        content, license_added, year_updated, different_holder = process_file_content(
            content=original_content,
            ext=ext,
            current_year=current_year,
            holder=args.holder,
            target_license=args.license,
        )

        if content != original_content:
            file_path.write_text(content, encoding="utf-8")
            modified_files.append(str(file_path))
            if license_added:
                log(f"  - Applying {args.license} license to {file_path}")
            elif year_updated:
                log(f"  - Updating copyright year in {file_path}")
            else:
                log(f"  - Fixing license formatting in {file_path}")

        if different_holder:
            log(
                f"Different copyright holder ('{different_holder}') found in {file_path}",
                level="warn",
            )

    if modified_files:
        log(
            f"Applied license headers or fixes to {len(modified_files)} files.",
            level="warn",
        )
        sys.exit(1)
    else:
        log(
            "All tracked files already have valid license headers and formatting.",
            level="success",
        )
        sys.exit(0)


if __name__ == "__main__":
    main()
