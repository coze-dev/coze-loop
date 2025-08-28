// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

type ExportType string

const (
	ExportType_Append    ExportType = "append"
	ExportType_Overwrite ExportType = "overwrite"
)

type ExportTracesToDatasetRequest struct {
	WorkspaceID  int64
	SpanIds      []SpanID
	Category     entity.DatasetCategory
	Config       DatasetConfig
	StartTime    int64
	EndTime      int64
	PlatformType loop_span.PlatformType
	// 导入方式，不填默认为追加
	ExportType    ExportType
	FieldMappings []entity.FieldMapping
}

type ExportTracesToDatasetResponse struct {
	// 成功导入的数量
	SuccessCount int32
	// 错误信息
	Errors []entity.ItemErrorGroup
	// 数据集id
	DatasetID int64
	// 数据集名称
	DatasetName string
}

type SpanID struct {
	TraceID string
	SpanID  string
}

type PreviewExportTracesToDatasetResponse struct {
	// 预览数据
	Items []*entity.DatasetItem
	// 概要错误信息
	Errors []entity.ItemErrorGroup
}

type DatasetConfig struct {
	// 是否是新增数据集
	IsNewDataset bool
	// 数据集id，新增数据集时可为空
	DatasetID *int64
	// 数据集名称，选择已有数据集时可为空
	DatasetName *string
	// 数据集列数据schema
	DatasetSchema entity.DatasetSchema
}

//go:generate mockgen -destination=mocks/trace_export_service_mock.go -package=mocks . ITraceExportService
type ITraceExportService interface {
	ExportTracesToDataset(ctx context.Context, req *ExportTracesToDatasetRequest) (*ExportTracesToDatasetResponse, error)
	PreviewExportTracesToDataset(ctx context.Context, req *ExportTracesToDatasetRequest) (*PreviewExportTracesToDatasetResponse, error)
}

func NewTraceExportServiceImpl(
	tRepo repo.ITraceRepo,
	traceConfig config.ITraceConfig,
	traceProducer mq.ITraceProducer,
	annotationProducer mq.IAnnotationProducer,
	metrics metrics.ITraceMetrics,
	tenantProvider tenant.ITenantProvider,
	datasetServiceProvider *DatasetServiceAdaptor,
) (ITraceExportService, error) {
	return &TraceExportServiceImpl{
		traceRepo:             tRepo,
		traceConfig:           traceConfig,
		traceProducer:         traceProducer,
		annotationProducer:    annotationProducer,
		tenantProvider:        tenantProvider,
		metrics:               metrics,
		DatasetServiceAdaptor: datasetServiceProvider,
	}, nil
}

type TraceExportServiceImpl struct {
	traceRepo             repo.ITraceRepo
	traceConfig           config.ITraceConfig
	traceProducer         mq.ITraceProducer
	annotationProducer    mq.IAnnotationProducer
	metrics               metrics.ITraceMetrics
	tenantProvider        tenant.ITenantProvider
	DatasetServiceAdaptor *DatasetServiceAdaptor
}

func (r *TraceExportServiceImpl) ExportTracesToDataset(ctx context.Context, req *ExportTracesToDatasetRequest) (
	*ExportTracesToDatasetResponse, error,
) {
	resp := &ExportTracesToDatasetResponse{}

	spans, err := r.getSpans(ctx, req.WorkspaceID, req.SpanIds, req.StartTime, req.EndTime, req.PlatformType)
	if err != nil {
		return resp, err
	}
	if len(spans) == 0 {
		logs.CtxError(ctx, "No span found. SpanIDs:%v", req.SpanIds)
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	logs.CtxInfo(ctx, "Get spans success, total count:%v", len(spans))

	dataset, err := r.createOrUpdateDataset(ctx, req.WorkspaceID, req.Category, req.Config)
	if err != nil {
		return resp, err
	}
	datasetID := dataset.ID
	logs.CtxInfo(ctx, "Dataset is ready, ID:%v", datasetID)

	if err := r.clearDataset(ctx, datasetID, req); err != nil {
		return resp, err
	}

	successItems, errorGroups, err := r.addToDataset(ctx, spans, req.FieldMappings, req.WorkspaceID, dataset)
	if err != nil {
		return resp, err
	}

	resp.DatasetID = dataset.ID
	resp.DatasetName = dataset.Name
	resp.SuccessCount = int32(len(successItems))
	resp.Errors = errorGroups

	if err := r.addSpanAnnotations(ctx, spans, successItems, datasetID, req.Category); err != nil {
		logs.CtxError(ctx, "Add span annotations failed, err:%v", err)
		// 忽略add annotations的错误，防止用户重复导入数据集。
		return resp, nil
	}
	logs.CtxInfo(ctx, "Add span annotations success")

	return resp, nil
}

func (r *TraceExportServiceImpl) PreviewExportTracesToDataset(ctx context.Context, req *ExportTracesToDatasetRequest) (
	*PreviewExportTracesToDatasetResponse, error,
) {
	resp := &PreviewExportTracesToDatasetResponse{}
	spans, err := r.getSpans(ctx, req.WorkspaceID, req.SpanIds, req.StartTime, req.EndTime, req.PlatformType)
	if err != nil {
		return resp, err
	}
	logs.CtxInfo(ctx, "Get spans success, total count:%v", len(spans))

	dataset, err := r.buildPreviewDataset(ctx, req.WorkspaceID, req.Category, req.Config)
	if err != nil {
		return resp, err
	}

	successItems, failedItems, allItems := r.buildDatasetItems(ctx, spans, req.FieldMappings, req.WorkspaceID, dataset)

	var ignoreCurrentCount *bool
	if !req.Config.IsNewDataset && req.ExportType == ExportType_Overwrite {
		ignoreCurrentCount = lo.ToPtr(true)
	}
	addSuccess, errorGroups, err := r.getDatasetProvider(dataset.DatasetCategory).ValidateDatasetItems(ctx, dataset, successItems, ignoreCurrentCount)
	if err != nil {
		return resp, err
	}
	logs.CtxInfo(ctx, "Validate dataset items success, success count:%v, error groups:%#v", len(addSuccess), errorGroups)

	errorGroups = r.mergeErrorGroups(failedItems, errorGroups)
	if len(errorGroups) > 0 {
		logs.CtxInfo(ctx, "Merge error groups:%#v", errorGroups)
	}

	resp.Items = allItems
	resp.Errors = errorGroups
	return resp, nil
}

func (r *TraceExportServiceImpl) createOrUpdateDataset(ctx context.Context, workspaceID int64, category entity.DatasetCategory, config DatasetConfig) (*entity.Dataset, error) {
	var err error
	var datasetID int64

	if config.IsNewDataset {
		if config.DatasetName == nil || *config.DatasetName == "" {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset name is empty"))
		}
		if len(config.DatasetSchema.FieldSchemas) == 0 {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset schema is empty"))
		}

		datasetID, err = r.getDatasetProvider(category).CreateDataset(ctx, entity.NewDataset(
			0,
			workspaceID,
			*config.DatasetName,
			category,
			config.DatasetSchema,
		))
		if err != nil {
			return nil, err
		}
	} else {
		if config.DatasetID == nil {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("dataset id is nil"))
		}
		datasetID = *config.DatasetID
		needUpdate := false
		for _, schema := range config.DatasetSchema.FieldSchemas {
			if schema.Key == nil || *schema.Key == "" {
				needUpdate = true
				break
			}
		}
		if needUpdate {
			if err := r.getDatasetProvider(category).UpdateDatasetSchema(ctx, entity.NewDataset(
				datasetID,
				workspaceID,
				"",
				category,
				config.DatasetSchema,
			)); err != nil {
				return nil, err
			}
		}
	}

	// 新增或修改评测集后，都需要重新查询一次，拿到fieldSchema里的key
	return r.getDatasetProvider(category).GetDataset(ctx, workspaceID, datasetID, category)
}

func (r *TraceExportServiceImpl) getSpans(ctx context.Context, workspaceID int64, sids []SpanID, startTime, endTime int64, platformType loop_span.PlatformType) (loop_span.SpanList, error) {
	tenant, err := r.tenantProvider.GetTenantsByPlatformType(ctx, platformType)
	if err != nil {
		return nil, err
	}
	spanIDs := lo.Map(sids, func(s SpanID, _ int) string { return s.SpanID })
	traceIDs := lo.UniqMap(sids, func(s SpanID, _ int) string { return s.TraceID })
	result, err := r.traceRepo.ListSpans(ctx, &repo.ListSpansParam{
		Tenants: tenant,
		Filters: &loop_span.FilterFields{
			FilterFields: []*loop_span.FilterField{
				{
					FieldName: "space_id",
					FieldType: loop_span.FieldTypeString,
					Values:    []string{strconv.FormatInt(workspaceID, 10)},
					QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
				},
				{
					FieldName: "trace_id",
					FieldType: loop_span.FieldTypeString,
					Values:    traceIDs,
					QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				},
				{
					FieldName: "span_id",
					FieldType: loop_span.FieldTypeString,
					Values:    spanIDs,
					QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				},
			},
		},
		StartAt: startTime,
		EndAt:   endTime,
		Limit:   int32(len(sids)),
	})
	if err != nil {
		return nil, err
	}

	// todo tyf 解密
	return result.Spans, nil
}

func (r *TraceExportServiceImpl) clearDataset(ctx context.Context, datasetID int64, req *ExportTracesToDatasetRequest) error {
	if req.ExportType == ExportType_Overwrite && !req.Config.IsNewDataset {
		err := r.getDatasetProvider(req.Category).ClearDatasetItems(ctx, req.WorkspaceID, datasetID, req.Category)
		if err != nil {
			return err
		}
		logs.CtxInfo(ctx, "Clear dataset success, ID:%v", datasetID)
		return nil
	}
	return nil
}

func (r *TraceExportServiceImpl) addToDataset(ctx context.Context, spans []*loop_span.Span, fieldMappings []entity.FieldMapping,
	workspaceID int64, dataset *entity.Dataset,
) ([]*entity.DatasetItem, []entity.ItemErrorGroup, error) {
	successItems, failedItems, _ := r.buildDatasetItems(ctx, spans, fieldMappings, workspaceID, dataset)
	logs.CtxInfo(ctx, "Build dataset items success, success count:%v, failed count:%v", len(successItems), len(failedItems))

	addSuccess, errorGroups, err := r.getDatasetProvider(dataset.DatasetCategory).AddDatasetItems(ctx, dataset.ID, dataset.DatasetCategory, successItems)
	if err != nil {
		return nil, nil, err
	}
	logs.CtxInfo(ctx, "Add dataset items success, success count:%v, error groups:%#v", len(addSuccess), errorGroups)

	errorGroups = r.mergeErrorGroups(failedItems, errorGroups)
	if len(errorGroups) > 0 {
		logs.CtxInfo(ctx, "Merge error groups:%#v", errorGroups)
	}

	return addSuccess, errorGroups, nil
}

func (r *TraceExportServiceImpl) mergeErrorGroups(failedItems []*entity.DatasetItem, errorGroups []entity.ItemErrorGroup) []entity.ItemErrorGroup {
	errorGroupMap := lo.SliceToMap(errorGroups, func(errorGroup entity.ItemErrorGroup) (int64, *entity.ItemErrorGroup) {
		return errorGroup.Type, &errorGroup
	})
	for _, failedItem := range failedItems {
		// 按Span粒度而不是按field粒度聚合错误信息，只保留第一个错误
		itemError := failedItem.Error[0]
		if errorGroup, ok := errorGroupMap[itemError.Type]; ok {
			errorGroup.ErrorCount++
		} else {
			errorGroupMap[itemError.Type] = &entity.ItemErrorGroup{
				Type:       itemError.Type,
				Summary:    itemError.Message,
				ErrorCount: 1,
			}
		}
	}
	return lo.MapToSlice(errorGroupMap, func(key int64, value *entity.ItemErrorGroup) entity.ItemErrorGroup {
		return *value
	})
}

func (r *TraceExportServiceImpl) addSpanAnnotations(ctx context.Context, spans []*loop_span.Span, successItems []*entity.DatasetItem, datasetID int64, category entity.DatasetCategory) error {
	spanMap := lo.SliceToMap(spans, func(span *loop_span.Span) (string, *loop_span.Span) {
		return span.SpanID, span
	})
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return errorx.NewByCode(errno.UserParseFailedCode)
	}

	var annotationType loop_span.AnnotationType
	switch category {
	case entity.DatasetCategory_General:
		annotationType = loop_span.AnnotationTypeManualDataset
	case entity.DatasetCategory_Evaluation:
		annotationType = loop_span.AnnotationTypeManualEvaluationSet
	default:
		annotationType = loop_span.AnnotationTypeManualDataset
	}

	for _, item := range successItems {
		span, ok := spanMap[item.SpanID]
		if !ok {
			logs.CtxWarn(ctx, "Span not found, span_id:%v", item.SpanID)
			continue
		}
		annotation, err := span.AddManualDatasetAnnotation(item.DatasetID, userID, annotationType)
		if err != nil {
			// 忽略add annotations的错误，防止用户重复导入数据集。
			logs.CtxError(ctx, "Failed to add annotation, span_id:%v, err:%+v", item.SpanID, err)
			continue
		}
		err = r.traceRepo.InsertAnnotations(ctx, &repo.InsertAnnotationParam{
			Tenant:      span.GetTenant(),
			TTL:         span.GetTTL(ctx),
			Annotations: []*loop_span.Annotation{annotation},
		})
		if err != nil {
			// 忽略add annotations的错误，防止用户重复导入数据集。
			logs.CtxError(ctx, "Failed to add annotation, span_id:%v, err:%+v", item.SpanID, err)
			continue
		}
	}

	return nil
}

func (r *TraceExportServiceImpl) buildDatasetItems(ctx context.Context, spans []*loop_span.Span, fieldMappings []entity.FieldMapping,
	workspaceID int64, dataset *entity.Dataset,
) (successItems, failedItems, allItems []*entity.DatasetItem) {
	successItems = make([]*entity.DatasetItem, 0, len(spans))
	failedItems = make([]*entity.DatasetItem, 0)
	allItems = make([]*entity.DatasetItem, 0, len(spans))
	for i, span := range spans {
		item := r.buildItem(ctx, span, i, fieldMappings, workspaceID, dataset)
		allItems = append(allItems, item)
		if len(item.Error) > 0 {
			failedItems = append(failedItems, item)
		} else {
			successItems = append(successItems, item)
		}
	}

	return successItems, failedItems, allItems
}

func (r *TraceExportServiceImpl) buildItem(ctx context.Context, span *loop_span.Span, i int, fieldMappings []entity.FieldMapping, workspaceID int64,
	dataset *entity.Dataset,
) *entity.DatasetItem {
	item := entity.NewDatasetItem(workspaceID, dataset.ID, span.SpanID)
	for _, mapping := range fieldMappings {
		value, err := span.ExtractByJsonpath(ctx, mapping.TraceFieldKey, mapping.TraceFieldJsonpath)
		if err != nil {
			// 非json但使用了jsonpath，也不报错，置空
			logs.CtxInfo(ctx, "Extract field failed, err:%v", err)
		}

		content, errCode := entity.GetContentInfo(ctx, mapping.FieldSchema.ContentType, value)
		if errCode == entity.DatasetErrorType_MismatchSchema {
			item.AddError("invalid multi part", entity.DatasetErrorType_MismatchSchema, nil)
			continue
		}

		// 前端传入的是Name，评测集需要的是key，需要做一下mapping
		key := dataset.GetFieldSchemaKeyByName(mapping.FieldSchema.Name)
		if key == "" {
			logs.CtxInfo(ctx, "Dataset field key is empty, name:%v", mapping.FieldSchema.Name)
			item.AddError("Dataset field key is empty", entity.DatasetErrorType_InternalError, nil)
			continue
		}
		item.AddFieldData(key, mapping.FieldSchema.Name, content)
	}
	return item
}

func (r *TraceExportServiceImpl) buildPreviewDataset(ctx context.Context, workspaceID int64, category entity.DatasetCategory, config DatasetConfig) (*entity.Dataset, error) {
	schema := config.DatasetSchema
	for i := range schema.FieldSchemas {
		fieldSchema := &schema.FieldSchemas[i]
		// 预览时不一定有schema key，没有的时候用name生成
		if fieldSchema.Key == nil || *fieldSchema.Key == "" {
			fieldSchema.Key = lo.ToPtr(fieldSchema.Name)
		}
	}

	dataset := entity.NewDataset(
		0,
		workspaceID,
		"",
		category,
		schema,
	)
	if config.DatasetID != nil {
		dataset.ID = *config.DatasetID
	}
	if config.DatasetName != nil {
		dataset.Name = *config.DatasetName
	}
	return dataset, nil
}

func (r *TraceExportServiceImpl) getDatasetProvider(category entity.DatasetCategory) rpc.IDatasetProvider {
	return r.DatasetServiceAdaptor.getDatasetProvider(category)
}

type DatasetServiceAdaptor struct {
	datasetServiceMap map[entity.DatasetCategory]rpc.IDatasetProvider
}

func NewDatasetServiceAdaptor() *DatasetServiceAdaptor {
	return &DatasetServiceAdaptor{}
}

func (d *DatasetServiceAdaptor) Register(category entity.DatasetCategory, provider rpc.IDatasetProvider) {
	if d.datasetServiceMap == nil {
		d.datasetServiceMap = make(map[entity.DatasetCategory]rpc.IDatasetProvider)
	}
	d.datasetServiceMap[category] = provider
}

func (d *DatasetServiceAdaptor) getDatasetProvider(category entity.DatasetCategory) rpc.IDatasetProvider {
	datasetProvider, ok := d.datasetServiceMap[category]
	if !ok {
		return rpc.NoopDatasetProvider
	}
	return datasetProvider
}
