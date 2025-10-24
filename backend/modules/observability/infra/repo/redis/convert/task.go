// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/samber/lo"
)

func NewTaskConverter() *TaskConverter {
	return &TaskConverter{}
}

type TaskConverter struct{}

func (TaskConverter) FromDO(qse *entity.ObservabilityTask) ([]byte, error) {
	bytes, err := json.Marshal(qse)
	if err != nil {
		return nil, errorx.Wrapf(err, "json marshal failed")
	}
	return bytes, nil
}

func (TaskConverter) ToDO(b []byte) (*entity.ObservabilityTask, error) {
	qse := &entity.ObservabilityTask{}
	if err := lo.TernaryF(
		len(b) > 0,
		func() error { return json.Unmarshal(b, qse) },
		func() error { return nil },
	); err != nil {
		return nil, errorx.Wrapf(err, "TaskExpt json unmarshal failed")
	}
	return qse, nil
}
