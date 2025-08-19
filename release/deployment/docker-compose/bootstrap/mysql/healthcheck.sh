#!/bin/sh

set -e

export MYSQL_PWD="${COZE_LOOP_MYSQL_PASSWORD}"

if mysql -h 127.0.0.1 --protocol=TCP \
      -u "${COZE_LOOP_MYSQL_USER}" \
      --silent --skip-column-names \
      -e "SELECT 1 FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME='${COZE_LOOP_MYSQL_DATABASE}'" \
      2>/dev/null \
      | grep -q 1; then
  exit 0
else
  exit 1
fi