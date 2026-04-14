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

# Android Studio for Platform Layer Architecture

The **Android Studio for Platform (ASFP) Layer** provides a specialized development environment on top of the GNOME foundation. It follows a "thin layer" philosophy, centralizing core system logic in the base image and utilizing declarative configuration for AOSP-specific tools.

## Layer Responsibility

| Feature | Responsibility | Integration Method |
| :--- | :--- | :--- |
| **Toolchain** | ASFP IDE, AOSP tools, and ABFS client | Declarative (`EXTRA_PKGS` & `EXTRA_DEB_URLS`) |
| **Virtualization** | Cuttlefish emulator support (KVM) | Base Layer Propagation & Hooks |
| **Desktop UX** | ASFP launcher and Activities integration | XDG Desktop Files |

## Build-Time Integration

This layer leverages the base blueprint's `configure_workstation.sh` to perform automated setup:

1.  **Asset Injection**: The `Dockerfile` copies `assets/` to the image root, placing desktop entries, startup scripts, and build hooks.
2.  **Centralized Setup**: The base setup script automatically handles regional APT configuration, installs packages listed in `EXTRA_PKGS`, and executes numeric hooks in `assets/build-hooks.d/`.
3.  **Cuttlefish Build**: A separate build stage handles the compilation of Cuttlefish Debian packages, which are then injected and installed in the main system.

## Session Lifecycle

ASFP integrates with the GNOME session via standard XDG mechanisms:
*   **Desktop Integration**: Custom `.desktop` files ensure ASFP is available in the GNOME Activities view.
*   **Startup Logic**: Scripts in `/etc/workstation-startup.d/` handle runtime initialization, such as adding the user to necessary groups for virtualization.
