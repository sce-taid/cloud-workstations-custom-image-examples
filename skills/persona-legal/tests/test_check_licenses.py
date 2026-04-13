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

"""Unit tests for the license checker tool."""

import sys
import unittest
from pathlib import Path

# Ensure the scripts directory is in the path to import check_licenses
SCRIPT_DIR = Path(__file__).parent.parent
sys.path.append(str(SCRIPT_DIR / "scripts"))

import check_licenses


class TestCheckLicenses(unittest.TestCase):
    """Test suite for check_licenses logic."""

    def test_adds_license_to_empty_file(self):
        """Verify that a license is correctly added to an empty file."""
        content, added, updated, diff_holder = check_licenses.process_file_content(
            content="",
            ext="py",
            current_year=2026,
            holder="Google LLC",
            target_license="Apache-2.0",
        )
        self.assertTrue(added)
        self.assertFalse(updated)
        self.assertIsNone(diff_holder)
        self.assertIn("Copyright 2026 Google LLC", content)
        self.assertIn(
            'Licensed under the Apache License, Version 2.0 (the "License");', content
        )

    def test_updates_past_year(self):
        """Verify that a past copyright year is updated to a range."""
        initial = '# Copyright 2024 Google LLC\n#\n# Licensed under the Apache License, Version 2.0 (the "License");'
        content, added, updated, diff_holder = check_licenses.process_file_content(
            content=initial,
            ext="py",
            current_year=2026,
            holder="Google LLC",
            target_license="Apache-2.0",
        )
        self.assertFalse(added)
        self.assertTrue(updated)
        self.assertIsNone(diff_holder)
        self.assertIn("Copyright 2024-2026 Google LLC", content)

    def test_updates_past_year_range(self):
        """Verify that an existing year range is updated to the current year."""
        initial = '# Copyright 2022-2024 Google LLC\n#\n# Licensed under the Apache License, Version 2.0 (the "License");'
        content, added, updated, diff_holder = check_licenses.process_file_content(
            content=initial,
            ext="py",
            current_year=2026,
            holder="Google LLC",
            target_license="Apache-2.0",
        )
        self.assertFalse(added)
        self.assertTrue(updated)
        self.assertIsNone(diff_holder)
        self.assertIn("Copyright 2022-2026 Google LLC", content)

    def test_warns_on_different_holder(self):
        """Verify that a different copyright holder is correctly detected."""
        initial = "# Copyright 2024 Google Inc. All Rights Reserved.\n"
        content, added, updated, diff_holder = check_licenses.process_file_content(
            content=initial,
            ext="py",
            current_year=2026,
            holder="Google LLC",
            target_license="Apache-2.0",
        )
        self.assertTrue(added)
        self.assertFalse(updated)
        self.assertEqual("Google Inc. All Rights Reserved.", diff_holder)

    def test_ignores_current_year(self):
        """Verify that a header with the current year is left unchanged."""
        initial = '# Copyright 2026 Google LLC\n#\n# Licensed under the Apache License, Version 2.0 (the "License");'
        content, added, updated, diff_holder = check_licenses.process_file_content(
            content=initial,
            ext="py",
            current_year=2026,
            holder="Google LLC",
            target_license="Apache-2.0",
        )
        self.assertFalse(added)
        self.assertFalse(updated)
        self.assertIsNone(diff_holder)
        self.assertEqual(initial, content)

    def test_preserves_old_year_on_new_license_addition(self):
        """Verify that the original start year is preserved when adding a new license."""
        initial = "# Copyright 2024 Google LLC\n\nprint('hello')"
        content, added, updated, diff_holder = check_licenses.process_file_content(
            content=initial,
            ext="py",
            current_year=2026,
            holder="Google LLC",
            target_license="Apache-2.0",
        )
        self.assertTrue(added)
        self.assertIn("Copyright 2024-2026 Google LLC", content)

    def test_preserves_old_year_on_header_replacement_md(self):
        """Verify year preservation during full header replacement in Markdown."""
        initial = '<!--\nCopyright 2024 Google LLC\n\nLicensed under the Apache License, Version 2.0 (the "License");\n... \n-->\n\n# Body'
        content, added, updated, diff_holder = check_licenses.process_file_content(
            content=initial,
            ext="md",
            current_year=2026,
            holder="Google LLC",
            target_license="Apache-2.0",
        )
        self.assertTrue(added)
        self.assertIn("Copyright 2024-2026 Google LLC", content)

    def test_preserves_old_year_range_on_header_replacement(self):
        """Verify year range preservation during full header replacement in TypeScript."""
        initial = "/**\n * Copyright 2022-2024 Google LLC\n */\n\nexport const x = 1;"
        content, added, updated, diff_holder = check_licenses.process_file_content(
            content=initial,
            ext="ts",
            current_year=2026,
            holder="Google LLC",
            target_license="Apache-2.0",
        )
        self.assertTrue(added)
        self.assertIn("Copyright 2022-2026 Google LLC", content)


if __name__ == "__main__":
    unittest.main()
