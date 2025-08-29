// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
)

//go:generate mockgen -destination=mocks/Task_dao.go -package=mocks . ITaskDAO
type ITaskDAO interface {
	MSet(ctx context.Context, Tasks []*entity.ObservabilityTask) error
	MGet(ctx context.Context, queries []TaskQuery) (TaskMap map[TaskQuery]*entity.ObservabilityTask, err error)
}

type TaskQuery struct {
	TaskID int64

	WithCommit    bool
	CommitVersion string
}

type TaskDAOImpl struct{}

// NewTaskDAO noop impl
func NewTaskDAO() ITaskDAO {
	return &TaskDAOImpl{}
}

func (p *TaskDAOImpl) MSet(ctx context.Context, Tasks []*entity.ObservabilityTask) error {
	return nil
}

func (p *TaskDAOImpl) MGet(ctx context.Context, queries []TaskQuery) (TaskMap map[TaskQuery]*entity.ObservabilityTask, err error) {
	return nil, nil
}
