// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// SignatureInput describes the exact POST to send; URL and RawBody are not rewritten.
type SignatureInput struct {
	Method      string
	URL         *url.URL
	UnixSeconds int64
	Nonce       string
	OperationID string
	RawBody     []byte
}

// SignV1 is not authorization: keyID/key must come from a trusted endpoint-bound
// resolver. The caller supplies a fresh nonce/time per attempt, not per operation.
func SignV1(input SignatureInput, keyID string, key []byte) (http.Header, error) {
	if len(key) == 0 || !signatureToken(keyID) {
		return nil, errors.New("invalid hook signing key or key ID")
	}
	message, err := signatureMessage(input)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	headers := make(http.Header, 5)
	headers.Set("X-Fornax-Hook-Signature-Version", "v1")
	headers.Set("X-Fornax-Hook-Key-Id", keyID)
	headers.Set("X-Fornax-Hook-Timestamp", strconv.FormatInt(input.UnixSeconds, 10))
	headers.Set("X-Fornax-Hook-Nonce", input.Nonce)
	headers.Set("X-Fornax-Hook-Signature", hex.EncodeToString(mac.Sum(nil)))
	return headers, nil
}

func signatureMessage(input SignatureInput) (string, error) {
	if input.Method != http.MethodPost || input.UnixSeconds <= 0 || !signatureToken(input.Nonce) ||
		!signatureToken(input.OperationID) || len(input.OperationID) > 128 || len(input.RawBody) > 64*1024 {
		return "", errors.New("invalid hook signature input")
	}
	if _, _, err := normalizedEndpointURL(input.URL); err != nil {
		return "", err
	}
	target := input.URL.EscapedPath()
	if target == "" {
		target = "/"
	}
	if input.URL.ForceQuery || input.URL.RawQuery != "" {
		target += "?" + input.URL.RawQuery
	}
	bodyHash := sha256.Sum256(input.RawBody)
	return strings.Join([]string{
		http.MethodPost, target, strconv.FormatInt(input.UnixSeconds, 10), input.Nonce,
		input.OperationID, hex.EncodeToString(bodyHash[:]),
	}, "\n"), nil
}

// Signature tokens cannot contain whitespace or control bytes at header/line boundaries.
func signatureToken(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	for _, c := range value {
		if c <= ' ' || c >= 0x7f {
			return false
		}
	}
	return true
}
