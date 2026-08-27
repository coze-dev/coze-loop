// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	filtermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/pagetoken"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func newThreadChatSvc(ctrl *gomock.Controller, repoMock repo.ITraceRepo) ITraceService {
	filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
	buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, map[entity.ProcessorScene][]span_processor.Factory{entity.SceneThreadChat: {}})
	tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
	tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
	r, _ := NewTraceServiceImpl(repoMock, nil, nil, nil, nil, buildHelper, tenantProviderMock, nil, nil, nil)
	return r
}

// modelSpan 构造一个只带 Input 的 model span，buildChatMessages 会展开成正好 1 条 user message，便于按 SpanID 校验顺序。
func modelSpan(id string, startUs int64) *loop_span.Span {
	return &loop_span.Span{
		SpanID:    id,
		SpanType:  loop_span.SpanTypeModel,
		StartTime: startUs,
		Input:     "in-" + id,
	}
}

func msgSpanIDs(msgs []*entity.ChatMessage) []string {
	ids := make([]string, 0, len(msgs))
	for _, m := range msgs {
		ids = append(ids, m.Span.SpanID)
	}
	return ids
}

func TestListThreadChat_FirstPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repoMock := repomocks.NewMockITraceRepo(ctrl)
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *repo.ListSpansParam) (*repo.ListSpansResult, error) {
			assert.True(t, p.AscByStartTime)
			assert.False(t, p.DescByStartTime)
			assert.Empty(t, p.PageToken)
			assert.Equal(t, defaultChatPageSize, p.Limit)
			return &repo.ListSpansResult{
				Spans:     loop_span.SpanList{modelSpan("s1", 100), modelSpan("s2", 200)},
				PageToken: "next-tok",
				HasMore:   true,
			}, nil
		})
	svc := newThreadChatSvc(ctrl, repoMock)
	resp, err := svc.ListThreadChat(context.Background(), &ListThreadChatRequest{WorkspaceID: 1, ThreadID: "t"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"s1", "s2"}, msgSpanIDs(resp.Messages))
	assert.Equal(t, "next-tok", resp.NextPageToken)
	assert.True(t, resp.HasMore)
	// 首页无历史方向。
	assert.Empty(t, resp.PrevPageToken)
	assert.False(t, resp.PrevHasMore)
}

func TestListThreadChat_ForwardPaging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repoMock := repomocks.NewMockITraceRepo(ctrl)
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *repo.ListSpansParam) (*repo.ListSpansResult, error) {
			assert.True(t, p.AscByStartTime)
			assert.Equal(t, "cur", p.PageToken)
			return &repo.ListSpansResult{
				Spans:     loop_span.SpanList{modelSpan("s3", 300)},
				PageToken: "next2",
				HasMore:   false,
			}, nil
		})
	svc := newThreadChatSvc(ctrl, repoMock)
	resp, err := svc.ListThreadChat(context.Background(), &ListThreadChatRequest{WorkspaceID: 1, ThreadID: "t", PageToken: "cur"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"s3"}, msgSpanIDs(resp.Messages))
	assert.Equal(t, "next2", resp.NextPageToken)
	// 向后翻页时历史方向必有更多；prev 游标 = 本页时间序最早 span，由 service 自行编码。
	assert.Equal(t, pagetoken.Encode(300, "s3"), resp.PrevPageToken)
	assert.True(t, resp.PrevHasMore)
}

func TestListThreadChat_BackwardPaging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repoMock := repomocks.NewMockITraceRepo(ctrl)
	// desc 查询返回 newest->oldest，service 需反转成时间序。
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *repo.ListSpansParam) (*repo.ListSpansResult, error) {
			assert.True(t, p.DescByStartTime)
			assert.False(t, p.AscByStartTime)
			assert.Equal(t, "prevcur", p.PageToken)
			return &repo.ListSpansResult{
				Spans:     loop_span.SpanList{modelSpan("s9", 900), modelSpan("s8", 800)},
				PageToken: "older-tok", // desc 末条(最旧) = 向前游标
				HasMore:   true,        // 向前还有更旧
			}, nil
		})
	svc := newThreadChatSvc(ctrl, repoMock)
	resp, err := svc.ListThreadChat(context.Background(), &ListThreadChatRequest{WorkspaceID: 1, ThreadID: "t", PrevPageToken: "prevcur"})
	assert.NoError(t, err)
	// 反转成时间序：s8(800) 在前，s9(900) 在后。
	assert.Equal(t, []string{"s8", "s9"}, msgSpanIDs(resp.Messages))
	assert.Equal(t, "older-tok", resp.PrevPageToken)
	assert.True(t, resp.PrevHasMore)
	// next 游标 = 反转后时间序最新 span(s9)，由 service 自行编码。
	assert.Equal(t, pagetoken.Encode(900, "s9"), resp.NextPageToken)
}

func TestListThreadChat_Anchor_Enough(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repoMock := repomocks.NewMockITraceRepo(ctrl)
	// 锚点前后两次 ListSpans 并发执行，按 desc/asc 方向分流断言（不依赖调用顺序）。
	// 锚点 250ms → 250000us，(start_time, span_id) 编码为 keyset 游标下发给两次查询。
	wantAnchor := pagetoken.Encode(250000, "anchor-span")
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *repo.ListSpansParam) (*repo.ListSpansResult, error) {
			assert.Equal(t, wantAnchor, p.PageToken)
			assert.Equal(t, int32(25), p.Limit) // pageSize/2 = 50/2
			if p.DescByStartTime {
				assert.False(t, p.PageTokenInclusive) // 向前段严格 <，不含锚点
				return &repo.ListSpansResult{
					Spans:     loop_span.SpanList{modelSpan("b2", 200), modelSpan("b1", 100)},
					PageToken: "back-older",
					HasMore:   true,
				}, nil
			}
			assert.True(t, p.AscByStartTime)
			assert.True(t, p.PageTokenInclusive) // 向后段含锚点自身
			return &repo.ListSpansResult{
				Spans:     loop_span.SpanList{modelSpan("f1", 300), modelSpan("f2", 400)},
				PageToken: "fwd-next",
				HasMore:   true,
			}, nil
		}).Times(2)
	svc := newThreadChatSvc(ctrl, repoMock)
	resp, err := svc.ListThreadChat(context.Background(), &ListThreadChatRequest{
		WorkspaceID: 1, ThreadID: "t", AnchorStartTime: 250, AnchorSpanID: "anchor-span",
	})
	assert.NoError(t, err)
	// 向前段反转(b1,b2) + 向后段(f1,f2) 合并时间序。
	assert.Equal(t, []string{"b1", "b2", "f1", "f2"}, msgSpanIDs(resp.Messages))
	assert.Equal(t, "back-older", resp.PrevPageToken)
	assert.True(t, resp.PrevHasMore)
	assert.Equal(t, "fwd-next", resp.NextPageToken)
	assert.True(t, resp.HasMore)
}

func TestListThreadChat_Anchor_Boundary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repoMock := repomocks.NewMockITraceRepo(ctrl)
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *repo.ListSpansParam) (*repo.ListSpansResult, error) {
			// 向前段到最早边界：无数据 → prev token 空、prev has_more false。
			if p.DescByStartTime {
				return &repo.ListSpansResult{Spans: loop_span.SpanList{}, PageToken: "", HasMore: false}, nil
			}
			assert.True(t, p.AscByStartTime)
			return &repo.ListSpansResult{
				Spans:     loop_span.SpanList{modelSpan("f1", 300)},
				PageToken: "fwd-next", HasMore: false,
			}, nil
		}).Times(2)
	svc := newThreadChatSvc(ctrl, repoMock)
	resp, err := svc.ListThreadChat(context.Background(), &ListThreadChatRequest{
		WorkspaceID: 1, ThreadID: "t", AnchorStartTime: 250, AnchorSpanID: "anchor-span",
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"f1"}, msgSpanIDs(resp.Messages))
	assert.Empty(t, resp.PrevPageToken)
	assert.False(t, resp.PrevHasMore)
}

func TestListThreadChat_FiltersAndWithoutDetail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repoMock := repomocks.NewMockITraceRepo(ctrl)
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, p *repo.ListSpansParam) (*repo.ListSpansResult, error) {
			// without_detail 放大分页 + 省略正文列。
			assert.Equal(t, maxChatPageSizeWithoutDetail, p.Limit)
			assert.ElementsMatch(t, []string{"input", "output"}, p.OmitColumns)
			// filters 作为一个 SubFilter 并入基础 filter。
			var hasSub bool
			for _, f := range p.Filters.FilterFields {
				if f.SubFilter != nil {
					hasSub = true
				}
			}
			assert.True(t, hasSub)
			// without_detail 下 span 无正文，buildChatMessages 每 span 产 1 条空壳。
			return &repo.ListSpansResult{Spans: loop_span.SpanList{
				{SpanID: "m1", SpanType: loop_span.SpanTypeModel, StartTime: 100},
			}}, nil
		})
	svc := newThreadChatSvc(ctrl, repoMock)
	resp, err := svc.ListThreadChat(context.Background(), &ListThreadChatRequest{
		WorkspaceID:   1,
		ThreadID:      "t",
		WithoutDetail: true,
		Filters: &loop_span.FilterFields{
			FilterFields: []*loop_span.FilterField{
				{FieldName: loop_span.SpanFieldOutput, FieldType: loop_span.FieldTypeString, Values: []string{"kw"}},
			},
		},
	})
	assert.NoError(t, err)
	assert.Len(t, resp.Messages, 1)
	assert.Equal(t, "m1", resp.Messages[0].Span.SpanID)
}

func TestListThreadChat_NoHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repoMock := repomocks.NewMockITraceRepo(ctrl)
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{}}, nil)
	svc := newThreadChatSvc(ctrl, repoMock)
	resp, err := svc.ListThreadChat(context.Background(), &ListThreadChatRequest{
		WorkspaceID: 1, ThreadID: "t",
		Filters: &loop_span.FilterFields{FilterFields: []*loop_span.FilterField{
			{FieldName: loop_span.SpanFieldOutput, Values: []string{"nomatch"}},
		}},
	})
	assert.NoError(t, err)
	assert.Empty(t, resp.Messages)
	assert.False(t, resp.HasMore)
}
