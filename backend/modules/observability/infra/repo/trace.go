// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	time_util "github.com/coze-dev/coze-loop/backend/pkg/time"
	"github.com/samber/lo"
)

func NewTraceCKRepoImpl(
	spanDao ck.ISpansDao,
	annoDao ck.IAnnotationDao,
	traceConfig config.ITraceConfig,
) (repo.ITraceRepo, error) {
	return &TraceCkRepoImpl{
		spansDao:    spanDao,
		annoDao:     annoDao,
		traceConfig: traceConfig,
	}, nil
}

type TraceCkRepoImpl struct {
	spansDao    ck.ISpansDao
	annoDao     ck.IAnnotationDao
	traceConfig config.ITraceConfig
}

type PageToken struct {
	StartTime int64  `json:"StartTime"`
	SpanID    string `json:"SpanID"`
}

func (t *TraceCkRepoImpl) InsertSpans(ctx context.Context, param *repo.InsertTraceParam) error {
	table, err := t.getSpanInsertTable(ctx, param.Tenant, param.TTL)
	if err != nil {
		return err
	}
	if err := t.spansDao.Insert(ctx, &ck.InsertParam{
		Table: table,
		Spans: convertor.SpanListDO2PO(param.Spans, param.TTL),
	}); err != nil {
		logs.CtxError(ctx, "fail to insert spans, %v", err)
		return err
	}
	logs.CtxInfo(ctx, "insert spans into table %s successfully, count %d", table, len(param.Spans))
	return nil
}

func (t *TraceCkRepoImpl) ListSpans(ctx context.Context, req *repo.ListSpansParam) (*repo.ListSpansResult, error) {
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
	spans, err := t.spansDao.Get(ctx, &ck.QueryParam{
		QueryType:        ck.QueryTypeListSpans,
		Tables:           tableCfg.SpanTables,
		AnnoTableMap:     tableCfg.AnnoTableMap,
		StartTime:        time_util.MillSec2MicroSec(req.StartAt),
		EndTime:          time_util.MillSec2MicroSec(req.EndAt),
		Filters:          req.Filters,
		Limit:            req.Limit + 1,
		OrderByStartTime: req.DescByStartTime,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "list spans successfully, spans count %d, cost %v", len(spans), time.Since(st))
	spanDOList := convertor.SpanListPO2DO(spans)
	if tableCfg.NeedQueryAnno && !req.NotQueryAnnotation {
		spanIDs := lo.UniqMap(spans, func(item *model.ObservabilitySpan, _ int) string {
			return item.SpanID
		})
		st = time.Now()
		annotations, err := t.annoDao.List(ctx, &ck.ListAnnotationsParam{
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
		annoDOList := convertor.AnnotationListPO2DO(annotations)
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

func (t *TraceCkRepoImpl) GetTrace(ctx context.Context, req *repo.GetTraceParam) (loop_span.SpanList, error) {
	tableCfg, err := t.getQueryTenantTables(ctx, req.Tenants)
	if err != nil {
		return nil, err
	}
	var filterFields []*loop_span.FilterField
	filterFields = append(filterFields, &loop_span.FilterField{
		FieldName: loop_span.SpanFieldTraceId,
		FieldType: loop_span.FieldTypeString,
		Values:    []string{req.TraceID},
		QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
	})
	if len(req.SpanIDs) > 0 {
		filterFields = append(filterFields, &loop_span.FilterField{
			FieldName: loop_span.SpanFieldSpanId,
			FieldType: loop_span.FieldTypeString,
			Values:    req.SpanIDs,
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		})
	}
	filter := &loop_span.FilterFields{
		QueryAndOr:   ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: filterFields,
	}
	st := time.Now()
	spans, err := t.spansDao.Get(ctx, &ck.QueryParam{
		QueryType:    ck.QueryTypeGetTrace,
		Tables:       tableCfg.SpanTables,
		AnnoTableMap: tableCfg.AnnoTableMap,
		StartTime:    time_util.MillSec2MicroSec(req.StartAt),
		EndTime:      time_util.MillSec2MicroSec(req.EndAt),
		Filters:      filter,
		Limit:        req.Limit,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "get trace %s successfully, spans count %d, cost %v",
		req.TraceID, len(spans), time.Since(st))
	spanDOList := convertor.SpanListPO2DO(spans)
	if tableCfg.NeedQueryAnno && !req.NotQueryAnnotation {
		spanIDs := lo.UniqMap(spans, func(item *model.ObservabilitySpan, _ int) string {
			return item.SpanID
		})
		st = time.Now()
		annotations, err := t.annoDao.List(ctx, &ck.ListAnnotationsParam{
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
		annoDOList := convertor.AnnotationListPO2DO(annotations)
		spanDOList.SetAnnotations(annoDOList.Uniq())
	}
	return spanDOList.Uniq(), nil
}

func (t *TraceCkRepoImpl) ListAnnotations(ctx context.Context, param *repo.ListAnnotationsParam) (loop_span.AnnotationList, error) {
	if param.SpanID == "" || param.TraceID == "" || param.WorkspaceId <= 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	tableCfg, err := t.getQueryTenantTables(ctx, param.Tenants)
	if err != nil {
		return nil, err
	}
	st := time.Now()
	annotations, err := t.annoDao.List(ctx, &ck.ListAnnotationsParam{
		Tables:          tableCfg.AnnoTables,
		SpanIDs:         []string{param.SpanID},
		StartTime:       time_util.MillSec2MicroSec(param.StartAt),
		EndTime:         time_util.MillSec2MicroSec(param.EndAt),
		DescByUpdatedAt: param.DescByUpdatedAt,
		Limit:           100,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "get annotations successfully, annotations count %d, cost %v", len(annotations), time.Since(st))
	workspaceIDStr := strconv.FormatInt(param.WorkspaceId, 10)
	annotations = lo.Filter(annotations, func(item *model.ObservabilityAnnotation, _ int) bool {
		return item.TraceID == param.TraceID && item.SpaceID == workspaceIDStr
	})
	return convertor.AnnotationListPO2DO(annotations).Uniq(), nil
}

func (t *TraceCkRepoImpl) GetAnnotation(ctx context.Context, param *repo.GetAnnotationParam) (*loop_span.Annotation, error) {
	tableCfg, err := t.getQueryTenantTables(ctx, param.Tenants)
	if err != nil {
		return nil, err
	}
	st := time.Now()
	annotation, err := t.annoDao.Get(ctx, &ck.GetAnnotationParam{
		Tables:    tableCfg.AnnoTables,
		ID:        param.ID,
		StartTime: time_util.MillSec2MicroSec(param.StartAt),
		EndTime:   time_util.MillSec2MicroSec(param.EndAt),
		Limit:     2,
	})
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "get annotation successfully, cost %v", time.Since(st))
	return convertor.AnnotationPO2DO(annotation), nil
}

func (t *TraceCkRepoImpl) InsertAnnotation(ctx context.Context, param *repo.InsertAnnotationParam) error {
	table, err := t.getAnnoInsertTable(ctx, param.Tenant, param.TTL)
	if err != nil {
		return err
	}
	annotationPO, err := convertor.AnnotationDO2PO(param.Annotation)
	if err != nil {
		return err
	}
	return t.annoDao.Insert(ctx, &ck.InsertAnnotationParam{
		Table:      table,
		Annotation: annotationPO,
	})
}

type queryTableCfg struct {
	SpanTables    []string
	AnnoTables    []string
	AnnoTableMap  map[string]string
	NeedQueryAnno bool
}

func (t *TraceCkRepoImpl) getQueryTenantTables(ctx context.Context, tenants []string) (*queryTableCfg, error) {
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
			ret.AnnoTables = append(ret.AnnoTables, tableCfg.AnnoTable)
			ret.AnnoTableMap[tableCfg.SpanTable] = tableCfg.AnnoTable
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

func (t *TraceCkRepoImpl) getSpanInsertTable(ctx context.Context, tenant string, ttl loop_span.TTL) (string, error) {
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

func (t *TraceCkRepoImpl) getAnnoInsertTable(ctx context.Context, tenant string, ttl loop_span.TTL) (string, error) {
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

func (t *TraceCkRepoImpl) addPageTokenFilter(pageToken *PageToken, filter *loop_span.FilterFields) *loop_span.FilterFields {
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
