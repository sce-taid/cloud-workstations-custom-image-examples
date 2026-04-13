// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"

	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/config"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/engine"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/logger"
	"github.com/sce-taid/cloud-workstations-custom-image-examples/skills/persona-swe/scripts/internal/gitseep/models"
)

var (
	rulesFile         string
	branch            string
	base              string
	allFiles          bool
	autoApprove       bool
	noLithify         bool
	stageOnly         bool
	dryRun            bool
	verbose           bool
	quiet             bool
	amend             bool
	experimentalGoGit bool
)

var rootCmd = &cobra.Command{
	Use:   "gitseep",
	Short: "GitSeep: Geological Source Code History Percolation",
	Long: `GitSeep automates Synthetic History Reconstruction. It reconstructs your
developer branch stratum-by-stratum to perfectly map code changes to source code
Bedrock layers based on declarative .gitseep.yaml rules.`,
	SilenceUsage:  true, // Don't print help on every error
	SilenceErrors: true, // We handle our own logging
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Init(verbose, quiet)

		if experimentalGoGit && !autoApprove && !quiet {
			logger.Warn("You have enabled an experimental feature: go-git integration.")
			logger.Warn("This implementation has known bugs with symlinks and can corrupt your worktree.")
			fmt.Print("Are you sure you want to proceed? [YES/NO]: ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToUpper(response))
			if response != "YES" {
				logger.Info("Aborted.")
				return nil
			}
		}

		// Parse configuration
		cfg, err := config.Load(rulesFile)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		baseRef := base
		if baseRef == "" {
			baseRef = cfg.Global.BaseRef
		}

		opts := models.Options{
			TargetBranch:      branch,
			BaseCommit:        baseRef,
			AllFiles:          allFiles,
			AutoApprove:       autoApprove || quiet,
			NoLithify:         noLithify,
			StageOnly:         stageOnly,
			DryRun:            dryRun,
			Amend:             amend,
			Quiet:             quiet,
			ExperimentalGoGit: experimentalGoGit,
		}

		return engine.Run(cfg, opts)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate GitSeep stratigraphy and workspace cleanliness (Pre-commit hook)",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Init(verbose, quiet)

		repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
		if err != nil {
			return fmt.Errorf("failed to open git repository: %w", err)
		}

		cfg, err := config.Load(rulesFile)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		baseRef := base
		if baseRef == "" {
			baseRef = cfg.Global.BaseRef
		}

		opts := models.Options{
			TargetBranch: branch,
			BaseCommit:   baseRef,
			Amend:        amend,
			Quiet:        quiet,
		}

		// Pre-flight safety check
		if err := engine.VerifyWorktreeIsClean(repo, opts); err != nil {
			return err
		}

		return engine.Check(cfg, opts)
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)

	rootCmd.PersistentFlags().StringVarP(&rulesFile, "config", "c", "", "Path to YAML seepage rules file (defaults to .gitseep.yaml)")
	rootCmd.PersistentFlags().StringVarP(&branch, "branch", "B", "", "Target branch name (defaults to current)")
	rootCmd.PersistentFlags().StringVar(&base, "base", "", "Base commit to start refactor from (defaults to oldest bedrock)")
	rootCmd.PersistentFlags().BoolVar(&allFiles, "all-files", false, "List all files in bedrock strata, not just changes")
	rootCmd.PersistentFlags().BoolVarP(&autoApprove, "auto-approve", "y", false, "Skip interactive confirmations")
	rootCmd.PersistentFlags().BoolVar(&noLithify, "no-lithify", false, "Prevent squashing multiple historical versions of a file")
	rootCmd.PersistentFlags().BoolVar(&noLithify, "no-squash", false, "Alias for --no-lithify")
	rootCmd.PersistentFlags().BoolVar(&stageOnly, "stage-only", false, "Leave result on temporary branch for inspection")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Simulate the refactor without modifying history")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output (useful with -a -y)")
	rootCmd.PersistentFlags().BoolVarP(&amend, "amend", "a", false, "Amend uncommitted changes into the last commit before running")
	rootCmd.PersistentFlags().BoolVar(&experimentalGoGit, "experimental-go-git", false, "Use buggy go-git for status/checkout instead of system git")
	if err := rootCmd.PersistentFlags().MarkHidden("experimental-go-git"); err != nil {
		logger.Debug("Failed to hide experimental flag: %v", err)
	}
}
