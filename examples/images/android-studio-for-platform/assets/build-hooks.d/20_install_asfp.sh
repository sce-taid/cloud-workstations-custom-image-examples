#!/bin/bash

# Copyright 2024-2026 Google LLC
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

# Sourced logic (from preflight or local)
# shellcheck disable=SC1091
source "/google/scripts/common.sh"

install_asfp() {
  local asfp_version="${ASFP_VERSION:-canary}"

  # Update desktop entry to canary if applicable
  # Note: The package is installed by configure_workstation.sh via EXTRA_DEB_URLS
  local desktop_file="/usr/share/applications/jetbrains-studio.desktop"
  if [[ "${asfp_version}" == "canary" ]] && [[ -f "${desktop_file}" ]]; then
    echo "Patching desktop entry for Canary..."
    sed -i 's/android-studio-for-platform/android-studio-for-platform-canary/g' "${desktop_file}"
  fi
}

main() {
  install_asfp
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
