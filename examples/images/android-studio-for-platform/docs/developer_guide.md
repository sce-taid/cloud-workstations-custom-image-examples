<!--
Copyright 2026 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# Android Studio for Platform Developer Guide

This guide explains how to customize and extend the **Android Studio for Platform (ASFP) Layer**.

## Customization Patterns

### 1. Adding Build Tools
To add new packages required for your AOSP builds:
*   Add the package name to the `AOSP_PKGS` argument in the `Dockerfile`.
*   The foundation's setup script will install it during the build process.

### 2. Modifying Startup Configuration
To add system-level tweaks for virtualization or builds:
*   Place scripts in `assets/etc/workstation-startup.d/`.
*   Place `.conf` files in `assets/etc/sysctl.d/` for kernel parameter tuning.

### 3. Adding Helper Scripts
*   Place new automation scripts in `assets/google/scripts/`.
*   These will automatically be marked as executable and available to all users.

## Build and Validation

### Triggering a Build
The ASFP image is managed via the **cicd-foundation** blueprint. The primary way to build is by committing changes to the repository.

For local source builds, use the provided skill script:

```bash
# From the repository root
./skills/manage-cws-lifecycle/scripts/trigger_build.sh examples/images/android-studio-for-platform
```

### Validation Workflow
Follow the mandatory [Agent Validation Lifecycle](../../../../AGENTS.md#2-agent-validation-lifecycle):
1.  Run local tests: `skills/validate-image-updates/scripts/run_local_tests.sh`.
2.  Monitor the Cloud Build to completion.
3.  Start the workstation and verify ASFP launches correctly and ABFS (if enabled) mounts the source tree.
