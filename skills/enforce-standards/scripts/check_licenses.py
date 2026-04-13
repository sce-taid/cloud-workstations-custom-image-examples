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
import argparse
import datetime
import logging
import re
import subprocess
import sys
from pathlib import Path
from typing import Dict, List, Optional, Tuple
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


def log(msg: str = "", level: str = "info"):
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
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA."""
}


def get_tracked_files(filter_regex: str) -> List[str]:
    """Returns a list of git-tracked files matching the filter regex.

    Args:
        filter_regex: A regular expression to filter file paths.

    Returns:
        A list of matching file paths relative to the repository root.
    """
    try:
        result = subprocess.run(
            ["git", "ls-files"],
            stdout=subprocess.PIPE,
            text=True,
            check=True
        )
        files = result.stdout.splitlines()
        filter_pattern = re.compile(filter_regex, re.IGNORECASE)
        return [f for f in files if filter_pattern.search(f)]
    except subprocess.CalledProcessError as e:
        log(f"Error running git ls-files: {e}", level="error")
        sys.exit(1)


def process_file_content(
    content: str,
    ext: str,
    current_year: int,
    holder: str,
    target_license: str
) -> Tuple[str, bool, bool, Optional[str]]:
    """Processes the file content to ensure license compliance.

    Args:
        content: The original file content.
        ext: File extension (lowercase).
        current_year: The year to use for new/updated headers.
        holder: The copyright holder name.
        target_license: SPDX license identifier.

    Returns:
        A tuple of (modified_content, license_added, year_updated, different_holder).
    """
    original_content = content
    license_added = False
    year_updated = False
    different_holder = None

    # 0. Extract start year if it exists for the target holder before any deletions
    start_year = current_year
    year_pattern = re.compile(
        rf"Copyright ([0-9]{{4}})(?:-[0-9]{{4}})? {re.escape(holder)}",
        re.IGNORECASE
    )
    year_match = year_pattern.search(original_content)
    if year_match:
        start_year = int(year_match.group(1))

    # Check for different copyright holders (for warnings)
    holder_pattern = re.compile(
        r"Copyright [0-9]{4}(?:-[0-9]{4})? (.*?)(?:\n|$)",
        re.IGNORECASE
    )
    for match in holder_pattern.finditer(original_content):
        found_holder = match.group(1).strip()
        if holder.lower() not in found_holder.lower():
            different_holder = found_holder

    # 1. Remove all redundant short headers (if any)
    content = re.sub(
        r'/\*\*\s*\n\s*\* Copyright.*?\n\s*\*/(?:\s*\n)?',
        '', content, flags=re.DOTALL
    )
    content = re.sub(
        r'<!--\s*\n\s*Copyright.*?\n\s*-->(?:\s*\n)?',
        '', content, flags=re.DOTALL
    )

    # 2. Check for full license string
    if (target_license in LICENSES and
        LICENSES[target_license].splitlines()[0] not in content):
        license_added = True
        effective_year = (
            f"{start_year}-{current_year}"
            if start_year < current_year
            else str(current_year)
        )
        copyright_line = f"Copyright {effective_year} {holder}\n\n"
        full_header_text = copyright_line + LICENSES[target_license]

        if ext in ['sh', 'bash', 'bats', 'py', 'desktop']:
            formatted_header = '\n'.join(
                [f"# {line}" if line else "#" for line in full_header_text.splitlines()]
            ) + '\n\n'
            if content.startswith('#!'):
                lines = content.splitlines(True)
                content = lines[0] + '\n' + formatted_header + ''.join(lines[1:])
            else:
                content = formatted_header + content
        elif ext in ['html', 'md']:
            formatted_header = f"<!--\n{full_header_text}\n-->\n\n"
            content = formatted_header + content
        elif ext in ['css', 'ts', 'js']:
            formatted_header = "/**\n" + '\n'.join(
                [f" * {line}" if line else " *" for line in full_header_text.splitlines()]
            ) + "\n */\n\n"
            content = formatted_header + content

    # 3. Update copyright year to range if in the past
    pattern = re.compile(
        rf"Copyright ([0-9]{{4}})(?:-[0-9]{{4}})? {re.escape(holder)}",
        re.IGNORECASE
    )
    def year_replacer(match):
        nonlocal year_updated
        match_start_year = int(match.group(1))
        if match_start_year < current_year:
            year_updated = True
            return f"Copyright {match_start_year}-{current_year} {holder}"
        return match.group(0)

    content = pattern.sub(year_replacer, content)

    # 4. Remove extra leading blank lines
    content = re.sub(r'^\n+', '', content)

    # 5. Enforce blank line spacing rules even if license already existed
    end_markers = [
        r'limitations under the License\.',
        r'DEALINGS IN THE SOFTWARE\.',
        r'02110-1301, USA\.'
    ]
    end_pattern = r'(' + r'|'.join(end_markers) + r')'

    if ext in ['sh', 'bash', 'bats', 'py', 'desktop']:
        content = re.sub(r'^(#!.*?\n)\n*(# Copyright)', r'\1\n\2', content)
        content = re.sub(rf'(# {end_pattern})\n+(?!$)', r'\1\n\n', content)
    elif ext in ['html', 'md']:
        content = re.sub(rf'({end_pattern}\n-->)\n+(?!$)', r'\1\n\n', content)
    elif ext in ['css', 'ts', 'js']:
        content = re.sub(rf'({end_pattern}\n \*/)\n+(?!$)', r'\1\n\n', content)

    return content, license_added, year_updated, different_holder


def main():
    """Main entry point for the license checker CLI."""
    parser = argparse.ArgumentParser(description="Check and apply license headers.")
    # go/keep-sorted start
    parser.add_argument(
        "--exclude",
        default="node_modules/ dist/ licenses/ check_licenses.py",
        help="Space-separated list of strings to exclude."
    )
    parser.add_argument(
        "--filter",
        default=r"\.(ts|js|sh|css|html|md|bash|bats|py|desktop)$",
        help="Regex to filter files."
    )
    parser.add_argument(
        "--holder",
        default="Google LLC",
        help="Copyright holder name."
    )
    parser.add_argument(
        "--license",
        default="Apache-2.0",
        help="SPDX license identifier to enforce."
    )
    # go/keep-sorted end
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

        ext = file_path.suffix.lstrip('.').lower()
        # Use keyword arguments for best practice
        content, license_added, year_updated, different_holder = process_file_content(
            content=original_content,
            ext=ext,
            current_year=current_year,
            holder=args.holder,
            target_license=args.license
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
                level="warn"
            )

    if modified_files:
        log(
            f"Applied license headers or fixes to {len(modified_files)} files.",
            level="warn"
        )
        sys.exit(1)
    else:
        log("All tracked files already have valid license headers and formatting.", level="success")
        sys.exit(0)


if __name__ == "__main__":
    main()
