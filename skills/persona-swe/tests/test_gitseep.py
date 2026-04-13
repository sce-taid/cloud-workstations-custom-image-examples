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

"""Tests for GitSeep tool."""

import os
import unittest
from pathlib import Path
from unittest import mock
import gitseep


class TestGitseep(unittest.TestCase):
    """Test suite for GitSeep core logic."""

    @mock.patch("gitseep.run_git")
    def test_resolve_commit_by_date(self, mock_run_git):
        """Test resolving commit by date."""
        mock_run_git.return_value = mock.Mock(stdout="hash1 2026-04-15\n")
        h = gitseep.resolve_commit_by_date("2026-04-15")
        self.assertEqual(h, "hash1")
        mock_run_git.assert_called_once()

    @mock.patch("gitseep.run_git")
    def test_resolve_commit_by_date_not_found(self, mock_run_git):
        """Test date resolution when no commits are found."""
        mock_run_git.return_value = mock.Mock(stdout="")
        with self.assertRaises(gitseep.GitError):
            gitseep.resolve_commit_by_date("2026-04-15")

    @mock.patch("gitseep.run_git")
    @mock.patch("gitseep.Path")
    @mock.patch("gitseep.shutil.rmtree")
    def test_force_path_state(self, mock_rmtree, mock_path_cls, mock_run_git):
        """Test hard path state reset."""
        mock_path_instance = mock_path_cls.return_value
        mock_path_instance.exists.return_value = True

        # Test directory removal
        mock_path_instance.is_dir.return_value = True
        gitseep.force_path_state("dir", "HEAD")
        mock_rmtree.assert_called_once_with(mock_path_instance)

        # Test file removal
        mock_rmtree.reset_mock()
        mock_path_instance.is_dir.return_value = False
        gitseep.force_path_state("file.txt", "HEAD")
        mock_path_instance.unlink.assert_called_once()

        # Mocking ls-tree found for checkout
        mock_run_git.reset_mock()
        mock_run_git.return_value = mock.Mock(stdout="file.txt\n")
        gitseep.force_path_state("file.txt", "HEAD")
        # Use mock.ANY because str(Path) in mock context is messy
        mock_run_git.assert_any_call(["checkout", "HEAD", "--", mock.ANY])

    @mock.patch("gitseep.get_git_root")
    @mock.patch("gitseep.os.chdir")
    @mock.patch("gitseep.run_git")
    @mock.patch("gitseep.resolve_commit_by_date")
    @mock.patch("gitseep.get_single_key")
    @mock.patch("gitseep.force_path_state")
    def test_seep_history_success(
        self,
        mock_force,
        mock_get_key,
        mock_resolve,
        mock_run_git,
        mock_chdir,
        mock_root,
    ):
        """Test successful history percolation."""
        mock_root.return_value = Path("/repo")
        mock_resolve.return_value = "hash1"

        def run_git_side_effect(args, **kwargs):
            cmd = " ".join(str(a) for a in args)
            if "status" in cmd:
                return mock.Mock(stdout="")
            if "rev-parse HEAD" in cmd:
                return mock.Mock(stdout="ORIGINAL_HEAD")
            if "rev-parse --abbrev-ref" in cmd:
                return mock.Mock(stdout="main")
            if "log -1 --format=%at" in cmd:
                return mock.Mock(stdout="1000")
            if "rev-parse hash1^" in cmd:
                return mock.Mock(stdout="PARENT_HASH")
            if "log --format=%H" in cmd:
                return mock.Mock(stdout="hash1\n")
            if "log -1 --format=%ai|%s" in cmd:
                return mock.Mock(stdout="date|msg")
            if "log" in cmd and "--name-only" in cmd:
                return mock.Mock(stdout="[hash1]\npath1/file1.txt\n")
            if "log -1 --format=%B" in cmd:
                return mock.Mock(stdout="Message\n")
            if "|DELIM|" in cmd:
                # 7 fields expected
                return mock.Mock(stdout=f"an{gitseep.UI.DELIM}ae{gitseep.UI.DELIM}ad{gitseep.UI.DELIM}cn{gitseep.UI.DELIM}ce{gitseep.UI.DELIM}cd{gitseep.UI.DELIM}body")
            return mock.Mock(stdout="")

        mock_run_git.side_effect = run_git_side_effect
        # 1. 'y' for Proceed to Phase 2
        # 2. 'c' for Finish Phase 2
        mock_get_key.side_effect = ["y", "c"]

        rules = {"2026-04-15": ["path1"]}
        gitseep.seep_history(rules, target_branch="main")

        # Verify final branch update
        mock_run_git.assert_any_call(["checkout", "-B", "main", mock.ANY])

    @mock.patch("gitseep.get_git_root")
    @mock.patch("gitseep.os.chdir")
    @mock.patch("gitseep.run_git")
    @mock.patch("gitseep.resolve_commit_by_date")
    @mock.patch("gitseep.get_single_key")
    def test_seep_history_no_lithify_error(
        self, mock_get_key, mock_resolve, mock_run_git, mock_chdir, mock_root
    ):
        """Test policy violation on unpermitted lithification when --no-lithify is set."""
        mock_root.return_value = Path("/repo")
        mock_resolve.return_value = "bedrock_h"

        def run_git_side_effect(args, **kwargs):
            cmd = " ".join(str(a) for a in args)
            if "status" in cmd: return mock.Mock(stdout="")
            if "rev-parse HEAD" in cmd: return mock.Mock(stdout="ORIGINAL_HEAD")
            if "rev-parse --abbrev-ref" in cmd: return mock.Mock(stdout="main")
            if "log -1 --format=%at" in cmd: return mock.Mock(stdout="1000")
            if "rev-parse bedrock_h^" in cmd: return mock.Mock(stdout="PARENT_HASH")
            # Order bedrock_h first, then source_h
            if "log --format=%H" in cmd: return mock.Mock(stdout="bedrock_h\nsource_h\n")
            if "log -1 --format=%ai|%s" in cmd: return mock.Mock(stdout="date|msg")
            if "log" in cmd and "--name-only" in cmd: return mock.Mock(stdout="[source_h]\nfile.txt\n[bedrock_h]\nfile.txt\n")
            if "log -1 --format=%B" in cmd: return mock.Mock(stdout="Msg\n")
            if "|DELIM|" in cmd:
                return mock.Mock(stdout=f"an{gitseep.UI.DELIM}ae{gitseep.UI.DELIM}ad{gitseep.UI.DELIM}cn{gitseep.UI.DELIM}ce{gitseep.UI.DELIM}cd{gitseep.UI.DELIM}body")
            return mock.Mock(stdout="")

        mock_run_git.side_effect = run_git_side_effect
        mock_get_key.side_effect = ["y", "c"]

        rules = {"2026-04-15": ["file.txt"]}

        with self.assertRaises(SystemExit) as cm:
            gitseep.seep_history(rules, target_branch="main", no_lithify=True)
        # Should exit with error due to policy violation
        self.assertEqual(cm.exception.code, 1)

    @mock.patch("gitseep.run_git")
    def test_calculate_percolation_schedule(self, mock_run_git):
        """Test the logic that maps strata changes to bedrocks."""
        ctx = gitseep.SeepageContext(
            repo_root=Path("/repo"),
            original_head="h2",
            current_branch="main",
            target_branch="main",
            parent_of_strata="h0",
            strata=["h1", "h2"],
            resolved_rules={"h1": ["path1"]},
            path_to_bedrock={"path1": "h1"},
            date_to_branch={},
            all_files=False,
            auto_approve=False,
            no_lithify=False,
            stage_only=False,
            dry_run=False,
        )

        def run_git_side_effect(args, **kwargs):
            if "log" in args and "--name-only" in args:
                return mock.Mock(stdout="[h1]\npath1/file1.txt\n[h2]\npath1/file2.txt\n")
            return mock.Mock(stdout="")

        mock_run_git.side_effect = run_git_side_effect

        schedule, sources = gitseep._calculate_percolation_schedule(ctx)

        self.assertIn("h1", schedule)
        self.assertEqual(schedule["h1"]["h2"], ["path1/file2.txt"])
        self.assertEqual(sources["path1/file1.txt"], {"h1"})

    @mock.patch("gitseep.resolve_commit_by_date")
    @mock.patch("gitseep.run_git")
    def test_perform_sedimentation(self, mock_run_git, mock_resolve):
        """Test the floating pointer sedimentation sync."""
        mock_resolve.return_value = "bedrock_h"
        ctx = gitseep.SeepageContext(
            repo_root=Path("/repo"),
            original_head="HEAD_HASH",
            current_branch="main",
            target_branch="main",
            parent_of_strata="PARENT_HASH",
            strata=["h1", "h2"],
            resolved_rules={},
            path_to_bedrock={},
            date_to_branch={"2026-04-14": "feature/branch"},
            all_files=False,
            auto_approve=False,
            no_lithify=False,
            stage_only=False,
            dry_run=False,
        )

        def run_git_side_effect(args, **kwargs):
            cmd = " ".join(str(a) for a in args)
            if "log -1 --format=%B" in cmd:
                return mock.Mock(stdout="Bedrock Msg\n", returncode=0)
            if "log --format=%H --grep" in cmd:
                return mock.Mock(stdout="new_hash\n", returncode=0)
            return mock.Mock(stdout="", returncode=0)

        mock_run_git.side_effect = run_git_side_effect
        summary = gitseep.SeepageSummary()

        gitseep._perform_sedimentation(ctx, summary)

        mock_run_git.assert_any_call(["branch", "-f", "feature/branch", "new_hash"])
        self.assertIn("feature/branch", summary.sedimented_branches)

    @mock.patch("gitseep.get_git_root")
    @mock.patch("gitseep.os.chdir")
    @mock.patch("gitseep.run_git")
    @mock.patch("gitseep.resolve_commit_by_date")
    def test_seep_history_dry_run_no_mutation(
        self, mock_resolve, mock_run_git, mock_chdir, mock_root
    ):
        """Test that dry run simulation does not execute Git mutations."""
        mock_root.return_value = Path("/repo")
        mock_resolve.return_value = "h1"

        def run_git_side_effect(args, **kwargs):
            cmd = " ".join(str(a) for a in args)
            if "status" in cmd: return mock.Mock(stdout="")
            if "rev-parse HEAD" in cmd: return mock.Mock(stdout="hash\n")
            if "rev-parse h1^" in cmd: return mock.Mock(stdout="PARENT\n")
            if "rev-parse --abbrev-ref" in cmd: return mock.Mock(stdout="main\n")
            if "log -1 --format=%at" in cmd: return mock.Mock(stdout="1000")
            if "log -1 --format=%ai|%s" in cmd: return mock.Mock(stdout="date|msg")
            if "log --format=%H" in cmd: return mock.Mock(stdout="h1\n")
            if "log" in cmd and "--name-only" in cmd: return mock.Mock(stdout="[h1]\nf.txt\n")
            if "|DELIM|" in cmd:
                return mock.Mock(stdout=f"an{gitseep.UI.DELIM}ae{gitseep.UI.DELIM}ad{gitseep.UI.DELIM}cn{gitseep.UI.DELIM}ce{gitseep.UI.DELIM}cd{gitseep.UI.DELIM}body")
            return mock.Mock(stdout="")

        mock_run_git.side_effect = run_git_side_effect

        rules = {"2026-04-15": ["f.txt"]}
        gitseep.seep_history(rules, target_branch="main", dry_run=True)

        for call in mock_run_git.call_args_list:
            args = call[0][0]
            self.assertNotIn("checkout", args)
            self.assertNotIn("branch", args)
            self.assertNotIn("-B", args)
            self.assertNotIn("-D", args)

    @mock.patch("gitseep.get_git_root")
    @mock.patch("gitseep.os.chdir")
    @mock.patch("gitseep.run_git")
    @mock.patch("gitseep.resolve_commit_by_date")
    def test_dirty_worktree_exit(self, mock_resolve, mock_run_git, mock_chdir, mock_root):
        """Test that the tool exits if the working directory is not clean."""
        mock_root.return_value = Path("/repo")
        mock_run_git.side_effect = [
            mock.Mock(stdout="M file.txt\n"),  # status
        ]

        with self.assertRaises(gitseep.DirtyWorktreeError):
            gitseep._initialize_seepage_context(
                rules_dict={},
                target_branch="main",
                all_files=False,
                auto_approve=False,
                no_lithify=False,
                stage_only=False,
                dry_run=False,
                base_commit=None
            )

    @mock.patch("gitseep.get_git_root")
    @mock.patch("gitseep.os.chdir")
    @mock.patch("gitseep.run_git")
    @mock.patch("gitseep.resolve_commit_by_date")
    @mock.patch("gitseep.get_single_key")
    @mock.patch("gitseep.force_path_state")
    @mock.patch("gitseep._finalize_seepage")
    def test_selective_exclusion_effective(
        self, mock_finalize, mock_force, mock_get_key, mock_resolve, mock_run_git, mock_chdir, mock_root
    ):
        """Test that excluding a file in Phase 2 prevents its migration."""
        mock_root.return_value = Path("/repo")

        def run_git_side_effect(args, **kwargs):
            cmd = " ".join(str(a) for a in args)
            if "status" in cmd: return mock.Mock(stdout="")
            if "rev-parse HEAD" in cmd: return mock.Mock(stdout="hash")
            if "rev-parse --abbrev-ref" in cmd: return mock.Mock(stdout="main")
            if "log -1 --format=%at" in cmd: return mock.Mock(stdout="1000")
            if "rev-parse h1^" in cmd: return mock.Mock(stdout="PARENT")
            if "log -1 --format=%ai|%s" in cmd: return mock.Mock(stdout="date|msg")
            # Strata MUST contain all hashes used in the schedule
            if "log --format=%H" in cmd: return mock.Mock(stdout="h1\nsource_h\n")
            if "log" in cmd and "--name-only" in cmd:
                return mock.Mock(stdout="[source_h]\nexclude.txt\nkeep.txt\n")
            if "log -1 --format=%B" in cmd: return mock.Mock(stdout="Msg\n")
            if "|DELIM|" in cmd:
                return mock.Mock(stdout=f"an{gitseep.UI.DELIM}ae{gitseep.UI.DELIM}ad{gitseep.UI.DELIM}cn{gitseep.UI.DELIM}ce{gitseep.UI.DELIM}cd{gitseep.UI.DELIM}body")
            return mock.Mock(stdout="")

        mock_run_git.side_effect = run_git_side_effect
        mock_resolve.return_value = "h1"

        mock_get_key.side_effect = ["y", "x", "c"]

        rules = {"2026-04-15": ["keep.txt", "exclude.txt"]}
        gitseep.seep_history(rules, target_branch="main")

        summary = mock_finalize.call_args[0][1]
        # In this test setup, source_h (idx 1) -> h1 (idx 0) is a Percolate Down
        self.assertEqual(summary.percolate_files, 1)
        self.assertIn("keep.txt", summary.percolate_paths)
        self.assertNotIn("exclude.txt", summary.percolate_paths)

    def test_path_ownership_overlap(self):
        """Test that most specific (longest) path wins in rule resolution."""
        ctx = gitseep.SeepageContext(
            repo_root=Path("/repo"),
            original_head="HEAD",
            current_branch="main",
            target_branch="main",
            parent_of_strata="PARENT",
            strata=["h1", "h2"],
            resolved_rules={"h1": ["src/"], "h2": ["src/core/"]},
            path_to_bedrock={"src/": "h1", "src/core/": "h2"},
            date_to_branch={},
            all_files=False,
            auto_approve=False,
            no_lithify=False,
            stage_only=False,
            dry_run=False,
        )

        with mock.patch("gitseep.run_git") as mock_run:
            mock_run.return_value = mock.Mock(stdout="[h1]\nsrc/main.py\nsrc/core/api.py\n")
            schedule, sources = gitseep._calculate_percolation_schedule(ctx)

            self.assertIn("src/main.py", schedule["h1"]["h1"])
            self.assertIn("src/core/api.py", schedule["h2"]["h1"])
            self.assertNotIn("src/core/api.py", schedule["h1"]["h1"])

    @mock.patch("gitseep.run_git")
    @mock.patch("gitseep.log")
    def test_stratigraphy_mismatch_warning(self, mock_log, mock_run_git):
        """Test that a stratigraphy mismatch is detected and warned."""
        ctx = gitseep.SeepageContext(
            repo_root=Path("/repo"),
            original_head="ORIGINAL_HEAD",
            current_branch="main",
            target_branch="main",
            parent_of_strata="PARENT",
            strata=["h1"],
            resolved_rules={},
            path_to_bedrock={},
            date_to_branch={},
            all_files=False,
            auto_approve=False,
            no_lithify=False,
            stage_only=False,
            dry_run=False,
        )

        def run_git_side_effect(args, **kwargs):
            cmd = " ".join(str(a) for a in args)
            if "rev-parse HEAD" in cmd:
                return mock.Mock(stdout="CURRENT_HEAD\n")
            if "diff ORIGINAL_HEAD CURRENT_HEAD" in cmd:
                return mock.Mock(stdout="diff content\n")
            if "checkout -B" in cmd or "branch -D" in cmd:
                return mock.Mock(stdout="")
            return mock.Mock(stdout="")

        mock_run_git.side_effect = run_git_side_effect
        summary = gitseep.SeepageSummary()

        gitseep._finalize_seepage(ctx, summary, "temp_branch")

        self.assertFalse(summary.parity_passed)
        mock_log.assert_any_call("warning: stratigraphy mismatch", level="warn")
        mock_log.assert_any_call("The reconstructed history deviates from the original state.")

    @mock.patch("gitseep.Path.exists")
    @mock.patch("gitseep.argparse.ArgumentParser.parse_args")
    @mock.patch("gitseep.run_git")
    def test_main_no_rules_file(self, mock_run_git, mock_parse_args, mock_exists):
        """Test that main exits with NOINPUT when rules file is missing."""
        mock_parse_args.return_value = mock.Mock(
            all_files=False, auto_approve=False, base=None, branch=None,
            dry_run=False, no_lithify=False, rules=None, stage_only=False, verbose=False
        )
        mock_run_git.return_value = mock.Mock(stdout="main\n")
        # Mock exists to return False so neither local nor script rules are found
        mock_exists.return_value = False

        with self.assertRaises(SystemExit) as cm:
            gitseep.main()
        self.assertEqual(cm.exception.code, gitseep.ExitCode.NOINPUT)

    @mock.patch("gitseep.run_git")
    @mock.patch("gitseep.log")
    def test_stage_only_mode(self, mock_log, mock_run_git):
        """Test that stage_only exits without modifying target branch."""
        ctx = gitseep.SeepageContext(
            repo_root=Path("/repo"),
            original_head="ORIGINAL_HEAD",
            current_branch="main",
            target_branch="main",
            parent_of_strata="PARENT",
            strata=["h1"],
            resolved_rules={},
            path_to_bedrock={},
            date_to_branch={},
            all_files=False,
            auto_approve=False,
            no_lithify=False,
            stage_only=True,
            dry_run=False,
        )

        def run_git_side_effect(args, **kwargs):
            cmd = " ".join(str(a) for a in args)
            if "rev-parse HEAD" in cmd:
                return mock.Mock(stdout="ORIGINAL_HEAD\n")
            if "diff ORIGINAL_HEAD ORIGINAL_HEAD" in cmd:
                return mock.Mock(stdout="")
            return mock.Mock(stdout="")

        mock_run_git.side_effect = run_git_side_effect
        summary = gitseep.SeepageSummary()
        summary.parity_passed = True

        gitseep._finalize_seepage(ctx, summary, "temp_branch")

        # Ensure no checkout or branch deletion occurred
        for call in mock_run_git.call_args_list:
            args = call[0][0]
            self.assertNotIn("checkout", args)
            self.assertNotIn("-B", args)
            self.assertNotIn("-D", args)

        mock_log.assert_any_call("\n--- Stage-only mode ---")



if __name__ == "__main__":
    unittest.main()
