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

# Key URLs
# keep-sorted start
ADOPTIUM_KEY_URL="https://packages.adoptium.net/artifactory/api/gpg/key/public"
DOCKER_KEY_URL="https://download.docker.com/linux/ubuntu/gpg"
GOOGLE_CHROME_KEY_URL="https://dl.google.com/linux/linux_signing_key.pub"
GOOGLE_CLOUD_SDK_KEY_URL="https://packages.cloud.google.com/apt/doc/apt-key.gpg"
HELM_KEY_URL="https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x4B196BE9C4313D06"
NODESOURCE_KEY_URL="https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key"
NVIDIA_KEY_URL="https://nvidia.github.io/libnvidia-container/gpgkey"
YARN_KEY_URL="https://dl.yarnpkg.com/debian/pubkey.gpg"
# keep-sorted end

CURL_OPTS="-fsSL"

declare -A KEYS_TO_DOWNLOAD=(
  # keep-sorted start
  ["adoptium"]="${ADOPTIUM_KEY_URL}"
  ["docker"]="${DOCKER_KEY_URL}"
  ["google-chrome"]="${GOOGLE_CHROME_KEY_URL}"
  ["google-cloud-sdk"]="${GOOGLE_CLOUD_SDK_KEY_URL}"
  ["helm"]="${HELM_KEY_URL}"
  ["nodesource"]="${NODESOURCE_KEY_URL}"
  ["nvidia"]="${NVIDIA_KEY_URL}"
  ["yarn"]="${YARN_KEY_URL}"
  # keep-sorted end
)

# Target directory
TARGET_DIR="examples/images/gnome/assets/etc/apt/keyrings"

# Create target directory if it doesn't exist
mkdir -p "$TARGET_DIR"

download_key() {
  local key_name="$1"
  local url="$2"
  echo "Downloading ${key_name} key..."
  curl ${CURL_OPTS} "$url" | gpg --dearmor --yes -o "$TARGET_DIR/${key_name}.gpg"
}

# Download keys
for key_name in "${!KEYS_TO_DOWNLOAD[@]}"; do
  download_key "$key_name" "${KEYS_TO_DOWNLOAD[$key_name]}"
done

echo "All keys downloaded successfully."
