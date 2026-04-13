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

# GNOME Blueprint: Developer Guide

This guide explains how to build upon the GNOME base layer to create customized workstation images. It is intended for platform engineers and developers who need to add new tools, applications, and desktop integrations.

## Centralized Configuration

The GNOME blueprint uses a centralized configuration script (`/google/scripts/build/configure_workstation.sh`) to provide a consistent build orchestration for all child layers. This script is the primary entrypoint for customization and should be called in the `Dockerfile` of any image that inherits from `gnome`.

### Build Orchestration

The script executes the following steps in a precise order:
1.  **APT Configuration**: Adjusts APT sources for the target region (`GCP_REGION`).
2.  **Build Hooks**: Executes any custom scripts found in `/build-hooks.d/`. This is the ideal place to add new APT repositories or perform layer-specific setup *before* packages are installed.
3.  **`apt-get update`**: Refreshes package lists.
4.  **Transport Detection**: Automatically detects if any repository uses the `ar+` protocol and installs `apt-transport-artifact-registry` if needed.
5.  **Package Installation**:
    - Installs all packages listed in the `EXTRA_PKGS` environment variable.
    - Downloads and installs all `.deb` files listed in the `EXTRA_DEB_URLS` environment variable.
6.  **Desktop Integration**: Calls the `desktop_apply_integration` utility to handle autostart and dock pinning.
7.  **Cleanup**: Runs `apt-get autoremove`, `apt-get clean`, and removes APT lists to minimize layer size.

### Declarative Installation Pattern

To add software, child layers should use the `ARG` -> `ENV` mapping pattern in their `Dockerfile`. This provides a single source of truth for all installed packages.

**Example (`android-studio-for-platform/Dockerfile`)**:
```dockerfile
# 1. Stage-specific ARGs for package lists
ARG AOSP_PKGS="..."
ARG ABFS_PKGS="..."
ARG EXTRA_DEB_URLS="..."

# 2. Map to ENVs for the centralized installer
ENV EXTRA_PKGS="${AOSP_PKGS} ${ABFS_PKGS}"
ENV EXTRA_DEB_URLS=${EXTRA_DEB_URLS}

# 3. Merge assets (including .desktop files and hooks)
COPY assets/ /

# 4. Run centralized configuration
RUN /google/scripts/build/configure_workstation.sh &&
    rm -rf /build-hooks.d/
```

## Desktop Integration

Desktop integration (autostart, dock pinning, menu visibility) is managed automatically through a metadata-driven system.

### Autodiscovery

The `desktop_integration.sh` utility automatically performs the following actions:
- **Autostart**: Any application with `X-GNOME-Autostart-enabled=true` in its `.desktop` file will be symlinked into `/etc/xdg/autostart/`.
- **Dock Pinning**: Any application with `Categories=` containing `Development` or `IDE` will be automatically pinned to the GNOME dock.

### Manual Registration

For more granular control, you can use the `desktop_register_app` function within a build hook (`/build-hooks.d/`).

**Usage**: `desktop_register_app <desktop_file_path> [priority] [autostart] [favorite]`

- **Example**: To pin an application with a high priority but disable autostart:
  ```bash
  # In /build-hooks.d/10_register_my_app.sh
  source /google/scripts/build/desktop_integration.sh
  desktop_register_app "/usr/share/applications/my-app.desktop" 5 false true
  ```
