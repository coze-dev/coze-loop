// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package signer 提供 webhook 投递 HMAC-SHA256 签名工具。
// 契约（tech_design 已锁）：
//   - canonical string = timestamp + "\n" + body（body 为 canonical JSON，签名前已去除多余空格）
//   - 签名值 = HMAC-SHA256(secret, canonical) 的 hex 小写
//   - 输出 header 值格式统一为 "sha256=<hex>"
//   - secret 为空 → 返回 "sha256="（不抛错，走 test_case 22 降级路径）
package signer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignatureHeaderPrefix 是 X-Fornax-Signature header value 的固定前缀。
const SignatureHeaderPrefix = "sha256="

// Sign 计算 webhook 投递签名，返回可直接写入 X-Fornax-Signature header 的 value。
//
// canonical string 拼接规则严格对齐 tech_design：`timestamp + "\n" + body`。
// timestamp 为 Unix 秒 10 位数字字符串；body 为投递前已 canonicalize 的 JSON 字节。
// secret 为空 → 返回 "sha256="（对齐 test_case 22 OSS 缺 secret 时降级）。
func Sign(secret string, timestamp string, body []byte) string {
	if secret == "" {
		return SignatureHeaderPrefix
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return SignatureHeaderPrefix + hex.EncodeToString(mac.Sum(nil))
}
