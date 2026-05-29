// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strconv"
	"time"
)

const (
	HeaderTimestamp = "X-CozeLoop-Timestamp"
	HeaderNonce     = "X-CozeLoop-Nonce"
	HeaderSignature = "X-CozeLoop-Signature"
)

// ComputeSignature 计算 HMAC-SHA256 签名
// message = timestamp + "\n" + nonce + "\n"
func ComputeSignature(secret, timestamp, nonce string) string {
	message := timestamp + "\n" + nonce + "\n"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateNonce 生成 16 字符密码学安全随机串
func GenerateNonce() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 16)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// GenerateTimestamp 生成当前 Unix 秒时间戳字符串
func GenerateTimestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// SignRequest 生成完整的签名 headers
func SignRequest(secret string) (timestamp, nonce, signature string) {
	timestamp = GenerateTimestamp()
	nonce = GenerateNonce()
	signature = ComputeSignature(secret, timestamp, nonce)
	return
}
