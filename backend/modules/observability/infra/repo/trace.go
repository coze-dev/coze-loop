// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/storage"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	metric_repo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao/converter"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	time_util "github.com/coze-dev/coze-loop/backend/pkg/time"
	"github.com/samber/lo"
)

type TraceRepoOption func(*TraceRepoImpl)

func WithTraceStorageDaos(storageType string, spanDao dao.ISpansDao, annoDao dao.IAnnotationDao) TraceRepoOption {
	return func(t *TraceRepoImpl) {
		WithTraceStorageSpanDao(storageType, spanDao)(t)
		WithTraceStorageAnnotationDao(storageType, annoDao)(t)
	}
}

func WithTraceStorageSpanDao(storageType string, spanDao dao.ISpansDao) TraceRepoOption {
	return func(t *TraceRepoImpl) {
		if storageType == "" || spanDao == nil {
			return
		}
		if t.spanDaos == nil {
			t.spanDaos = make(map[string]dao.ISpansDao)
		}
		t.spanDaos[storageType] = spanDao
	}
}

func WithTraceStorageAnnotationDao(storageType string, annoDao dao.IAnnotationDao) TraceRepoOption {
	return func(t *TraceRepoImpl) {
		if storageType == "" || annoDao == nil {
			return
		}
		if t.annoDaos == nil {
			t.annoDaos = make(map[string]dao.IAnnotationDao)
		}
		t.annoDaos[storageType] = annoDao
	}
}

func NewTraceRepoImpl(
	traceConfig config.ITraceConfig,
	storageProvider storage.IStorageProvider,
	opts ...TraceRepoOption,
) (repo.ITraceRepo, error) {
	impl, err := newTraceRepoImpl(traceConfig, storageProvider, opts...)
	if err != nil {
		return nil, err
	}
	return impl, nil
}

func NewTraceMetricCKRepoImpl(
	traceConfig config.ITraceConfig,
	storageProvider storage.IStorageProvider,
	opts ...TraceRepoOption,
) (metric_repo.IMetricRepo, error) {
	return newTraceRepoImpl(traceConfig, storageProvider, opts...)
}

func newTraceRepoImpl(
	traceConfig config.ITraceConfig,
	storageProvider storage.IStorageProvider,
	opts ...TraceRepoOption,
) (*TraceRepoImpl, error) {
	impl := &TraceRepoImpl{
		traceConfig:     traceConfig,
		storageProvider: storageProvider,
		spanDaos:        make(map[string]dao.ISpansDao),
		annoDaos:        make(map[string]dao.IAnnotationDao),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(impl)
		}
	}
	return impl, nil
}

type TraceRepoImpl struct {
	traceConfig     config.ITraceConfig
	storageProvider storage.IStorageProvider
	spanDaos        map[string]dao.ISpansDao
	annoDaos        map[string]dao.IAnnotationDao
}

type PageToken struct {
	StartTime int64  `json:"StartTime"`
	SpanID    string `json:"SpanID"`
}

func (t *TraceRepoImpl) InsertSpans(ctx context.Context, param *repo.InsertTraceParam) error {
	spanDao := t.spanDaos[ck.TraceStorageTypeCK]
	if spanDao == nil {
		return errorx.WrapByCode(errors.New("invalid storage"), obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	table, err := t.getSpanInsertTable(ctx, param.Tenant, param.TTL)
	if err != nil {
		return err
	}
	if err := spanDao.Insert(ctx, &dao.InsertParam{
		Table: table,
		Spans: converter.SpanListDO2PO(param.Spans, param.TTL),
	}); err != nil {
		logs.CtxError(ctx, "fail to insert spans, %v", err)
		return err
	}
	logs.CtxInfo(ctx, "insert spans into table %s successfully, count %d", table, len(param.Spans))
	return nil
}

func (t *TraceRepoImpl) ListSpans(ctx context.Context, req *repo.ListSpansParam) (*repo.ListSpansResult, error) {
	spanStorage := t.storageProvider.GetTraceStorage(ctx, req.WorkSpaceID)
	spanDao := t.spanDaos[spanStorage.StorageName]
	if spanDao == nil {
		return nil, errorx.WrapByCode(errors.New("invalid storage"), obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	annoDao := t.annoDaos[spanStorage.StorageName]
	if annoDao == nil {
		return nil, errorx.WrapByCode(errors.New("invalid storage"), obErrorx.CommercialCommonInvalidParamCodeCode)
	}

	pageToken, err := parsePageToken(req.PageToken)
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid list spans request"))
	}
	if pageToken != nil {
		req.Filters = t.addPageTokenFilter(pageToken, req.Filters)
	}
	tableCfg, err := t.getQueryTenantTables(ctx, req.Tenants)
	if err != nil {
		return nil, err
	}
	st := time.Now()
	spans, err := spanDao.Get(ctx, &dao.QueryParam{
		QueryType:        dao.QueryTypeListSpans,
		Tables:           tableCfg.SpanTables,
		AnnoTableMap:     tableCfg.AnnoTableMap,
		StartTime:        time_util.MillSec2MicroSec(req.StartAt),
		EndTime:          time_util.MillSec2MicroSec(req.EndAt),
		Filters:          req.Filters,
		Limit:            req.Limit + 1,
		OrderByStartTime: req.DescByStartTime,
		OmitColumns:      req.OmitColumns,
		Extra:            spanStorage.StorageConfig,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "list spans successfully, spans count %d, cost %v", len(spans), time.Since(st))
	spanDOList := converter.SpanListPO2DO(spans)
	if tableCfg.NeedQueryAnno && !req.NotQueryAnnotation {
		spanIDs := lo.UniqMap(spans, func(item *dao.Span, _ int) string {
			return item.SpanID
		})
		st = time.Now()
		annotations, err := annoDao.List(ctx, &dao.ListAnnotationsParam{
			Tables:    tableCfg.AnnoTables,
			SpanIDs:   spanIDs,
			StartTime: time_util.MillSec2MicroSec(req.StartAt),
			EndTime:   time_util.MillSec2MicroSec(req.EndAt),
			Limit:     int32(min(len(spanIDs)*100, 10000)),
		})
		logs.CtxInfo(ctx, "get annotations successfully, annotations count %d, cost %v", len(annotations), time.Since(st))
		if err != nil {
			return nil, err
		}
		annoDOList := converter.AnnotationListPO2DO(annotations)
		spanDOList.SetAnnotations(annoDOList)
	}
	result := &repo.ListSpansResult{
		Spans:   spanDOList,
		HasMore: len(spans) > int(req.Limit),
	}
	if result.HasMore {
		result.Spans = result.Spans[:len(result.Spans)-1]
	}
	if len(result.Spans) > 0 {
		lastSpan := result.Spans[len(result.Spans)-1]
		pageToken := &PageToken{
			StartTime: lastSpan.StartTime,
			SpanID:    lastSpan.SpanID,
		}
		pt, _ := json.Marshal(pageToken)
		result.PageToken = base64.StdEncoding.EncodeToString(pt)
	}
	result.Spans = result.Spans.Uniq()
	return result, nil
}

func (t *TraceRepoImpl) GetTrace(ctx context.Context, req *repo.GetTraceParam) (loop_span.SpanList, error) {
	spanStorage := t.storageProvider.GetTraceStorage(ctx, req.WorkSpaceID)
	spanDao := t.spanDaos[spanStorage.StorageName]
	if spanDao == nil {
		return nil, errorx.WrapByCode(errors.New("invalid storage"), obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	annoDao := t.annoDaos[spanStorage.StorageName]
	if annoDao == nil {
		return nil, errorx.WrapByCode(errors.New("invalid storage"), obErrorx.CommercialCommonInvalidParamCodeCode)
	}

	tableCfg, err := t.getQueryTenantTables(ctx, req.Tenants)
	if err != nil {
		return nil, err
	}
	filter := &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
	}
	if req.TraceID != "" {
		filter.FilterFields = append(filter.FilterFields, &loop_span.FilterField{
			FieldName: loop_span.SpanFieldTraceId,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{req.TraceID},
			QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		})
	} else {
		filter.FilterFields = append(filter.FilterFields, &loop_span.FilterField{
			FieldName: loop_span.SpanFieldLogID,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{req.LogID},
			QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		})
	}
	if len(req.SpanIDs) > 0 {
		filter.FilterFields = append(filter.FilterFields, &loop_span.FilterField{
			FieldName: loop_span.SpanFieldSpanId,
			FieldType: loop_span.FieldTypeString,
			Values:    req.SpanIDs,
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		})
	}
	filter.FilterFields = append(filter.FilterFields, &loop_span.FilterField{
		SubFilter: req.Filters,
	})
	st := time.Now()
	spans, err := spanDao.Get(ctx, &dao.QueryParam{
		QueryType:     dao.QueryTypeGetTrace,
		Tables:        tableCfg.SpanTables,
		AnnoTableMap:  tableCfg.AnnoTableMap,
		StartTime:     time_util.MillSec2MicroSec(req.StartAt),
		EndTime:       time_util.MillSec2MicroSec(req.EndAt),
		Filters:       filter,
		Limit:         req.Limit,
		OmitColumns:   req.OmitColumns,
		SelectColumns: req.SelectColumns,
		Extra:         spanStorage.StorageConfig,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "get trace %s successfully, spans count %d, cost %v",
		req.TraceID, len(spans), time.Since(st))
	spanDOList := converter.SpanListPO2DO(spans)
	if tableCfg.NeedQueryAnno && !req.NotQueryAnnotation {
		spanIDs := lo.UniqMap(spans, func(item *dao.Span, _ int) string {
			return item.SpanID
		})
		st = time.Now()
		annotations, err := annoDao.List(ctx, &dao.ListAnnotationsParam{
			Tables:    tableCfg.AnnoTables,
			SpanIDs:   spanIDs,
			StartTime: time_util.MillSec2MicroSec(req.StartAt),
			EndTime:   time_util.MillSec2MicroSec(req.EndAt),
			Limit:     int32(min(len(spanIDs)*100, 10000)),
		})
		logs.CtxInfo(ctx, "get annotations successfully, annotations count %d, cost %v", len(annotations), time.Since(st))
		if err != nil {
			return nil, err
		}
		annoDOList := converter.AnnotationListPO2DO(annotations)
		spanDOList.SetAnnotations(annoDOList.Uniq())
	}
	return spanDOList.Uniq(), nil
}

func (t *TraceRepoImpl) ListAnnotations(ctx context.Context, param *repo.ListAnnotationsParam) (loop_span.AnnotationList, error) {
	spanStorage := t.storageProvider.GetTraceStorage(ctx, param.WorkSpaceID)
	annoDao := t.annoDaos[spanStorage.StorageName]
	if annoDao == nil {
		return nil, errorx.WrapByCode(errors.New("invalid storage"), obErrorx.CommercialCommonInvalidParamCodeCode)
	}

	if param.SpanID == "" || param.TraceID == "" || param.WorkspaceId <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	tableCfg, err := t.getQueryTenantTables(ctx, param.Tenants)
	if err != nil {
		return nil, err
	} else if len(tableCfg.AnnoTables) == 0 {
		return loop_span.AnnotationList{}, nil
	}
	st := time.Now()
	annotations, err := annoDao.List(ctx, &dao.ListAnnotationsParam{
		Tables:          tableCfg.AnnoTables,
		SpanIDs:         []string{param.SpanID},
		StartTime:       time_util.MillSec2MicroSec(param.StartAt),
		EndTime:         time_util.MillSec2MicroSec(param.EndAt),
		DescByUpdatedAt: param.DescByUpdatedAt,
		Limit:           100,
		Extra:           spanStorage.StorageConfig,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "get annotations successfully, annotations count %d, cost %v", len(annotations), time.Since(st))
	workspaceIDStr := strconv.FormatInt(param.WorkspaceId, 10)
	annotations = lo.Filter(annotations, func(item *dao.Annotation, _ int) bool {
		return item.TraceID == param.TraceID && item.SpaceID == workspaceIDStr
	})
	return converter.AnnotationListPO2DO(annotations).Uniq(), nil
}

func (t *TraceRepoImpl) GetAnnotation(ctx context.Context, param *repo.GetAnnotationParam) (*loop_span.Annotation, error) {
	spanStorage := t.storageProvider.GetTraceStorage(ctx, param.WorkSpaceID)
	annoDao := t.annoDaos[spanStorage.StorageName]
	if annoDao == nil {
		return nil, errorx.WrapByCode(errors.New("invalid storage"), obErrorx.CommercialCommonInvalidParamCodeCode)
	}

	tableCfg, err := t.getQueryTenantTables(ctx, param.Tenants)
	if err != nil {
		return nil, err
	} else if len(tableCfg.AnnoTables) == 0 {
		return nil, nil
	}
	st := time.Now()
	annotation, err := annoDao.Get(ctx, &dao.GetAnnotationParam{
		Tables:    tableCfg.AnnoTables,
		ID:        param.ID,
		StartTime: time_util.MillSec2MicroSec(param.StartAt),
		EndTime:   time_util.MillSec2MicroSec(param.EndAt),
		Limit:     2,
		Extra:     spanStorage.StorageConfig,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "get annotation successfully, cost %v", time.Since(st))
	return converter.AnnotationPO2DO(annotation), nil
}

func (t *TraceRepoImpl) InsertAnnotations(ctx context.Context, param *repo.InsertAnnotationParam) error {
	spanStorage := t.storageProvider.GetTraceStorage(ctx, param.WorkSpaceID)
	annoDao := t.annoDaos[spanStorage.StorageName]
	if annoDao == nil {
		return errorx.WrapByCode(errors.New("invalid storage"), obErrorx.CommercialCommonInvalidParamCodeCode)
	}

	table, err := t.getAnnoInsertTable(ctx, param.Tenant, param.TTL)
	if err != nil {
		return err
	}
	pos := make([]*dao.Annotation, 0, len(param.Annotations))
	for _, annotation := range param.Annotations {
		annotationPO, err := converter.AnnotationDO2PO(annotation)
		if err != nil {
			return err
		}
		pos = append(pos, annotationPO)
	}
	return annoDao.Insert(ctx, &dao.InsertAnnotationParam{
		Table:       table,
		Annotations: pos,
		Extra:       spanStorage.StorageConfig,
	})
}

func (t *TraceRepoImpl) GetMetrics(ctx context.Context, param *metric_repo.GetMetricsParam) (*metric_repo.GetMetricsResult, error) {
	spanStorage := t.storageProvider.GetTraceStorage(ctx, param.WorkSpaceID)
	spanDao := t.spanDaos[spanStorage.StorageName]
	if spanDao == nil {
		return nil, errorx.WrapByCode(errors.New("invalid storage"), obErrorx.CommercialCommonInvalidParamCodeCode)
	}

	tableCfg, err := t.getQueryTenantTables(ctx, param.Tenants)
	if err != nil {
		return nil, err
	}
	st := time.Now()
	metrics, err := spanDao.GetMetrics(ctx, &dao.GetMetricsParam{
		Tables:       tableCfg.SpanTables,
		Aggregations: param.Aggregations,
		GroupBys:     param.GroupBys,
		Filters:      param.Filters,
		StartAt:      time_util.MillSec2MicroSec(param.StartAt),
		EndAt:        time_util.MillSec2MicroSec(param.EndAt),
		Granularity:  param.Granularity,
		Extra:        spanStorage.StorageConfig,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "get metrics successfully, cost %v", time.Since(st))
	return &metric_repo.GetMetricsResult{
		Data: metrics,
	}, nil
}

type queryTableCfg struct {
	SpanTables    []string
	AnnoTables    []string
	AnnoTableMap  map[string]string
	NeedQueryAnno bool
}

func (t *TraceRepoImpl) getQueryTenantTables(ctx context.Context, tenants []string) (*queryTableCfg, error) {
	tenantTableCfg, err := t.traceConfig.GetTenantConfig(ctx)
	if err != nil {
		logs.CtxError(ctx, "fail to get tenant table config, %v", err)
		return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	if len(tenants) == 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("no tenants configured"))
	}
	ret := &queryTableCfg{
		SpanTables:   make([]string, 0),
		AnnoTableMap: make(map[string]string),
	}
	for _, tenant := range tenants {
		tables, ok := tenantTableCfg.TenantTables[tenant]
		if !ok {
			continue
		}
		for _, tableCfg := range tables {
			ret.SpanTables = append(ret.SpanTables, tableCfg.SpanTable)
			if tableCfg.AnnoTable != "" {
				ret.AnnoTables = append(ret.AnnoTables, tableCfg.AnnoTable)
				ret.AnnoTableMap[tableCfg.SpanTable] = tableCfg.AnnoTable
			}
		}
	}
	for _, tenant := range tenants {
		if tenantTableCfg.TenantsSupportAnnotation[tenant] {
			ret.NeedQueryAnno = true
			break
		}
	}
	ret.SpanTables = lo.Uniq(ret.SpanTables)
	ret.AnnoTables = lo.Uniq(ret.AnnoTables)
	return ret, nil
}

func (t *TraceRepoImpl) getSpanInsertTable(ctx context.Context, tenant string, ttl loop_span.TTL) (string, error) {
	tenantTableCfg, err := t.traceConfig.GetTenantConfig(ctx)
	if err != nil {
		logs.CtxError(ctx, "fail to get tenant config, %v", err)
		return "", err
	}
	tableCfg, ok := tenantTableCfg.TenantTables[tenant][ttl]
	if !ok {
		return "", fmt.Errorf("no table config found for tenant %s with ttl %s", tenant, ttl)
	} else if tableCfg.SpanTable == "" {
		return "", fmt.Errorf("no table config found for tenant %s with ttl %s", tenant, ttl)
	}
	return tableCfg.SpanTable, nil
}

func (t *TraceRepoImpl) getAnnoInsertTable(ctx context.Context, tenant string, ttl loop_span.TTL) (string, error) {
	tenantTableCfg, err := t.traceConfig.GetTenantConfig(ctx)
	if err != nil {
		logs.CtxError(ctx, "fail to get tenant config, %v", err)
		return "", err
	}
	tableCfg, ok := tenantTableCfg.TenantTables[tenant][ttl]
	if !ok {
		return "", fmt.Errorf("no annotation table config found for tenant %s with ttl %s", tenant, ttl)
	} else if tableCfg.AnnoTable == "" {
		return "", fmt.Errorf("no annotation table config found for tenant %s with ttl %s", tenant, ttl)
	}
	return tableCfg.AnnoTable, nil
}

func (t *TraceRepoImpl) addPageTokenFilter(pageToken *PageToken, filter *loop_span.FilterFields) *loop_span.FilterFields {
	timeStr := strconv.FormatInt(pageToken.StartTime, 10)
	filterFields := &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
		FilterFields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldStartTime,
				FieldType: loop_span.FieldTypeLong,
				Values:    []string{timeStr},
				QueryType: ptr.Of(loop_span.QueryTypeEnumLt),
			},
			{
				FieldName:  loop_span.SpanFieldStartTime,
				FieldType:  loop_span.FieldTypeLong,
				Values:     []string{timeStr},
				QueryType:  ptr.Of(loop_span.QueryTypeEnumEq),
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
				SubFilter: &loop_span.FilterFields{
					QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
					FilterFields: []*loop_span.FilterField{
						{
							FieldName: loop_span.SpanFieldSpanId,
							FieldType: loop_span.FieldTypeString,
							Values:    []string{pageToken.SpanID},
							QueryType: ptr.Of(loop_span.QueryTypeEnumLt),
						},
					},
				},
			},
		},
	}
	if filter == nil {
		return filterFields
	} else {
		return &loop_span.FilterFields{
			QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
			FilterFields: []*loop_span.FilterField{
				{
					SubFilter: filterFields,
				},
				{
					SubFilter: filter,
				},
			},
		}
	}
}

func parsePageToken(pageToken string) (*PageToken, error) {
	if pageToken == "" {
		return nil, nil
	}
	ptStr, err := base64.StdEncoding.DecodeString(pageToken)
	if err != nil {
		return nil, fmt.Errorf("fail to decode pageToken %s, %v", pageToken, err)
	}
	pt := new(PageToken)
	if err := json.Unmarshal(ptStr, pt); err != nil {
		return nil, fmt.Errorf("fail to unmarshal pageToken %s, %v", string(ptStr), err)
	}
	return pt, nil
}
