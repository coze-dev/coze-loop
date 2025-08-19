#!/bin/sh

exec 2>&1
set -e

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

CONFIG_FILE="/etc/clickhouse-server/config.xml"

cat > "$CONFIG_FILE" <<EOF
<yandex>
    <listen_host>0.0.0.0</listen_host>

    <tcp_port>9000</tcp_port>

    <path>/var/lib/clickhouse/</path>
    <tmp_path>/var/lib/clickhouse/tmp/</tmp_path>
    <user_files_path>/var/lib/clickhouse/user_files/</user_files_path>
    <format_schema_path>/var/lib/clickhouse/format_schemas/</format_schema_path>

    <logger>
        <log>/var/log/clickhouse-server/clickhouse-server.log</log>
        <errorlog>/var/log/clickhouse-server/clickhouse-server.err.log</errorlog>
        <level>trace</level>
    </logger>

    <profiles>
        <default>
            <max_memory_usage>10000000000</max_memory_usage>
            <use_uncompressed_cache>1</use_uncompressed_cache>
            <load_balancing>random</load_balancing>
            <max_threads>4</max_threads>
        </default>
    </profiles>

    <users>
        <${COZE_LOOP_CLICKHOUSE_USER}>
            <password>${COZE_LOOP_CLICKHOUSE_PASSWORD}</password>
            <networks>
                <ip>::/0</ip>
                <ip>0.0.0.0/0</ip>
            </networks>
            <profile>default</profile>
            <quota>default</quota>
        </${COZE_LOOP_CLICKHOUSE_USER}>
    </users>
</yandex>
EOF

(
  while true; do
    if sh /coze-loop-clickhouse/bootstrap/healthcheck.sh; then
      print_banner "Completed!"
      break
    else
      sleep 1
    fi
  done
)&

exec clickhouse-server --config="${CONFIG_FILE}"