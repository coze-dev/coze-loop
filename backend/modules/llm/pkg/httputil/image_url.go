// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package httputil

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func IsFullUrl(url string) bool {
	return strings.Index(url, "http://") == 0 || strings.Index(url, "https://") == 0
}

func ImageURLToBase64(url string) (base64Str string, mimeType string, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("获取图片失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP请求失败: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取响应失败: %w", err)
	}
	mimeType = http.DetectContentType(body)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(body)), mimeType, nil
}
