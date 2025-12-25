#!/bin/bash

# 设置目标目录（如果没有提供参数，则使用当前目录）
TARGET_DIR=${1:-.}

# license 头内容
LICENSE_HEADER="// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0"

# 函数：检查文件是否已有 license 头
has_license_header() {
    grep -q "Copyright (c) 2025 coze-dev Authors" "$1"
}

add_header_in_dir() {
    # 遍历目标目录中的所有 .ts/.tsx/.less/.css 文件
    find "$1" \( -name "*.ts" -o -name "*.tsx" -o -name "*.less" -o -name "*.css" \) -print0 | while read -r -d $'\0' file; do
        if ! has_license_header "$file"; then
            echo "Adding license header to $file"
            temp_file=$(mktemp)
            echo "$LICENSE_HEADER" | cat - "$file" > "$temp_file"
            mv "$temp_file" "$file"
        else
            echo "Skipping $file (license header already exists)"
        fi
    done
}

add_header_in_dir "./frontend/apps/cozeloop"
add_header_in_dir "./frontend/packages/cozeloop"
