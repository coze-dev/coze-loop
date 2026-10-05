// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type ExptTemplateScheduleState struct {
	Template *entity.ExptTemplate
	Config   *entity.HookConfigRecord
	Binding  *entity.ExptTemplateScheduleBinding
	Revision string
}

// ExptTemplateScheduleWrite is internal, after management authorization and validation.
type ExptTemplateScheduleWrite struct {
	Key              entity.ExptTemplateScheduleBindingKey
	Create           bool
	ExpectedRevision string
	Template         *entity.ExptTemplate
	Refs             []*entity.ExptTemplateEvaluatorRef
	Fields           map[string]any
	Hook             entity.HookConfigUpdateInput
	Binding          *entity.ExptTemplateScheduleBinding
	Disable          bool
}

type IExptTemplateScheduleStore interface {
	Read(context.Context, entity.ExptTemplateScheduleBindingKey) (*ExptTemplateScheduleState, error)
	Write(context.Context, ExptTemplateScheduleWrite) error
}
