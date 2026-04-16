#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DOCKER_INIT_SQL="release/deployment/docker-compose/bootstrap/mysql-init/init-sql"
DOCKER_PATCH_SQL="release/deployment/docker-compose/bootstrap/mysql-init/patch-sql"
HELM_INIT_SQL="release/deployment/helm-chart/charts/app/bootstrap/init/mysql/init-sql"

SCHEMADIFF="${SCHEMADIFF_BIN:-schemadiff}"

errors=0

log_error() {
    echo "❌ ERROR: $1"
    errors=$((errors + 1))
}

log_ok() {
    echo "✅ $1"
}

log_info() {
    echo "ℹ️  $1"
}

extract_create_table() {
    local file="$1"
    local out="$2"
    sed -n '/^CREATE TABLE/,/;$/p' "$file" > "$out"
}

echo "========================================"
echo "  MySQL Schema Consistency Check"
echo "========================================"
echo ""

if ! command -v "$SCHEMADIFF" &>/dev/null; then
    log_info "schemadiff not found, installing..."
    go install github.com/planetscale/schemadiff/cmd/schemadiff@latest
    SCHEMADIFF="$(go env GOPATH)/bin/schemadiff"
fi

echo "--- Check 1: CREATE TABLE schema consistency (docker-compose vs helm-chart) ---"
echo ""

check1_start_errors=$errors
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

for file in "$REPO_ROOT/$DOCKER_INIT_SQL"/*.sql; do
    filename=$(basename "$file")
    helm_file="$REPO_ROOT/$HELM_INIT_SQL/$filename"

    if [ ! -f "$helm_file" ]; then
        log_error "File '$filename' exists in docker-compose init-sql but missing in helm-chart init-sql"
        continue
    fi

    docker_create="$TMP_DIR/docker_$filename"
    helm_create="$TMP_DIR/helm_$filename"
    extract_create_table "$file" "$docker_create"
    extract_create_table "$helm_file" "$helm_create"

    if [ ! -s "$docker_create" ]; then
        continue
    fi

    diff_output=$("$SCHEMADIFF" diff-table --source "$docker_create" --target "$helm_create" 2>&1) || true
    if [ -n "$diff_output" ]; then
        log_error "Table schema differs for '$filename' between docker-compose and helm-chart:"
        echo "  $diff_output"
    fi
done

for file in "$REPO_ROOT/$HELM_INIT_SQL"/*.sql; do
    filename=$(basename "$file")
    if echo "$filename" | grep -qE '_alter\.sql|_proc\.sql'; then
        docker_file="$REPO_ROOT/$DOCKER_PATCH_SQL/$filename"
        if [ ! -f "$docker_file" ]; then
            log_error "Alter/proc file '$filename' exists in helm-chart but missing in docker-compose patch-sql"
        fi
    else
        docker_file="$REPO_ROOT/$DOCKER_INIT_SQL/$filename"
        if [ ! -f "$docker_file" ]; then
            log_error "File '$filename' exists in helm-chart init-sql but missing in docker-compose init-sql"
        fi
    fi
done

if [ $errors -eq $check1_start_errors ]; then
    log_ok "All CREATE TABLE schemas are consistent between docker-compose and helm-chart"
fi
echo ""

echo "--- Check 2: Patch/Alter SQL files consistency ---"
echo ""

check2_start_errors=$errors
for file in "$REPO_ROOT/$DOCKER_PATCH_SQL"/*.sql; do
    filename=$(basename "$file")
    helm_file="$REPO_ROOT/$HELM_INIT_SQL/$filename"
    if [ ! -f "$helm_file" ]; then
        log_error "Patch file '$filename' exists in docker-compose but missing in helm-chart"
    elif ! diff -q "$file" "$helm_file" > /dev/null 2>&1; then
        log_error "Patch file '$filename' differs between docker-compose and helm-chart"
        diff "$file" "$helm_file" || true
    fi
done

if [ $errors -eq $check2_start_errors ]; then
    log_ok "All patch/alter SQL files are consistent between docker-compose and helm-chart"
fi
echo ""

echo "--- Check 3: ALTER SQL completeness for modified tables ---"
echo ""

check3_start_errors=$errors

if [ -n "${GITHUB_BASE_REF:-}" ]; then
    log_info "Running in PR mode, checking changed files against base branch"

    git fetch origin "$GITHUB_BASE_REF" --depth=1 2>/dev/null || true

    changed_init_files=$(git diff --name-only "origin/$GITHUB_BASE_REF"...HEAD -- "$DOCKER_INIT_SQL"/*.sql 2>/dev/null || true)

    for changed_file in $changed_init_files; do
        if [ -z "$changed_file" ]; then
            continue
        fi

        filename=$(basename "$changed_file")
        table_name="${filename%.sql}"
        alter_file="$REPO_ROOT/$DOCKER_PATCH_SQL/${table_name}_alter.sql"

        if echo "$filename" | grep -qE '_alter\.sql|_proc\.sql|alter_proc\.sql'; then
            continue
        fi

        old_content=$(git show "origin/$GITHUB_BASE_REF:$changed_file" 2>/dev/null || echo "")

        if [ -z "$old_content" ]; then
            log_info "New table file '$filename', no ALTER check needed"
            continue
        fi

        old_create="$TMP_DIR/old_$filename"
        new_create="$TMP_DIR/new_$filename"
        echo "$old_content" | sed -n '/^CREATE TABLE/,/;$/p' > "$old_create"
        sed -n '/^CREATE TABLE/,/;$/p' "$REPO_ROOT/$changed_file" > "$new_create"

        if [ ! -s "$old_create" ] || [ ! -s "$new_create" ]; then
            continue
        fi

        schema_diff=$("$SCHEMADIFF" diff-table --source "$old_create" --target "$new_create" 2>&1) || true

        if [ -z "$schema_diff" ]; then
            continue
        fi

        has_add_column=$(echo "$schema_diff" | grep -ci "ADD COLUMN" || true)
        has_add_key=$(echo "$schema_diff" | grep -ciE "ADD (UNIQUE )?KEY|ADD INDEX" || true)

        if [ "$has_add_column" -gt 0 ] || [ "$has_add_key" -gt 0 ]; then
            if [ ! -f "$alter_file" ]; then
                log_error "Table '$table_name' has schema changes but no ALTER file found at: $DOCKER_PATCH_SQL/${table_name}_alter.sql"
                echo "  Schema diff: $schema_diff"
            else
                added_columns=$(echo "$schema_diff" | grep -oi 'ADD COLUMN `[^`]*`' | sed "s/ADD COLUMN \`//;s/\`//" | sort -u || true)
                added_indexes=$(echo "$schema_diff" | grep -oiE 'ADD (UNIQUE )?KEY `[^`]*`' | sed 's/.*`//;s/`//' | sort -u || true)
                alter_content=$(cat "$alter_file")

                for col in $added_columns; do
                    if ! echo "$alter_content" | grep -qi "ADD.*COLUMN.*\`$col\`"; then
                        log_error "Column '$col' was added to '$table_name' but has no corresponding ALTER TABLE ADD COLUMN in ${table_name}_alter.sql"
                    fi
                done

                for idx in $added_indexes; do
                    if ! echo "$alter_content" | grep -qi "ADD.*INDEX\|ADD.*KEY.*\`$idx\`"; then
                        log_error "Index '$idx' was added to '$table_name' but has no corresponding ALTER TABLE ADD INDEX in ${table_name}_alter.sql"
                    fi
                done
            fi
        fi
    done
else
    log_info "Not running in PR mode (GITHUB_BASE_REF not set), skipping incremental ALTER check"
    log_info "To test locally, set GITHUB_BASE_REF=main"
fi

if [ $errors -eq $check3_start_errors ]; then
    log_ok "All schema changes have corresponding ALTER statements"
fi
echo ""

echo "========================================"
if [ $errors -gt 0 ]; then
    echo "  ❌ Found $errors error(s)"
    echo "========================================"
    exit 1
else
    echo "  ✅ All checks passed"
    echo "========================================"
    exit 0
fi
