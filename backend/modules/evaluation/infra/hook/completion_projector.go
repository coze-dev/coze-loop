// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

var errCompletionProjection = errors.New("hook display projection unavailable")

type completionProjector struct{}

// NewCompletionProjector selects the approved receiver-owned display contract.
// It checks structure only; it neither detects PII nor adds platform identity data.
func NewCompletionProjector() hookcomponent.CompletionProjector { return completionProjector{} }

func (completionProjector) Project(ctx context.Context, key entity.HookRunKey, phase entity.HookPhase, in *spi.InvokeExperimentHookResponse) (map[string]string, error) {
	if !completionProjectionScope(ctx, key, phase) || in == nil || in.GetStatus() != spi.HookResultStatusSucceeded || in.Error != nil {
		return nil, errCompletionProjection
	}
	if _, err := entity.EncodeHookDisplayResult(in.Result_); err != nil {
		return nil, errCompletionProjection
	}
	out := make(map[string]string, len(in.Result_))
	for k, v := range in.Result_ {
		out[k] = v
	}
	return out, nil
}

func (completionProjector) ProjectError(ctx context.Context, key entity.HookRunKey, phase entity.HookPhase, in *spi.InvokeExperimentHookResponse) (*spi.HookError, error) {
	if !completionProjectionScope(ctx, key, phase) || in == nil || in.GetStatus() != spi.HookResultStatusFailed || in.Result_ != nil {
		return nil, errCompletionProjection
	}
	out, err := entity.NormalizeHookDisplayError(in.Error)
	if err != nil {
		return nil, errCompletionProjection
	}
	return out, nil
}

func completionProjectionScope(ctx context.Context, key entity.HookRunKey, phase entity.HookPhase) bool {
	return ctx != nil && ctx.Err() == nil && key.WorkspaceID > 0 && key.ExperimentID > 0 && key.RunID > 0 && (phase == entity.HookPhaseBefore || phase == entity.HookPhaseAfter)
}
