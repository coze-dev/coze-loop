#!/bin/sh

set -e

export MYSQL_PWD="${COZE_LOOP_MYSQL_PASSWORD}"

mysql -h 127.0.0.1 --protocol=TCP \
      -u "${COZE_LOOP_MYSQL_USER}" \
      --silent --skip-column-names \
      -e "SELECT 1 FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME='${COZE_LOOP_MYSQL_DATABASE}'" \
      >/dev/null 2>&1