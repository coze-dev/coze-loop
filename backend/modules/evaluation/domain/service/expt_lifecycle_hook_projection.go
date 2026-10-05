// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func (e *HookAttemptExecutor) projectCompletion(ctx context.Context, in hookcomponent.AttemptExecutionInput, result entity.HookTransportResult) ([]byte, string, string, error) {
	bad := func() ([]byte, string, string, error) { return nil, "", "", ErrHookExecutionUnavailable }
	r := result.Response
	if result.Outcome.HTTPStatus != 200 || r == nil {
		return bad()
	}
	switch result.Outcome.Code {
	case entity.HookSucceeded:
		if r.GetStatus() != spi.HookResultStatusSucceeded || r.Error != nil || result.Outcome.Retryable {
			return bad()
		}
		if _, err := entity.EncodeHookDisplayResult(r.Result_); err != nil {
			return bad()
		}
		projected, err := e.projector.Project(ctx, in.Key, in.Phase, r)
		if err != nil {
			return bad()
		}
		b, err := hookExecutionEncodeResult(projected)
		return b, "", "", err
	case entity.HookFailed:
		if r.GetStatus() != spi.HookResultStatusFailed || r.Result_ != nil {
			return bad()
		}
		original, err := entity.NormalizeHookDisplayError(r.Error)
		if err != nil || original.GetRetryable() != result.Outcome.Retryable {
			return bad()
		}
		projected, err := e.projector.ProjectError(ctx, in.Key, in.Phase, r)
		if err != nil {
			return bad()
		}
		projected, err = entity.NormalizeHookDisplayError(projected)
		if err != nil || projected.GetRetryable() != result.Outcome.Retryable {
			return bad()
		}
		return nil, projected.GetCode(), projected.GetMessage(), nil
	default:
		return bad()
	}
}

func hookExecutionEncodeResult(m map[string]string) ([]byte, error) {
	b, err := entity.EncodeHookDisplayResult(m)
	if err != nil {
		return nil, ErrHookExecutionUnavailable
	}
	return b, nil
}
