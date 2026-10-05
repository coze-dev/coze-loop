// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

var rfc3339Shape = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]([01]\d|2[0-3]):[0-5]\d)$`)

// BuildRequest returns the single byte sequence to sign and send. It does not
// authorize identities, allocate delivery IDs or mutate the supplied snapshot.
func BuildRequest(request *spi.InvokeExperimentHookRequest) ([]byte, error) {
	if err := validateRequest(request); err != nil {
		return nil, err
	}
	parameters, err := entity.NormalizeHookParameters(request.GetParameters())
	if err != nil {
		return nil, errors.New("invalid hook request parameters")
	}
	// Override only HTTP representations that differ from generated Thrift JSON.
	context := struct {
		*spi.HookRunContext
		EvalSets []*spi.HookEvalSetRef `json:"eval_sets"`
	}{request.Context, request.Context.EvalSets}
	payload := struct {
		*spi.InvokeExperimentHookRequest
		Context    any             `json:"context"`
		Parameters json.RawMessage `json:"parameters"`
	}{request, context, json.RawMessage(parameters)}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("cannot encode hook request")
	}
	if len(body) > 64*1024 {
		return nil, errors.New("hook request exceeds 64 KiB")
	}
	return body, nil
}

func validateRequest(r *spi.InvokeExperimentHookRequest) error {
	invalid := errors.New("invalid hook request protocol fields")
	if r == nil || r.GetSchemaVersion() != "1.0" || !validText(r.OperationID, 128, true) || !validText(r.IdempotencyKey, 256, true) || !validText(r.DeliveryID, 128, true) || r.GetAttempt() < 1 || r.GetAttempt() > 11 {
		return invalid
	}
	if r.GetEventType() != spi.HookEventTypeBefore && r.GetEventType() != spi.HookEventTypeAfter {
		return invalid
	}
	if !rfc3339Shape.MatchString(r.GetOccurredAt()) {
		return invalid
	}
	if _, err := time.Parse(time.RFC3339, r.GetOccurredAt()); err != nil {
		return invalid
	}
	phase := entity.HookPhaseBefore
	if r.GetEventType() == spi.HookEventTypeAfter {
		phase = entity.HookPhaseAfter
	}
	return entity.ValidateHookRunContext(r.Context, phase)
}

func validText(value *string, max int, required bool) bool {
	if value == nil {
		return !required
	}
	return utf8.ValidString(*value) && (!required || strings.TrimSpace(*value) != "") && (max == 0 || len(*value) <= max)
}
