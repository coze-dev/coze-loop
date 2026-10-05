// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// HookConfigCreateInput is supplied after destination authorization, never from stored ciphertext.
type HookConfigCreateInput struct {
	WorkspaceID    int64
	ExecutionScope string
	KeyID          string                    `json:"-"`
	Config         *entity.LifecycleHookConf `json:"-"`
}

// IHookConfigCreator is optional; existing config repositories need not implement it.
// Object IDs and ref IDs must be allocated before entry. Each method atomically inserts the object and refs.
type IHookConfigCreator interface {
	CreateExperimentWithHookConfig(context.Context, *entity.Experiment, []*entity.ExptEvaluatorRef, HookConfigCreateInput) error
	CreateTemplateWithHookConfig(context.Context, *entity.ExptTemplate, []*entity.ExptTemplateEvaluatorRef, HookConfigCreateInput) error
}
