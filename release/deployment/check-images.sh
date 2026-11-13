#!/usr/bin/env bash

# Verify that every image pinned in the docker-compose .env file is still
# resolvable in its registry.
#
# Only the image manifest is requested, so no image layers are downloaded.
#
# Usage:
#   ./check-images.sh [path/to/.env]
#
# Exit codes:
#   0  every image reference is resolvable
#   1  at least one image reference is NOT resolvable
#   2  usage error or missing prerequisite

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_ENV_FILE="${SCRIPT_DIR}/docker-compose/.env"
ENV_FILE="${1:-${DEFAULT_ENV_FILE}}"

usage() {
    echo "Usage: $(basename "$0") [path/to/.env]"
    echo
    echo "Checks that every image pinned in the given .env file can be resolved"
    echo "from its registry. Defaults to: ${DEFAULT_ENV_FILE}"
}

strip_quotes() {
    local value="$1"
    value="${value#\"}"
    value="${value%\"}"
    value="${value#\'}"
    value="${value%\'}"
    printf '%s' "${value}"
}

# Transient network failures are retried once. A missing tag or a repository
# that was removed upstream is NOT transient and must be reported as a failure.
is_transient() {
    printf '%s' "$1" | grep -qiE \
        'timeout|connection reset|tls handshake|temporary failure|no such host|502|503|504'
}

# Turn COZE_LOOP_<NAME>_IMAGE_REGISTRY / _REPOSITORY / _NAME / _TAG groups
# into one "PREFIX|REGISTRY|REPOSITORY|NAME|TAG" line per image.
#
# Association arrays are avoided on purpose: the default bash on macOS is 3.2.
parse_image_refs() {
    tr -d '\r' < "$ENV_FILE" | awk -F= '
        /^[[:space:]]*(#|$)/ { next }
        {
            key = $1
            sub(/^[[:space:]]+/, "", key)
            sub(/[[:space:]]+$/, "", key)

            value = substr($0, index($0, "=") + 1)
            sub(/^[[:space:]]+/, "", value)
            sub(/[[:space:]]+$/, "", value)

            if (key ~ /_IMAGE_REGISTRY$/) {
                prefix = key
                sub(/_IMAGE_REGISTRY$/, "", prefix)
                registry[prefix] = value
            } else if (key ~ /_IMAGE_REPOSITORY$/) {
                prefix = key
                sub(/_IMAGE_REPOSITORY$/, "", prefix)
                repository[prefix] = value
            } else if (key ~ /_IMAGE_NAME$/) {
                prefix = key
                sub(/_IMAGE_NAME$/, "", prefix)
                name[prefix] = value
            } else if (key ~ /_IMAGE_TAG$/) {
                prefix = key
                sub(/_IMAGE_TAG$/, "", prefix)
                printf "%s|%s|%s|%s|%s\n", prefix, registry[prefix], repository[prefix], name[prefix], value
            }
        }
    '
}

check_ref() {
    local ref="$1"
    local output

    if output="$(docker manifest inspect "$ref" 2>&1)"; then
        return 0
    fi

    if is_transient "$output"; then
        sleep 2
        if output="$(docker manifest inspect "$ref" 2>&1)"; then
            return 0
        fi
    fi

    # The docker CLI prefixes its diagnostics with a bare "errors:" line, so
    # report the first meaningful line instead of that header.
    LAST_ERROR=""
    while IFS= read -r line; do
        if [[ -z "$line" || "$line" == "errors:" ]]; then
            continue
        fi
        LAST_ERROR="${line:0:160}"
        break
    done <<< "$output"
    return 1
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
fi

if [[ ! -f "$ENV_FILE" ]]; then
    echo "error: env file not found: ${ENV_FILE}" >&2
    exit 2
fi

if ! command -v docker > /dev/null 2>&1; then
    echo "error: the docker CLI is required" >&2
    exit 2
fi

total=0
failed=0
failed_refs=""

while IFS='|' read -r prefix registry repository name tag; do
    registry="$(strip_quotes "$registry")"
    repository="$(strip_quotes "$repository")"
    name="$(strip_quotes "$name")"
    tag="$(strip_quotes "$tag")"

    if [[ -z "$name" || -z "$tag" ]]; then
        continue
    fi

    if [[ -z "$registry" ]]; then
        registry="docker.io"
    fi
    if [[ -z "$repository" ]]; then
        repository="library"
    fi

    ref="${registry}/${repository}/${name}:${tag}"
    total=$((total + 1))

    label="${prefix#COZE_LOOP_}"
    if [[ "$label" == "$prefix" ]]; then
        label="$prefix"
    fi

    LAST_ERROR=""
    if check_ref "$ref"; then
        printf '  OK    %-12s %s\n' "$label" "$ref"
    else
        printf '  FAIL  %-12s %s\n' "$label" "$ref"
        if [[ -n "$LAST_ERROR" ]]; then
            printf '        %s\n' "$LAST_ERROR"
        fi
        failed=$((failed + 1))
        failed_refs="${failed_refs}${ref}"$'\n'
    fi
done < <(parse_image_refs)

if [[ "$total" -eq 0 ]]; then
    echo "error: no *_IMAGE_* variables found in ${ENV_FILE}" >&2
    exit 2
fi

echo
if [[ "$failed" -eq 0 ]]; then
    echo "all ${total} image references are resolvable"
    exit 0
fi

echo "${failed}/${total} image references are NOT resolvable:"
printf '%s' "$failed_refs"
exit 1
