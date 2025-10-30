// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tracecommon "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	genquery "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/query"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

// 默认限制条数
const (
	DefaultTaskRunLimit  = 20
	MaxTaskRunLimit      = 501
	DefaultTaskRunOffset = 0
)

type ListTaskRunParam struct {
	WorkspaceID   *int64
	TaskID        *int64
	TaskRunStatus *task.RunStatus
	ReqLimit      int32
	ReqOffset     int32
	OrderBy       *tracecommon.OrderBy
}

//go:generate mockgen -destination=mocks/task_run.go -package=mocks . ITaskRunDao
type ITaskRunDao interface {
	// 基础CRUD操作
	GetBackfillTaskRun(ctx context.Context, workspaceID *int64, taskID int64) (*model.ObservabilityTaskRun, error)
	GetLatestNewDataTaskRun(ctx context.Context, workspaceID *int64, taskID int64) (*model.ObservabilityTaskRun, error)
	CreateTaskRun(ctx context.Context, po *model.ObservabilityTaskRun) (int64, error)
	UpdateTaskRun(ctx context.Context, po *model.ObservabilityTaskRun) error
	ListTaskRuns(ctx context.Context, param ListTaskRunParam) ([]*model.ObservabilityTaskRun, int64, error)
	UpdateTaskRunWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error
}

func NewTaskRunDaoImpl(db db.Provider) ITaskRunDao {
	return &TaskRunDaoImpl{
		dbMgr: db,
	}
}

type TaskRunDaoImpl struct {
	dbMgr db.Provider
}

// TaskRun非终态状态定义
var NonFinalTaskRunStatuses = []string{
	"pending",  // 等待执行
	"running",  // 执行中
	"paused",   // 暂停
	"retrying", // 重试中
}

// 活跃状态定义（非终态状态的子集）
var ActiveTaskRunStatuses = []string{
	"running",  // 执行中
	"retrying", // 重试中
}

// 计算分页参数
func calculateTaskRunPagination(reqLimit, reqOffset int32) (int, int) {
	limit := DefaultTaskRunLimit
	if reqLimit > 0 && reqLimit < MaxTaskRunLimit {
		limit = int(reqLimit)
	}

	offset := DefaultTaskRunOffset
	if reqOffset > 0 {
		offset = int(reqOffset)
	}

	return limit, offset
}

func (v *TaskRunDaoImpl) GetBackfillTaskRun(ctx context.Context, workspaceID *int64, taskID int64) (*model.ObservabilityTaskRun, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.TaskType.Eq(task.TaskRunTypeBackFill)).Where(q.TaskID.Eq(taskID))

	if workspaceID != nil {
		qd = qd.Where(q.WorkspaceID.Eq(*workspaceID))
	}
	taskRunPo, err := qd.First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		} else {
			return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
	}
	return taskRunPo, nil
}

func (v *TaskRunDaoImpl) GetLatestNewDataTaskRun(ctx context.Context, workspaceID *int64, taskID int64) (*model.ObservabilityTaskRun, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.TaskType.Eq(task.TaskRunTypeNewData)).Where(q.TaskID.Eq(taskID))

	if workspaceID != nil {
		qd = qd.Where(q.WorkspaceID.Eq(*workspaceID))
	}
	taskRunPo, err := qd.Order(q.CreatedAt.Desc()).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
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
	orderField := ""
	orderAsc := false
	if param.OrderBy != nil {
		orderField = param.OrderBy.Field
		orderAsc = param.OrderBy.IsAsc
	}
	qd = qd.Order(v.order(q, orderField, orderAsc))

	// 计算总数
	total, err := qd.Count()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}

	// 计算分页参数
	limit, offset := calculateTaskRunPagination(param.ReqLimit, param.ReqOffset)
	results, err := qd.Limit(limit).Offset(offset).Find()
	if err != nil {
		return nil, total, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, total, nil
}

func (d *TaskRunDaoImpl) order(q *genquery.Query, orderBy string, asc bool) field.Expr {
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

const (
	MaxRetries = 3
	RetryDelay = 100 * time.Millisecond
)

// UpdateTaskRunWithOCC 乐观并发控制更新
func (v *TaskRunDaoImpl) UpdateTaskRunWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.ID.Eq(id))
	if workspaceID != 0 {
		qd = qd.Where(q.WorkspaceID.Eq(workspaceID))
	}
	for i := 0; i < MaxRetries; i++ {
		// 使用原始 updated_at 作为乐观锁条件
		existingTaskRun, err := qd.First()
		if err != nil {
			return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
		updateMap["updated_at"] = time.Now()
		info, err := qd.Where(q.UpdatedAt.Eq(existingTaskRun.UpdatedAt)).Updates(updateMap)
		if err != nil {
			return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
		logs.CtxInfo(ctx, "TaskRun updated with OCC, id:%d, workspaceID:%d, rowsAffected:%d", id, workspaceID, info.RowsAffected)
		if info.RowsAffected == 1 {
			return nil
		}
		time.Sleep(RetryDelay)
	}
	return errorx.NewByCode(obErrorx.CommonMySqlErrorCode, errorx.WithExtraMsg("TaskRun update failed with OCC"))
}
