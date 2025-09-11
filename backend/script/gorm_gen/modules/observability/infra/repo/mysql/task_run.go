// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
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
	WorkspaceIDs   []int64
	TaskID         *int64
	TaskRunFilters *filter.TaskFilterFields // 暂时复用TaskFilterFields，后续可扩展为TaskRunFilterFields
	ReqLimit       int32
	ReqOffset      int32
	OrderBy        *common.OrderBy
}

//go:generate mockgen -destination=mocks/task_run.go -package=mocks . ITaskRunDao
type ITaskRunDao interface {
	GetTaskRun(ctx context.Context, id int64, workspaceID *int64, userID *string) (*model.ObservabilityTaskRun, error)
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

func (v *TaskRunDaoImpl) GetTaskRun(ctx context.Context, id int64, workspaceID *int64, userID *string) (*model.ObservabilityTaskRun, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityTaskRun
	qd := q.WithContext(ctx).Where(q.ID.Eq(id))
	if workspaceID != nil {
		qd = qd.Where(q.WorkspaceID.Eq(*workspaceID))
	}
	if userID != nil {
		// 注意：TaskRun模型中没有CreatedBy字段，此过滤条件暂时跳过
		// 如果需要按创建者过滤，需要通过关联Task表来实现
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

	// 工作空间过滤
	if len(param.WorkspaceIDs) != 0 {
		qd = qd.Where(q.ObservabilityTaskRun.WorkspaceID.In(param.WorkspaceIDs...))
	}

	// TaskID过滤
	if param.TaskID != nil {
		qd = qd.Where(q.ObservabilityTaskRun.TaskID.Eq(*param.TaskID))
	}

	// 应用过滤条件
	qdf, err := v.applyTaskRunFilters(q, param.TaskRunFilters)
	if err != nil {
		return nil, 0, err
	}
	if qdf != nil {
		qd = qd.Where(qdf)
	}

	// 排序
	qd = qd.Order(v.order(q, param.OrderBy.GetField(), param.OrderBy.GetIsAsc()))

	// 计算总数
	total, err = qd.Count()
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

// 处理TaskRun过滤条件
func (v *TaskRunDaoImpl) applyTaskRunFilters(q *query.Query, taskRunFilters *filter.TaskFilterFields) (field.Expr, error) {
	var filterExpr field.Expr
	if taskRunFilters == nil {
		return nil, nil
	}

	for _, f := range taskRunFilters.FilterFields {
		if f.FieldName == nil || f.QueryType == nil {
			return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("field name or query type is nil"))
		}

		switch *f.FieldName {
		case "task_run_id":
			if len(f.Values) == 0 {
				return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no value provided for task run id"))
			}
			var taskRunIDs []int64
			for _, value := range f.Values {
				taskRunID, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithMsgParam("invalid task run id: %v", err.Error()))
				}
				taskRunIDs = append(taskRunIDs, taskRunID)
			}
			filterExpr = q.ObservabilityTaskRun.ID.In(taskRunIDs...)
		case "task_id":
			if len(f.Values) == 0 {
				return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no value provided for task id"))
			}
			var taskIDs []int64
			for _, value := range f.Values {
				taskID, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithMsgParam("invalid task id: %v", err.Error()))
				}
				taskIDs = append(taskIDs, taskID)
			}
			filterExpr = q.ObservabilityTaskRun.TaskID.In(taskIDs...)
		case "task_run_status":
			switch *f.QueryType {
			case filter.QueryTypeIn:
				if len(f.Values) == 0 {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no values provided for in query"))
				}
				filterExpr = q.ObservabilityTaskRun.RunStatus.In(f.Values...)
			case filter.QueryTypeNotIn:
				if len(f.Values) == 0 {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no values provided for not in query"))
				}
				filterExpr = q.ObservabilityTaskRun.RunStatus.NotIn(f.Values...)
			default:
				return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("invalid query type for task run status"))
			}
		case "created_by":
			// TaskRun模型中没有CreatedBy字段，如果需要按创建者过滤，需要通过关联Task表来实现
			return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("created_by filter not supported for TaskRun, use Task association instead"))
		default:
			return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithMsgParam("invalid filter field name: %s", *f.FieldName))
		}
	}

	return filterExpr, nil
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