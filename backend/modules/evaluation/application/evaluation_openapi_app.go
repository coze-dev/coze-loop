// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"sync"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluation_set"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/kitexutil"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
)

var (
	evaluationOpenApiApplicationOnce = sync.Once{}
	evaluationOpenApiApplication     evaluation.EvaluationOpenAPIService
)

func NewEvaluationOpenApiApplicationImpl(
	evaluationSetService service.IEvaluationSetService,
	metric metrics.OpenAPIEvaluationSetMetrics,
) evaluation.EvaluationOpenAPIService {
	evaluationOpenApiApplicationOnce.Do(func() {
		evaluationOpenApiApplication = &EvaluationOpenApiApplicationImpl{
			evaluationSetService: evaluationSetService,
			metric:               metric,
		}
	})

	return evaluationOpenApiApplication
}

type EvaluationOpenApiApplicationImpl struct {
	evaluationSetService service.IEvaluationSetService
	metric               metrics.OpenAPIEvaluationSetMetrics
}

func (e *EvaluationOpenApiApplicationImpl) CreateEvaluationSet(ctx context.Context, req *openapi.CreateEvaluationSetOpenAPIRequest) (r *openapi.CreateEvaluationSetOpenAPIResponse, err error) {
	var evaluationSetID int64
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), evaluationSetID, kitexutil.GetTOMethod(ctx), startTime, err)
	}()
	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if req.Name == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("name is required"))
	}

	// 调用domain服务
	id, err := e.evaluationSetService.CreateEvaluationSet(ctx, &entity.CreateEvaluationSetParam{
		SpaceID:             req.WorkspaceID,
		Name:                req.Name,
		Description:         req.Description,
		EvaluationSetSchema: evaluation_set.OpenAPIEvaluationSetSchemaDTO2DO(req.EvaluationSetSchema),
	})
	if err != nil {
		return nil, err
	}

	evaluationSetID = id

	// 构建响应
	return &openapi.CreateEvaluationSetOpenAPIResponse{
		Data: &openapi.CreateEvaluationSetOpenAPIData{
			EvaluationSetID: gptr.Of(id),
		},
	}, nil
}

func (e *EvaluationOpenApiApplicationImpl) GetEvaluationSet(ctx context.Context, req *openapi.GetEvaluationSetOpenAPIRequest) (r *openapi.GetEvaluationSetOpenAPIResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluationSetID(), kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	// 调用domain服务
	set, err := e.evaluationSetService.GetEvaluationSet(ctx, &req.WorkspaceID, req.EvaluationSetID, nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluation set not found"))
	}

	// 数据转换
	dto := evaluation_set.OpenAPIEvaluationSetDO2DTO(set)

	// 构建响应
	return &openapi.GetEvaluationSetOpenAPIResponse{
		Data: &openapi.GetEvaluationSetOpenAPIData{
			EvaluationSet: dto,
		},
	}, nil
}

func (e *EvaluationOpenApiApplicationImpl) ListEvaluationSets(ctx context.Context, req *openapi.ListEvaluationSetsOpenAPIRequest) (r *openapi.ListEvaluationSetsOpenAPIResponse, err error) {
	startTime := time.Now().UnixNano() / int64(time.Millisecond)
	defer func() {
		// ListEvaluationSets没有单个evaluationSetID，使用0作为占位符
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), 0, kitexutil.GetTOMethod(ctx), startTime, err)
	}()

	// 参数校验
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}

	// 调用domain服务
	sets, total, nextPageToken, err := e.evaluationSetService.ListEvaluationSets(ctx, &entity.ListEvaluationSetsParam{
		SpaceID:          req.WorkspaceID,
		EvaluationSetIDs: req.EvaluationSetIds,
		Name:             req.Name,
		Creators:         req.Creators,
		PageSize:         req.PageSize,
		PageToken:        req.PageToken,
		OrderBys:         evaluation_set.OrderByDTO2DOs(req.OrderBys),
	})
	if err != nil {
		return nil, err
	}

	// 数据转换
	dtos := evaluation_set.OpenAPIEvaluationSetDO2DTOs(sets)

	// 构建响应
	hasMore := nextPageToken != nil && *nextPageToken != ""
	return &openapi.ListEvaluationSetsOpenAPIResponse{
		Data: &openapi.ListEvaluationSetsOpenAPIData{
			Items:         dtos,
			HasMore:       gptr.Of(hasMore),
			NextPageToken: nextPageToken,
			Total:         total,
		},
	}, nil
}

func (e *EvaluationOpenApiApplicationImpl) CreateEvaluationSetVersion(ctx context.Context, req *openapi.CreateEvaluationSetVersionOpenAPIRequest) (r *openapi.CreateEvaluationSetVersionOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) BatchCreateEvaluationSetItems(ctx context.Context, req *openapi.BatchCreateEvaluationSetItemsOpenAPIRequest) (r *openapi.BatchCreateEvaluationSetItemsOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) BatchUpdateEvaluationSetItems(ctx context.Context, req *openapi.BatchUpdateEvaluationSetItemsOpenAPIRequest) (r *openapi.BatchUpdateEvaluationSetItemsOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) BatchDeleteEvaluationSetItems(ctx context.Context, req *openapi.BatchDeleteEvaluationSetItemsOpenAPIRequest) (r *openapi.BatchDeleteEvaluationSetItemsOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) ClearEvaluationSetDraftItems(ctx context.Context, req *openapi.ClearEvaluationSetDraftItemsOpenAPIRequest) (r *openapi.ClearEvaluationSetDraftItemsOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) ListEvaluationSetVersionItems(ctx context.Context, req *openapi.ListEvaluationSetVersionItemsOpenAPIRequest) (r *openapi.ListEvaluationSetVersionItemsOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) CreateEvaluator(ctx context.Context, req *openapi.CreateEvaluatorOpenAPIRequest) (r *openapi.CreateEvaluatorOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) SubmitEvaluatorVersion(ctx context.Context, req *openapi.SubmitEvaluatorVersionOpenAPIRequest) (r *openapi.SubmitEvaluatorVersionOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) GetEvaluatorVersion(ctx context.Context, req *openapi.GetEvaluatorVersionOpenAPIRequest) (r *openapi.GetEvaluatorVersionOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) RunEvaluator(ctx context.Context, req *openapi.RunEvaluatorOpenAPIRequest) (r *openapi.RunEvaluatorOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) GetEvaluatorRecord(ctx context.Context, req *openapi.GetEvaluatorRecordOpenAPIRequest) (r *openapi.GetEvaluatorRecordOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) CreateExperiment(ctx context.Context, req *openapi.CreateExperimentOpenAPIRequest) (r *openapi.CreateExperimentOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *EvaluationOpenApiApplicationImpl) GetExperimentResult_(ctx context.Context, req *openapi.GetExperimentResultOpenAPIRequest) (r *openapi.GetExperimentResultOpenAPIResponse, err error) {
	// TODO implement me
	panic("implement me")
}