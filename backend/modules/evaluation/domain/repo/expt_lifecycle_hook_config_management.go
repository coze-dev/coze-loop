// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"errors"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

var ErrHookConfigScheduleBindingRequired = errors.New("hook configuration update requires schedule identity binding")

const MaxHookConfigBatchSize = 100

// IHookConfigMetadataUpdater is consumed after the existing Manager validates the metadata.
type IHookConfigMetadataUpdater interface {
	UpdateExperimentWithHookConfig(context.Context, hookcomponent.ConfigOwner, *entity.Experiment, entity.HookConfigUpdateInput) error
	UpdateTemplateWithHookConfig(context.Context, hookcomponent.ConfigOwner, *entity.ExptTemplate, []*entity.ExptTemplateEvaluatorRef, entity.HookConfigUpdateInput) error
}

type HookConfigReadResult struct {
	Owner  hookcomponent.ConfigOwner
	Record *entity.HookConfigRecord
	Err    error `json:"-"`
}

// IHookConfigBatchReader requires authorized owners of one kind, workspace and scope.
// Per-item errors distinguish missing or corrupt objects from a valid nil configuration.
type IHookConfigBatchReader interface {
	MGetConfigs(context.Context, []hookcomponent.ConfigOwner) ([]HookConfigReadResult, error)
}
