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

# Antigravity Technical Overview

The **Antigravity Layer** provides a high-performance desktop environment for cloud-native development and hackathon scenarios. It extends the **GNOME Blueprint** with specialized tools and a pre-configured dashboard.

## Feature Stack

| Layer              | Component                   | Description                                      |
| :----------------- | :-------------------------- | :----------------------------------------------- |
| **User Interface** | Antigravity Desktop Shell   | Customized launcher and participants' dashboard. |
| **Development**    | Agent Development Kit (ADK) | Python tools for building and testing AI agents. |
| **AI Integration** | Gemini CLI                  | Terminal access to Gemini models for automation. |

## Key Technologies

- **Thin Layer Architecture**: Patches the GNOME base image at build-time, ensuring security updates are inherited automatically.
- **Declarative Package Management**: Uses the foundation's `EXTRA_PKGS` mechanism for reliable toolchain installation.
- **Region Awareness**: Automatically adjusts APT sources for optimal download speeds during the build process.

## Use Cases

- **Hackathons**: Rapid deployment of identical, tool-rich desktop environments.
- **AI Research**: Pre-configured environments with the ADK and Gemini CLI.
- **Corporate Golden Images**: Template for internal images with proprietary SDKs.

---

Brought to you by the Google Cloud Professional Services Organization Apps team (PSO AppΣ)
