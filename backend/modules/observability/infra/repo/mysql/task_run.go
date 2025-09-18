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
	OrderBy       *common.OrderBy
}

//go:generate mockgen -destination=mocks/task_run.go -package=mocks . ITaskRunDao
type ITaskRunDao interface {
	// 基础CRUD操作
	GetTaskRun(ctx context.Context, id int64, workspaceID *int64, taskID *int64) (*model.ObservabilityTaskRun, error)
	GetBackfillTaskRun(ctx context.Context, workspaceID *int64, taskID int64) (*model.ObservabilityTaskRun, error)
	GetLatestNewDataTaskRun(ctx context.Context, workspaceID *int64, taskID int64) (*model.ObservabilityTaskRun, error)
	CreateTaskRun(ctx context.Context, po *model.ObservabilityTaskRun) (int64, error)
	UpdateTaskRun(ctx context.Context, po *model.ObservabilityTaskRun) error
	DeleteTaskRun(ctx context.Context, id int64, workspaceID int64, userID string) error
	ListTaskRuns(ctx context.Context, param ListTaskRunParam) ([]*model.ObservabilityTaskRun, int64, error)

	// 业务特定方法
	ListNonFinalTaskRun(ctx context.Context) ([]*model.ObservabilityTaskRun, error)
	ListNonFinalTaskRunByTaskID(ctx context.Context, taskID int64) ([]*model.ObservabilityTaskRun, error)
	ListNonFinalTaskRunBySpaceID(ctx context.Context, spaceID int64) ([]*model.ObservabilityTaskRun, error)
	UpdateTaskRunWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error
	GetObjListWithTaskRun(ctx context.Context) ([]string, []string, error)
	ListActiveTaskRunsByTask(ctx context.Context, taskID int64) ([]*model.ObservabilityTaskRun, error)
	GetLatestTaskRunByTask(ctx context.Context, taskID int64) (*model.ObservabilityTaskRun, error)
	ListTaskRunsByStatus(ctx context.Context, status string) ([]*model.ObservabilityTaskRun, error)
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
	limit, offset := calculateTaskRunPagination(param.ReqLimit, param.ReqOffset)
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

// ListNonFinalTaskRun 获取非终态TaskRun列表
func (v *TaskRunDaoImpl) ListNonFinalTaskRun(ctx context.Context) ([]*model.ObservabilityTaskRun, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.RunStatus.In(NonFinalTaskRunStatuses...))

	results, err := qd.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, nil
}

// ListNonFinalTaskRunByTaskID 按TaskID获取非终态TaskRun
func (v *TaskRunDaoImpl) ListNonFinalTaskRunByTaskID(ctx context.Context, taskID int64) ([]*model.ObservabilityTaskRun, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.TaskID.Eq(taskID)).Where(q.RunStatus.In(NonFinalTaskRunStatuses...))

	results, err := qd.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, nil
}

// ListNonFinalTaskRunBySpaceID 按空间ID获取非终态TaskRun
func (v *TaskRunDaoImpl) ListNonFinalTaskRunBySpaceID(ctx context.Context, spaceID int64) ([]*model.ObservabilityTaskRun, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.WorkspaceID.Eq(spaceID)).Where(q.RunStatus.In(NonFinalTaskRunStatuses...))

	results, err := qd.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, nil
}

// UpdateTaskRunWithOCC 乐观并发控制更新
func (v *TaskRunDaoImpl) UpdateTaskRunWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.ID.Eq(id))
	if workspaceID != 0 {
		qd = qd.Where(q.WorkspaceID.Eq(workspaceID))
	}

	// 执行更新操作
	info, err := qd.Updates(updateMap)
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}

	logs.CtxInfo(ctx, "TaskRun updated with OCC", "id", id, "workspaceID", workspaceID, "rowsAffected", info.RowsAffected)
	return nil
}

// GetObjListWithTaskRun 获取有TaskRun的对象列表
func (v *TaskRunDaoImpl) GetObjListWithTaskRun(ctx context.Context) ([]string, []string, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun

	// 获取不重复的WorkspaceID列表
	var spaceList []string
	err := q.WithContext(ctx).Select(q.WorkspaceID).Distinct().Scan(&spaceList)
	if err != nil {
		return nil, nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}

	// TaskRun表中没有bot相关字段，返回空的bot列表
	var botList []string

	return spaceList, botList, nil
}

// ListActiveTaskRunsByTask 获取Task的活跃TaskRun列表
func (v *TaskRunDaoImpl) ListActiveTaskRunsByTask(ctx context.Context, taskID int64) ([]*model.ObservabilityTaskRun, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.TaskID.Eq(taskID)).Where(q.RunStatus.In(ActiveTaskRunStatuses...))

	results, err := qd.Order(q.CreatedAt.Desc()).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, nil
}

// GetLatestTaskRunByTask 获取Task的最新TaskRun
func (v *TaskRunDaoImpl) GetLatestTaskRunByTask(ctx context.Context, taskID int64) (*model.ObservabilityTaskRun, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.TaskID.Eq(taskID)).Order(q.CreatedAt.Desc())

	taskRun, err := qd.First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 没有找到TaskRun，返回nil而不是错误
		}
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return taskRun, nil
}

// ListTaskRunsByStatus 按状态获取TaskRun列表
func (v *TaskRunDaoImpl) ListTaskRunsByStatus(ctx context.Context, status string) ([]*model.ObservabilityTaskRun, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.RunStatus.Eq(status))

	results, err := qd.Order(q.CreatedAt.Desc()).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, nil
}
