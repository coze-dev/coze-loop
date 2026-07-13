// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Sign computes the HMAC-SHA256 signature for a delivery attempt. Contract:
//
//	signature = "sha256=" + hex(HMAC_SHA256(secret, timestamp + "\n" + body))
//
// Timestamp is passed as the decimal Unix-seconds string so both sides can
// re-compute it deterministically from `X-Fornax-Timestamp`. Any byte change
// to `body` or `timestamp` invalidates the resulting hex.
func Sign(secret []byte, timestamp string, body []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(timestamp))
	h.Write([]byte("\n"))
	h.Write(body)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// SignWithAlgorithm dispatches by algorithm name; currently only `sha256`
// is supported (matches WebhookSecurityConf default).
func SignWithAlgorithm(algorithm string, secret []byte, timestamp string, body []byte) (string, error) {
	switch algorithm {
	case "", "sha256":
		return Sign(secret, timestamp, body), nil
	default:
		return "", fmt.Errorf("unsupported webhook signature algorithm: %s", algorithm)
	}
}
