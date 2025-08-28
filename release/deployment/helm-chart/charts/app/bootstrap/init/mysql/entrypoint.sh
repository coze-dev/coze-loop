#!/bin/sh

exec 2>&1
set -e

# Define common paths and configurations
MYSQL_HOST="coze-loop-mysql"
MYSQL_USER="${COZE_LOOP_MYSQL_USER}"
MYSQL_DATABASE="${COZE_LOOP_MYSQL_DATABASE}"
BASE_INIT_PATH="/coze-loop-mysql-init/bootstrap/init-sql"

print_banner() {
  msg="$1"
  side=30
  content=" $msg "
  content_len=${#content}
  line_len=$((side * 2 + content_len))

  line=$(printf '*%.0s' $(seq 1 "$line_len"))
  side_eq=$(printf '*%.0s' $(seq 1 "$side"))

  printf "%s\n%s%s%s\n%s\n" "$line" "$side_eq" "$content" "$side_eq" "$line"
}

print_banner "Starting..."

export MYSQL_PWD="${COZE_LOOP_MYSQL_PASSWORD}"

for i in $(seq 1 60); do
  if mysql \
      -h "$MYSQL_HOST" \
      -u "$MYSQL_USER" \
      --silent --skip-column-names \
      -e "SELECT 1 FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME='$MYSQL_DATABASE'" \
      2>/dev/null \
      | grep -q 1; then
    break
  else
    sleep 1
  fi
  if [ "$i" -eq 60 ]; then
    echo "[ERROR] MySQL server or database('$MYSQL_DATABASE') not available after 60 time."
    exit 1
  fi
done

# Step 1: Execute CREATE TABLE statements (excluding stored procedures and alter files)
echo "Creating tables..."
# shellcheck disable=SC2010
i=1
for file in $(ls "$BASE_INIT_PATH" | grep '\.sql$' | grep -v '_alter\.sql$' | grep -v '_proc\.sql$'); do
  echo "+ Init #$i: $file"
  mysql \
    -h "$MYSQL_HOST" \
    -u "$MYSQL_USER" \
    -D "$MYSQL_DATABASE" \
    < "$BASE_INIT_PATH/${file}"
  i=$((i + 1))
done

# Step 2: Execute compatibility functions
echo "Loading compatibility functions..."
if [ -f "$BASE_INIT_PATH/alter_proc.sql" ]; then
  echo "+ Proc: alter_proc.sql"
  mysql \
    -h "$MYSQL_HOST" \
    -u "$MYSQL_USER" \
    -D "$MYSQL_DATABASE" \
    < "$BASE_INIT_PATH/alter_proc.sql"
fi

# Step 3: Execute ALTER TABLE statements
echo "Applying table alterations..."
# shellcheck disable=SC2010
for file in $(ls "$BASE_INIT_PATH" | grep '_alter\.sql$'); do
  echo "+ Alter: $file"

  # Read file content and clean it up for MySQL execution
  file_content=$(cat "$BASE_INIT_PATH/${file}" | \
    sed '/^[[:space:]]*--/d' | \
    sed '/^[[:space:]]*$/d' | \
    tr '\n' ' ' | \
    sed 's/[[:space:]]\+/ /g' | \
    sed 's/;[[:space:]]*;/;/g' | \
    sed "s/'/''/g")

  # Execute the file content through the compatibility system
  mysql \
    -h "$MYSQL_HOST" \
    -u "$MYSQL_USER" \
    -D "$MYSQL_DATABASE" \
    -e "CALL CozeLoopExecuteAlterFile('${file_content}');"
done

print_banner "Completed!"
