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

# Android Studio for Platform Technical Overview

The **Android Studio for Platform (ASFP) Layer** provides a specialized environment for platform-level Android development. It extends the **GNOME Blueprint** with the full AOSP toolchain and emulator support.

## Feature Stack

| Layer | Component | Description |
| :--- | :--- | :--- |
| **IDE** | Android Studio for Platform | Optimized IDE for navigating and developing the AOSP codebase. |
| **Build Tools** | AOSP Toolchain | Pre-installed packages (bison, build-essential, etc.) and `repo` tool. |
| **Virtualization** | Cuttlefish Emulator | Full support for running Android virtual devices (KVM-enabled). |
| **Storage** | ABFS Client | Optional integration for the Android Build File System to speed up builds. |

## Key Technologies

*   **Thin Layer Architecture**: Extends the GNOME foundation via declarative configuration, inheriting all base security and system updates.
*   **Cuttlefish in the Cloud**: Pre-configured KVM and kernel dependencies for low-latency Android emulation via the browser.
*   **Android Build File System (ABFS)**: Optimized I/O for large-scale AOSP builds, reducing sync times and improving developer productivity.
*   **Centralized Automation**: Utilizes the foundation's `configure_workstation.sh` and numeric build hooks for modular toolchain setup.

## Use Cases

*   **Platform Engineering**: Developing and testing core Android OS features.
*   **AOSP Onboarding**: Rapid deployment of standardized development environments.
*   **Automated Testing**: Reproducible system-level testing using Cuttlefish.

---
Brought to you by the Google Cloud Professional Services Organization Apps team (PSO AppΣ)
