// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/samber/lo"
)

func NewTaskRunConverter() *TaskRunConverter {
	return &TaskRunConverter{}
}

type TaskRunConverter struct{}

func (TaskRunConverter) FromDO(taskRun *entity.TaskRun) ([]byte, error) {
	bytes, err := json.Marshal(taskRun)
	if err != nil {
		return nil, errorx.Wrapf(err, "TaskRun json marshal failed")
	}
	return bytes, nil
}

func (TaskRunConverter) ToDO(b []byte) (*entity.TaskRun, error) {
	taskRun := &entity.TaskRun{}
	bytes := toTaskRunBytes(b)
	if err := lo.TernaryF(
		len(bytes) > 0,
		func() error { return json.Unmarshal(bytes, taskRun) },
		func() error { return nil },
	); err != nil {
		return nil, errorx.Wrapf(err, "TaskRun json unmarshal failed")
	}
	return taskRun, nil
}

// toTaskRunBytes
//
//nolint:staticcheck
func toTaskRunBytes(v any) []byte {
	switch v.(type) {
	case string:
		return conv.UnsafeStringToBytes(v.(string))
	case []byte:
		return v.([]byte)
	default:
		return nil
	}
}
