// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// DecodeResponse owns neither the reader nor any transport lifetime. Non-200
// bodies are bounded diagnostics only and cannot change HTTP retry semantics.
func DecodeResponse(status int, contentType string, body io.Reader) (*spi.InvokeExperimentHookResponse, entity.HookOutcome) {
	classify := func(code entity.HookOutcomeCode) (*spi.InvokeExperimentHookResponse, entity.HookOutcome) {
		return nil, entity.ClassifyHookOutcome(code, status, "", false)
	}
	if status != 200 {
		if body != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(body, 32*1024))
		}
		return classify("")
	}
	if body == nil {
		return classify(entity.HookProtocolError)
	}
	data, err := io.ReadAll(io.LimitReader(body, 32*1024+1))
	if len(data) > 32*1024 {
		return classify(entity.HookResponseTooLarge)
	}
	if err != nil {
		return classify(entity.HookTransportError)
	}
	media, _, err := mime.ParseMediaType(contentType)
	if err != nil || media != "application/json" || !validJSONObject(data) {
		return classify(entity.HookProtocolError)
	}
	var raw map[string]json.RawMessage
	var responseStatus string
	if json.Unmarshal(data, &raw) != nil || json.Unmarshal(raw["status"], &responseStatus) != nil {
		return classify(entity.HookProtocolError)
	}
	response := spi.InvokeExperimentHookResponse{Status: &responseStatus}
	switch response.GetStatus() {
	case spi.HookResultStatusSucceeded:
		if !absentOrNull(raw["error"]) {
			return classify(entity.HookProtocolError)
		}
		response.Result_ = map[string]string{}
		var values map[string]json.RawMessage
		if !absentOrNull(raw["result"]) {
			if json.Unmarshal(raw["result"], &values) != nil {
				return classify(entity.HookProtocolError)
			}
			if len(values) > 128 {
				return classify(entity.HookProtocolError)
			}
			for key, value := range values {
				// encoding/json otherwise turns map string values of null into "".
				var text string
				if absentOrNull(value) || json.Unmarshal(value, &text) != nil || len(key) == 0 || len(key) > 128 || len(text) > 8*1024 {
					return classify(entity.HookProtocolError)
				}
				response.Result_[key] = text
			}
		}
	case spi.HookResultStatusFailed:
		var valid bool
		response.Error, valid = decodeHookError(raw["error"])
		if !absentOrNull(raw["result"]) || !valid {
			return classify(entity.HookProtocolError)
		}
	default:
		return classify(entity.HookProtocolError)
	}
	return &response, entity.ClassifyHookOutcome("", status, response.GetStatus(), response.Error.GetRetryable())
}

func decodeHookError(data json.RawMessage) (*spi.HookError, bool) {
	var fields map[string]json.RawMessage
	if absentOrNull(data) || json.Unmarshal(data, &fields) != nil {
		return nil, false
	}
	var message string
	if json.Unmarshal(fields["message"], &message) != nil || !validText(&message, 2048, true) {
		return nil, false
	}
	code := "HOOK_FAILED"
	if !absentOrNull(fields["code"]) {
		if json.Unmarshal(fields["code"], &code) != nil || !validText(&code, 128, true) {
			return nil, false
		}
	}
	retryable := false
	if !absentOrNull(fields["retryable"]) && json.Unmarshal(fields["retryable"], &retryable) != nil {
		return nil, false
	}
	return &spi.HookError{Code: &code, Message: &message, Retryable: &retryable}, true
}

func absentOrNull(value json.RawMessage) bool {
	return len(value) == 0 || bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}
