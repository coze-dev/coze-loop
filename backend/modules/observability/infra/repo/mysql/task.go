// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	genquery "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/query"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"gorm.io/gorm"
)

// 默认限制条数
const (
	DefaultLimit  = 20
	MaxLimit      = 501
	DefaultOffset = 0
)

type ListTaskParam struct {
	WorkspaceIDs []int64
	TaskFilters  *filter.TaskFilterFields
	ReqLimit     int32
	ReqOffset    int32
	OrderBy      task.OrderType
}

//go:generate mockgen -destination=mocks/task.go -package=mocks . ITaskDao
type ITaskDao interface {
	GetTask(ctx context.Context, id int64, workspaceID *int64, userID *string) (*model.ObservabilityTask, error)
	ListTasks(ctx context.Context, workspaceID int64, userID string) ([]*model.ObservabilityTask, error)
	CreateTask(ctx context.Context, po *model.ObservabilityTask) (int64, error)
	UpdateTask(ctx context.Context, po *model.ObservabilityTask) error
	DeleteTask(ctx context.Context, id int64, workspaceID int64, userID string) error
	ListTask(ctx context.Context, param ListTaskParam) ([]*model.ObservabilityTask, int64, error)
}

func NewTaskDaoImpl(db db.Provider) ITaskDao {
	return &TaskDaoImpl{
		dbMgr: db,
	}
}

type TaskDaoImpl struct {
	dbMgr db.Provider
}

func (v *TaskDaoImpl) GetTask(ctx context.Context, id int64, workspaceID *int64, userID *string) (*model.ObservabilityTask, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTask
	qd := q.WithContext(ctx).Where(q.ID.Eq(id))
	if workspaceID != nil {
		qd = qd.Where(q.WorkspaceID.Eq(*workspaceID))
	}
	if userID != nil {
		qd = qd.Where(q.CreatedBy.Eq(*userID))
	}
	TaskPo, err := qd.First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("Task not found"))
		} else {
			return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
	}
	return TaskPo, nil
}

func (v *TaskDaoImpl) ListTasks(ctx context.Context, workspaceID int64, userID string) ([]*model.ObservabilityTask, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTask
	qd := q.WithContext(ctx)
	if workspaceID != 0 {
		qd = qd.Where(q.WorkspaceID.Eq(workspaceID))
	}
	if userID != "" {
		qd = qd.Where(q.CreatedBy.Eq(userID))
	}
	results, err := qd.Limit(100).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, nil
}

func (v *TaskDaoImpl) CreateTask(ctx context.Context, po *model.ObservabilityTask) (int64, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTask
	if err := q.WithContext(ctx).Create(po); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return 0, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("Task duplicate key"))
		} else {
			return 0, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
	} else {
		return po.ID, nil
	}
}

func (v *TaskDaoImpl) UpdateTask(ctx context.Context, po *model.ObservabilityTask) error {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTask
	if err := q.WithContext(ctx).Save(po); err != nil {
		return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	} else {
		return nil
	}
}

func (v *TaskDaoImpl) DeleteTask(ctx context.Context, id int64, workspaceID int64, userID string) error {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTask
	qd := q.WithContext(ctx).Where(q.ID.Eq(id)).Where(q.WorkspaceID.Eq(workspaceID)).Where(q.CreatedBy.Eq(userID))
	info, err := qd.Delete()
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	logs.CtxInfo(ctx, "%d rows deleted", info.RowsAffected)
	return nil
}

func (v *TaskDaoImpl) ListTask(ctx context.Context, param ListTaskParam) ([]*model.ObservabilityTask, int64, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTask
	var total int64
	qd := q.WithContext(ctx)
	if len(param.WorkspaceIDs) != 0 {
		qd = qd.Where(q.WorkspaceID.In(param.WorkspaceIDs...))
	}
	// 应用过滤条件
	//var err error
	//qd, err = applyTaskFilters(qd, param.TaskFilters)
	//if err != nil {
	//	return nil, 0, err
	//}

	// 计算分页参数
	limit, offset := calculatePagination(param.ReqLimit, param.ReqOffset)
	if param.OrderBy == task.OrderType_Asc {
		qd = qd.Order(q.CreatedAt.Asc())
	} else {
		qd = qd.Order(q.CreatedAt.Desc())
	}
	results, err := qd.Limit(limit).Offset(offset).Find()
	if err != nil {
		return nil, total, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, total, nil
}

// 处理任务过滤条件
//func applyTaskFilters(db *query.observabilityTaskDo, taskFilters *filter.TaskFilterFields) (*query.observabilityTaskDo, error) {
//	if taskFilters == nil {
//		return db, nil
//	}
//
//	return db, nil
//}

// 计算分页参数
func calculatePagination(reqLimit, reqOffset int32) (int, int) {
	limit := DefaultLimit
	if reqLimit > 0 && reqLimit < MaxLimit {
		limit = int(reqLimit)
	}

	offset := DefaultOffset
	if reqOffset > 0 {
		offset = int(reqOffset)
	}

	return limit, offset
}
