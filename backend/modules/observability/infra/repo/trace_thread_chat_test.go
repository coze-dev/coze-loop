// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
	daomock "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/pagetoken"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// containsKeysetFilter 递归检查 filter 树里是否含以给定 start_time 为值的 keyset 条件（addPageTokenFilter 产物）。
func containsKeysetFilter(ff *loop_span.FilterFields, startTimeStr string) bool {
	if ff == nil {
		return false
	}
	for _, f := range ff.FilterFields {
		if f.FieldName == loop_span.SpanFieldStartTime && len(f.Values) == 1 && f.Values[0] == startTimeStr {
			return true
		}
		if containsKeysetFilter(f.SubFilter, startTimeStr) {
			return true
		}
	}
	return false
}

// spanIDKeysetQueryType 递归找 keyset 里 span_id 子条件的 QueryType（addPageTokenFilter 复合游标的第二支），找不到返回空。
func spanIDKeysetQueryType(ff *loop_span.FilterFields) loop_span.QueryTypeEnum {
	if ff == nil {
		return ""
	}
	for _, f := range ff.FilterFields {
		if f.FieldName == loop_span.SpanFieldSpanId && f.QueryType != nil {
			return *f.QueryType
		}
		if qt := spanIDKeysetQueryType(f.SubFilter); qt != "" {
			return qt
		}
	}
	return ""
}

func newListSpansRepo(t *testing.T, ctrl *gomock.Controller, spansDao dao.ISpansDao) *TraceRepoImpl {
	traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
	traceConfigMock.EXPECT().GetTenantConfig(gomock.Any()).Return(&config.TenantCfg{
		TenantTables: map[string]map[loop_span.TTL]config.TableCfg{
			"test": {loop_span.TTL3d: {SpanTable: "spans"}},
		},
	}, nil).AnyTimes()
	r, err := NewTraceRepoImpl(
		traceConfigMock,
		&mockStorageProvider{},
		nil, nil, nil, nil,
		nil,
		WithTraceStorageDaos("ck", spansDao, daomock.NewMockIAnnotationDao(ctrl)),
	)
	assert.NoError(t, err)
	return r.(*TraceRepoImpl)
}

// repo 只认 PageToken：service 把锚点坐标编码成 token 传入，repo 走同一 keyset 过滤，只产出末条 PageToken。
func TestListSpans_PageTokenKeysetAndNextToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	spansDaoMock := daomock.NewMockISpansDao(ctrl)
	// Limit=2 → 底层取 3 条探 HasMore；返回 3 条，末条被裁剪。
	spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *dao.QueryParam) ([]*dao.Span, error) {
			// 传入 token 的 start_time(us=250000) 应作为 keyset 过滤下推。
			assert.True(t, containsKeysetFilter(p.Filters, "250000"))
			assert.Equal(t, int32(3), p.Limit) // req.Limit(2)+1
			return []*dao.Span{
				{SpanID: "a", StartTime: 100},
				{SpanID: "b", StartTime: 200},
				{SpanID: "c", StartTime: 300},
			}, nil
		})
	r := newListSpansRepo(t, ctrl, spansDaoMock)

	got, err := r.ListSpans(context.Background(), &repo.ListSpansParam{
		Tenants:        []string{"test"},
		Limit:          2,
		AscByStartTime: true,
		PageToken:      pagetoken.Encode(250000, "anchor"), // us
	})
	assert.NoError(t, err)
	assert.True(t, got.HasMore)
	assert.Len(t, got.Spans, 2) // 裁掉末条
	// PageToken = 保留末条坐标。
	next, err := pagetoken.Decode(got.PageToken)
	assert.NoError(t, err)
	assert.Equal(t, int64(200), next.StartTime)
	assert.Equal(t, "b", next.SpanID)
}

func TestListSpans_EmptyResultNoToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	spansDaoMock := daomock.NewMockISpansDao(ctrl)
	spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).Return([]*dao.Span{}, nil)
	r := newListSpansRepo(t, ctrl, spansDaoMock)

	got, err := r.ListSpans(context.Background(), &repo.ListSpansParam{
		Tenants: []string{"test"}, Limit: 2, DescByStartTime: true,
	})
	assert.NoError(t, err)
	assert.False(t, got.HasMore)
	assert.Empty(t, got.PageToken)
}

// PageTokenInclusive 控制 keyset 边界：默认严格(asc→gt)，inclusive 时 span_id 子条件放宽(asc→gte)，锚点自身被查出。
func TestListSpans_PageTokenInclusiveBoundary(t *testing.T) {
	tests := []struct {
		name      string
		inclusive bool
		wantQT    loop_span.QueryTypeEnum
	}{
		{name: "strict_gt", inclusive: false, wantQT: loop_span.QueryTypeEnumGt},
		{name: "inclusive_gte", inclusive: true, wantQT: loop_span.QueryTypeEnumGte},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			spansDaoMock := daomock.NewMockISpansDao(ctrl)
			spansDaoMock.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, p *dao.QueryParam) ([]*dao.Span, error) {
					assert.Equal(t, tt.wantQT, spanIDKeysetQueryType(p.Filters))
					return []*dao.Span{}, nil
				})
			r := newListSpansRepo(t, ctrl, spansDaoMock)
			_, err := r.ListSpans(context.Background(), &repo.ListSpansParam{
				Tenants:            []string{"test"},
				Limit:              2,
				AscByStartTime:     true,
				PageToken:          pagetoken.Encode(250000, "anchor"),
				PageTokenInclusive: tt.inclusive,
			})
			assert.NoError(t, err)
		})
	}
}
