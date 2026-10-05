// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
)

var hookDisplayJSON = sonic.Config{SortMapKeys: true}.Froze()

// EncodeHookDisplayResult validates display data, not its business meaning.
// Reserve the complete SPI envelope; do not expand HTML or Unicode separators.
func EncodeHookDisplayResult(result map[string]string) ([]byte, error) {
	if len(result) > 128 {
		return nil, invalidParam("invalid hook display result")
	}
	for k, v := range result {
		if len(k) == 0 || len(k) > 128 || len(v) > 8192 || !utf8.ValidString(k) || !utf8.ValidString(v) {
			return nil, invalidParam("invalid hook display result")
		}
	}
	if result == nil {
		result = map[string]string{}
	}
	b, err := hookDisplayJSON.Marshal(result)
	if err != nil || len(b)+len(`{"status":"succeeded","result":}`) > 32768 {
		return nil, invalidParam("invalid hook display response size")
	}
	return b, nil
}

// NormalizeHookDisplayError copies protocol fields; callers retain the platform outcome.
func NormalizeHookDisplayError(in *spi.HookError) (*spi.HookError, error) {
	if in == nil || in.Message == nil {
		return nil, invalidParam("invalid hook display error")
	}
	code, message, retryable := string(HookFailed), in.GetMessage(), in.GetRetryable()
	if in.Code != nil {
		code = in.GetCode()
	}
	if !utf8.ValidString(code) || !utf8.ValidString(message) || strings.TrimSpace(code) == "" || strings.TrimSpace(message) == "" || len(code) > 128 || len(message) > 2048 {
		return nil, invalidParam("invalid hook display error")
	}
	return &spi.HookError{Code: &code, Message: &message, Retryable: &retryable}, nil
}
