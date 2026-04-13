#!/bin/bash

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

set -euo pipefail

DESKTOP_FILE="/usr/share/applications/antigravity.desktop"

if [[ -f "${DESKTOP_FILE}" ]]; then
  echo "Patching ${DESKTOP_FILE} with required Electron flags..."
  # Inject flags immediately after the binary path in all Exec lines
  sed -i 's|^Exec=/usr/share/antigravity/antigravity|& --disable-gpu --disable-gpu-compositing --no-sandbox --ozone-platform=wayland --force-disable-user-env|g' "${DESKTOP_FILE}"
  echo "Successfully patched Antigravity desktop entry."
else
  echo "Warning: ${DESKTOP_FILE} not found. Skipping patch."
fi
