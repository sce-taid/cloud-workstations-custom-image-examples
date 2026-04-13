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

# Antigravity Developer Guide

This guide explains how to customize and extend the **Antigravity Layer**.

## Customization Patterns

### 1. Adding New Tools via APT

To add a new tool that is available via a public or private APT repository:

1.  **Add Repo**: Place a `.list` file in `assets/etc/apt/sources.list.d/`.
2.  **Register Package**: Add the package name to the `ANTIGRAVITY_PKGS` argument in the `Dockerfile`.
3.  **Optional Hook**: If the tool requires post-install configuration, create a script in `assets/build-hooks.d/`.

### 2. Injecting Desktop Configurations

- **GSettings**: Use a build hook to apply system-wide overrides in `/usr/share/glib-2.0/schemas/`.
- **Autostart**: Place `.desktop` files in `assets/etc/xdg/autostart/` to launch applications on session start.
- **Default Icons**: Add custom `.desktop` files to `assets/usr/share/applications/` to appear in the Activities view.

### 3. Modifying the Autostart Logic

The `antigravity` package provides its own entry. To override:

- Place a custom `antigravity.desktop` file in `assets/etc/xdg/autostart/`.

## Build and Validation

### Triggering a Build

The Antigravity image is managed via the **cicd-foundation** blueprint. The primary way to build is by committing changes to the repository.

For local source builds, use the provided skill script:

```bash
# From the repository root
./skills/persona-sre/scripts/trigger_build.sh examples/images/antigravity
```

### Validation Workflow

Follow the mandatory [Agent Validation Lifecycle](../../../../AGENTS.md#2-agent-validation-lifecycle):

1.  Run local tests: `skills/validate-image-updates/scripts/run_local_tests.sh`.
2.  Monitor the Cloud Build to completion.
3.  Start the workstation and verify the `antigravity` package is functional.
