// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/query"
	genquery "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/query"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

type ListTaskRunParam struct {
	WorkspaceID   *int64
	TaskID        *int64
	TaskRunStatus *task.RunStatus
	ReqLimit      int32
	ReqOffset     int32
	OrderBy       *common.OrderBy
}

//go:generate mockgen -destination=mocks/task_run.go -package=mocks . ITaskRunDao
type ITaskRunDao interface {
	GetTaskRun(ctx context.Context, id int64, workspaceID *int64, taskID *int64) (*model.ObservabilityTaskRun, error)
	CreateTaskRun(ctx context.Context, po *model.ObservabilityTaskRun) (int64, error)
	UpdateTaskRun(ctx context.Context, po *model.ObservabilityTaskRun) error
	DeleteTaskRun(ctx context.Context, id int64, workspaceID int64, userID string) error
	ListTaskRuns(ctx context.Context, param ListTaskRunParam) ([]*model.ObservabilityTaskRun, int64, error)
}

func NewTaskRunDaoImpl(db db.Provider) ITaskRunDao {
	return &TaskRunDaoImpl{
		dbMgr: db,
	}
}

type TaskRunDaoImpl struct {
	dbMgr db.Provider
}

func (v *TaskRunDaoImpl) GetTaskRun(ctx context.Context, id int64, workspaceID *int64, taskID *int64) (*model.ObservabilityTaskRun, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.ID.Eq(id))
	if workspaceID != nil {
		qd = qd.Where(q.WorkspaceID.Eq(*workspaceID))
	}
	if taskID != nil {
		qd = qd.Where(q.TaskID.Eq(*taskID))
	}
	taskRunPo, err := qd.First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("TaskRun not found"))
		} else {
			return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
	}
	return taskRunPo, nil
}

func (v *TaskRunDaoImpl) CreateTaskRun(ctx context.Context, po *model.ObservabilityTaskRun) (int64, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	if err := q.WithContext(ctx).Create(po); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return 0, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("TaskRun duplicate key"))
		} else {
			return 0, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
	} else {
		return po.ID, nil
	}
}

func (v *TaskRunDaoImpl) UpdateTaskRun(ctx context.Context, po *model.ObservabilityTaskRun) error {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	if err := q.WithContext(ctx).Save(po); err != nil {
		return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	} else {
		return nil
	}
}

func (v *TaskRunDaoImpl) DeleteTaskRun(ctx context.Context, id int64, workspaceID int64, userID string) error {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	// 注意：TaskRun模型中没有CreatedBy字段，只能按ID和WorkspaceID过滤
	qd := q.WithContext(ctx).Where(q.ID.Eq(id)).Where(q.WorkspaceID.Eq(workspaceID))
	// userID参数暂时忽略，因为TaskRun模型中没有CreatedBy字段
	info, err := qd.Delete()
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	logs.CtxInfo(ctx, "%d rows deleted", info.RowsAffected)
	return nil
}

func (v *TaskRunDaoImpl) ListTaskRuns(ctx context.Context, param ListTaskRunParam) ([]*model.ObservabilityTaskRun, int64, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx))
	qd := q.WithContext(ctx).ObservabilityTaskRun
	var total int64

	// TaskID过滤
	if param.TaskID != nil {
		qd = qd.Where(q.ObservabilityTaskRun.TaskID.Eq(*param.TaskID))
	}
	// TaskRunStatus过滤
	if param.TaskRunStatus != nil {
		qd = qd.Where(q.ObservabilityTaskRun.RunStatus.Eq(*param.TaskRunStatus))
	}
	// workspaceID过滤
	if param.WorkspaceID != nil {
		qd = qd.Where(q.ObservabilityTaskRun.WorkspaceID.Eq(*param.WorkspaceID))
	}

	// 排序
	qd = qd.Order(v.order(q, param.OrderBy.GetField(), param.OrderBy.GetIsAsc()))

	// 计算总数
	total, err := qd.Count()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}

	// 计算分页参数
	limit, offset := calculatePagination(param.ReqLimit, param.ReqOffset)
	results, err := qd.Limit(limit).Offset(offset).Find()
	if err != nil {
		return nil, total, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, total, nil
}

func (d *TaskRunDaoImpl) order(q *query.Query, orderBy string, asc bool) field.Expr {
	var orderExpr field.OrderExpr
	switch orderBy {
	case "created_at":
		orderExpr = q.ObservabilityTaskRun.CreatedAt
	case "run_start_at":
		orderExpr = q.ObservabilityTaskRun.RunStartAt
	case "run_end_at":
		orderExpr = q.ObservabilityTaskRun.RunEndAt
	case "updated_at":
		orderExpr = q.ObservabilityTaskRun.UpdatedAt
	default:
		orderExpr = q.ObservabilityTaskRun.CreatedAt
	}
	if asc {
		return orderExpr.Asc()
	}
	return orderExpr.Desc()
}
