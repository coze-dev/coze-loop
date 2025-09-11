// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
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
	OrderBy      *common.OrderBy
}

//go:generate mockgen -destination=mocks/task.go -package=mocks . ITaskDao
type ITaskDao interface {
	GetTask(ctx context.Context, id int64, workspaceID *int64, userID *string) (*model.ObservabilityTask, error)
	CreateTask(ctx context.Context, po *model.ObservabilityTask) (int64, error)
	UpdateTask(ctx context.Context, po *model.ObservabilityTask) error
	DeleteTask(ctx context.Context, id int64, workspaceID int64, userID string) error
	ListTasks(ctx context.Context, param ListTaskParam) ([]*model.ObservabilityTask, int64, error)
	ListNonFinalTask(ctx context.Context) ([]*model.ObservabilityTask, error)
	UpdateTaskWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error
	GetObjListWithTask(ctx context.Context) ([]string, []string, error)
	ListNonFinalTaskBySpaceID(ctx context.Context, workspaceID int64) ([]*model.ObservabilityTask, error)
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

func (v *TaskDaoImpl) ListTasks(ctx context.Context, param ListTaskParam) ([]*model.ObservabilityTask, int64, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx))
	qd := q.WithContext(ctx).ObservabilityTask
	var total int64
	if len(param.WorkspaceIDs) != 0 {
		qd = qd.Where(q.ObservabilityTask.WorkspaceID.In(param.WorkspaceIDs...))
	}
	// 应用过滤条件
	qdf, err := v.applyTaskFilters(q, param.TaskFilters)
	if err != nil {
		return nil, 0, err
	}
	if qdf != nil {
		qd = qd.Where(qdf)
	}
	// 计算总数
	total, err = qd.Count()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	// order by
	qd = qd.Order(v.order(q, param.OrderBy.GetField(), param.OrderBy.GetIsAsc()))
	// 计算分页参数
	limit, offset := calculatePagination(param.ReqLimit, param.ReqOffset)
	results, err := qd.Limit(limit).Offset(offset).Find()
	if err != nil {
		return nil, total, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, total, nil
}

// 处理任务过滤条件
func (v *TaskDaoImpl) applyTaskFilters(q *query.Query, taskFilters *filter.TaskFilterFields) (field.Expr, error) {
	var filterExpr field.Expr
	if taskFilters == nil {
		return nil, nil
	}
	for _, f := range taskFilters.FilterFields {
		if f.FieldName == nil || f.QueryType == nil {
			return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("field name or query type is nil"))
		}

		switch *f.FieldName {
		case filter.TaskFieldNameTaskName:
			switch *f.QueryType {
			case filter.QueryTypeEq:
				if len(f.Values) == 0 {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg(("no value provided for query")))
				}
				filterExpr = q.ObservabilityTask.Name.Eq(f.Values[0])
			case filter.QueryTypeMatch:
				if len(f.Values) == 0 {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no value provided for query"))
				}
				filterExpr = q.ObservabilityTask.Name.Like(fmt.Sprintf("%%%s%%", f.Values[0]))
			default:
				return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("invalid query type for task name"))
			}
		case filter.TaskFieldNameTaskType:
			switch *f.QueryType {
			case filter.QueryTypeIn:
				if len(f.Values) == 0 {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no values provided for in query"))
				}
				filterExpr = q.ObservabilityTask.TaskType.In(f.Values...)
			case filter.QueryTypeNotIn:
				if len(f.Values) == 0 {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no values provided for not in query"))
				}
				filterExpr = q.ObservabilityTask.TaskType.NotIn(f.Values...)
			default:
				return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("invalid query type for task type"))
			}
		case filter.TaskFieldNameTaskStatus:
			switch *f.QueryType {
			case filter.QueryTypeIn:
				if len(f.Values) == 0 {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no values provided for in query"))
				}
				filterExpr = q.ObservabilityTask.TaskStatus.In(f.Values...)
			case filter.QueryTypeNotIn:
				if len(f.Values) == 0 {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no values provided for not in query"))
				}
				filterExpr = q.ObservabilityTask.TaskStatus.NotIn(f.Values...)
			default:
				return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("invalid query type for task status"))
			}
		case filter.TaskFieldNameCreatedBy:
			switch *f.QueryType {
			case filter.QueryTypeIn:
				if len(f.Values) == 0 {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no values provided for in query"))
				}
				filterExpr = q.ObservabilityTask.CreatedBy.In(f.Values...)
			case filter.QueryTypeNotIn:
				if len(f.Values) == 0 {
					return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no values provided for not in query"))
				}
				filterExpr = q.ObservabilityTask.CreatedBy.NotIn(f.Values...)
			default:
				return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("invalid query type for created_by"))
			}
		case filter.TaskFieldNameSampleRate:
			if len(f.Values) == 0 {
				return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("no value provided for sample rate"))
			}
			//sampleRate, err := strconv.ParseFloat(f.Values[0], 64)
			//if err != nil {
			//	return nil,  errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithMsgParam("invalid sample rate: %v", err.Error()))
			//}
			switch *f.QueryType {
			case filter.QueryTypeGte:
				//filterExpr = q.ObservabilityTask.Sampler.Gte(sampleRate)
				//db = db.Where("JSON_EXTRACT(sampler, '$.sample_rate') >= ?", sampleRate)
			case filter.QueryTypeLte:
				//db = db.Where("JSON_EXTRACT(sampler, '$.sample_rate') <= ?", sampleRate)
			case filter.QueryTypeEq:
				//db = db.Where("JSON_EXTRACT(sampler, '$.sample_rate') = ?", sampleRate)
			case filter.QueryTypeNotEq:
				//db = db.Where("JSON_EXTRACT(sampler, '$.sample_rate') !=?", sampleRate)
			default:
				return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("invalid query type for sample rate"))
			}
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

			filterExpr = q.ObservabilityTask.ID.In(taskIDs...)
		default:
			return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithMsgParam("invalid filter field name: %s", *f.FieldName))
		}
	}

	return filterExpr, nil
}

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

func (d *TaskDaoImpl) order(q *query.Query, orderBy string, asc bool) field.Expr {
	var orderExpr field.OrderExpr
	switch orderBy {
	case "created_at":
		orderExpr = q.ObservabilityTask.CreatedAt
	default:
		orderExpr = q.ObservabilityTask.CreatedAt
	}
	if asc {
		return orderExpr.Asc()
	}
	return orderExpr.Desc()
}

func (v *TaskDaoImpl) ListNonFinalTask(ctx context.Context) ([]*model.ObservabilityTask, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx))
	qd := q.WithContext(ctx).ObservabilityTask
	qd = qd.Where(q.ObservabilityTask.TaskStatus.NotIn("success", "disabled"))

	results, err := qd.Limit(500).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, nil
}

func (v *TaskDaoImpl) ListNonFinalTaskBySpaceID(ctx context.Context, workspaceID int64) ([]*model.ObservabilityTask, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx))
	qd := q.WithContext(ctx).ObservabilityTask
	qd = qd.Where(q.ObservabilityTask.WorkspaceID.Eq(workspaceID))
	qd = qd.Where(q.ObservabilityTask.TaskStatus.NotIn("success", "disabled"))

	results, err := qd.Limit(500).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, nil
}

func (v *TaskDaoImpl) UpdateTaskWithOCC(ctx context.Context, id int64, workspaceID int64, updateMap map[string]interface{}) error {
	//todo[xun]: 乐观锁
	logs.CtxInfo(ctx, "UpdateTaskWithOCC, id:%d, workspaceID:%d, updateMap:%+v", id, workspaceID, updateMap)
	q := genquery.Use(v.dbMgr.NewSession(ctx))
	qd := q.WithContext(ctx).ObservabilityTask
	qd = qd.Where(q.ObservabilityTask.ID.Eq(id)).Where(q.ObservabilityTask.WorkspaceID.Eq(workspaceID))
	info, err := qd.Updates(updateMap)
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	logs.CtxInfo(ctx, "%d rows updated", info.RowsAffected)
	return nil
}

func (v *TaskDaoImpl) GetObjListWithTask(ctx context.Context) ([]string, []string, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx))
	qd := q.WithContext(ctx).ObservabilityTask

	// 查询非终态任务的workspace_id，使用DISTINCT去重
	qd = qd.Where(q.ObservabilityTask.TaskStatus.NotIn("success", "disabled"))
	qd = qd.Select(q.ObservabilityTask.WorkspaceID).Distinct()

	results, err := qd.Find()
	if err != nil {
		return nil, nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}

	// 转换为字符串数组
	var spaceList []string
	for _, task := range results {
		spaceList = append(spaceList, strconv.FormatInt(task.WorkspaceID, 10))
	}

	// botList暂时返回空数组，因为Task表中没有bot_id字段
	var botList []string

	return spaceList, botList, nil
}
