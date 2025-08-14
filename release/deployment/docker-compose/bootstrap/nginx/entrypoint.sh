#!/bin/bash

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

for i in $(seq 1 300); do
  if curl \
      -s http://coze-loop-app:8888/ping \
      2>/dev/null \
      | grep -q pong; then
    break
  else
    sleep 3
  fi
  if [ "$i" -eq 300 ]; then
    echo "[ERROR] Cozeloop app not available after 300 time."
    exit 1
  fi
done

mkdir -p /etc/nginx

if [ -z "${COZE_LOOP_OSS_PORT}" ]; then
    OSS_ENDPOINT="${COZE_LOOP_OSS_PROTOCOL}://${COZE_LOOP_OSS_DOMAIN}"
else
    OSS_ENDPOINT="${COZE_LOOP_OSS_PROTOCOL}://${COZE_LOOP_OSS_DOMAIN}:${COZE_LOOP_OSS_PORT}"
fi

cat > /etc/nginx/nginx.conf <<EOF
events {}

http {
    include       mime.types;
    default_type  application/octet-stream;

    sendfile        on;
    keepalive_timeout  65;

    server {
        listen       80;
        server_name  localhost;

        root /usr/share/nginx/html;
        index index.html;

        location = /index.html {
            etag on;
            add_header Cache-Control "no-cache, must-revalidate";
        }

        location / {
            try_files \$uri \$uri/ /index.html;
        }

        # app proxy
        location /api/ {
            proxy_pass         http://coze-loop-app:8888;
            proxy_http_version 1.1;

            proxy_set_header   Host \$host;
            proxy_set_header   X-Real-IP \$remote_addr;
            proxy_set_header   X-Forwarded-For \$proxy_add_x_forwarded_for;
            proxy_set_header   X-Forwarded-Proto \$scheme;

            add_header         Access-Control-Allow-Origin *;
            add_header         Access-Control-Allow-Methods "GET, POST, PUT, DELETE, OPTIONS";
            add_header         Access-Control-Allow-Headers "*";

            if (\$request_method = OPTIONS ) {
                add_header Access-Control-Max-Age 1728000;
                add_header Content-Type "text/plain charset=UTF-8";
                add_header Content-Length 0;
                return 204;
            }
        }

        # oss proxy
        location /${COZE_LOOP_OSS_BUCKET}/ {
            proxy_pass         ${OSS_ENDPOINT};
            proxy_http_version 1.1;

            client_max_body_size 1024m;

            proxy_set_header   X-Real-IP \$remote_addr;
            proxy_set_header   X-Forwarded-For \$proxy_add_x_forwarded_for;
            proxy_set_header   X-Forwarded-Proto \$scheme;

            proxy_set_header Connection "";
            chunked_transfer_encoding off;

            proxy_buffering off;
            proxy_request_buffering off;

            add_header         Access-Control-Allow-Origin *;
            add_header         Access-Control-Allow-Methods "GET, POST, PUT, DELETE, OPTIONS";
            add_header         Access-Control-Allow-Headers "*";

            if (\$request_method = OPTIONS ) {
                add_header Access-Control-Max-Age 1728000;
                add_header Content-Type "text/plain charset=UTF-8";
                add_header Content-Length 0;
                return 204;
            }
        }
    }
}
EOF

chmod 444 /etc/nginx/nginx.conf

(
  while true; do
    if sh /coze-loop-nginx/bootstrap/healthcheck.sh; then
      print_banner "Completed!"
      break
    else
      sleep 1
    fi
  done
)&

exec /docker-entrypoint.sh nginx -g 'daemon off;'