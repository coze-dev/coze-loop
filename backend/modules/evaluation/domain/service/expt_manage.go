// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate  mockgen -destination  ./mocks/expt_manage.go  --package mocks . IExptManager
type IExptManager interface {
	IExptConfigManager
	IExptExecutionManager
}

// IExptConfigManager 实验配置管理接口（负责实验元数据的增删改查）
type IExptConfigManager interface {
	CheckName(ctx context.Context, name string, spaceID int64, session *entity.Session) (bool, error)

	CreateExpt(ctx context.Context, req *entity.CreateExptParam, session *entity.Session) (*entity.Experiment, error)

	Update(ctx context.Context, expt *entity.Experiment, session *entity.Session) error
	Delete(ctx context.Context, exptID, spaceID int64, session *entity.Session) error
	MDelete(ctx context.Context, exptIDs []int64, spaceID int64, session *entity.Session) error

	List(ctx context.Context, page, pageSize int32, spaceID int64, filter *entity.ExptListFilter, orders []*entity.OrderBy, session *entity.Session) ([]*entity.Experiment, int64, error)
	ListExptRaw(ctx context.Context, page, pageSize int32, spaceID int64, filter *entity.ExptListFilter) ([]*entity.Experiment, int64, error)
	GetDetail(ctx context.Context, exptID, spaceID int64, session *entity.Session, opts ...entity.GetExptTupleOptionFn) (*entity.Experiment, error)
	MGetDetail(ctx context.Context, exptIDs []int64, spaceID int64, session *entity.Session) ([]*entity.Experiment, error)

	Get(ctx context.Context, exptID, spaceID int64, session *entity.Session) (*entity.Experiment, error)
	MGet(ctx context.Context, exptIDs []int64, spaceID int64, session *entity.Session) ([]*entity.Experiment, error)

	Clone(ctx context.Context, exptID, spaceID int64, session *entity.Session) (*entity.Experiment, error)
}

// IExptExecutionManager 实验执行控制接口（负责实验的运行、监控和状态管理）
type IExptExecutionManager interface {
	CheckRun(ctx context.Context, expt *entity.Experiment, spaceID int64, session *entity.Session, opts ...entity.ExptRunCheckOptionFn) error
	Run(ctx context.Context, exptID, runID, spaceID int64, session *entity.Session, runMode entity.ExptRunMode, ext map[string]string) error
	RetryUnSuccess(ctx context.Context, exptID, runID, spaceID int64, session *entity.Session, ext map[string]string) error

	Invoke(ctx context.Context, invokeExptReq *entity.InvokeExptReq) error
	Finish(ctx context.Context, exptID *entity.Experiment, exptRunID int64, session *entity.Session) error

	PendRun(ctx context.Context, exptID, exptRunID int64, spaceID int64, session *entity.Session) error
	PendExpt(ctx context.Context, exptID, spaceID int64, session *entity.Session, opts ...entity.CompleteExptOptionFn) error

	CompleteRun(ctx context.Context, exptID, exptRunID int64, mode entity.ExptRunMode, spaceID int64, session *entity.Session, opts ...entity.CompleteExptOptionFn) error
	CompleteExpt(ctx context.Context, exptID, spaceID int64, session *entity.Session, opts ...entity.CompleteExptOptionFn) error

	LogRun(ctx context.Context, exptID, exptRunID int64, mode entity.ExptRunMode, spaceID int64, session *entity.Session) error
	GetRunLog(ctx context.Context, exptID, exptRunID, spaceID int64, session *entity.Session) (*entity.ExptRunLog, error)
}
