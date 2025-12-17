// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	filtermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestTraceServiceImpl_GetTrajectoryConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repoMock := repomocks.NewMockITraceRepo(ctrl)
	// case: config nil
	repoMock.EXPECT().GetTrajectoryConfig(gomock.Any(), repo.GetTrajectoryConfigParam{WorkspaceId: 1}).Return(nil, nil)
	svc := &TraceServiceImpl{traceRepo: repoMock}
	resp, err := svc.GetTrajectoryConfig(context.Background(), &GetTrajectoryConfigRequest{WorkspaceID: 1})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Nil(t, resp.Filters)
	// case: config exists but filter nil
	repoMock.EXPECT().GetTrajectoryConfig(gomock.Any(), repo.GetTrajectoryConfigParam{WorkspaceId: 2}).Return(&entity.TrajectoryConfig{Filter: nil}, nil)
	resp, err = svc.GetTrajectoryConfig(context.Background(), &GetTrajectoryConfigRequest{WorkspaceID: 2})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Nil(t, resp.Filters)
	// case: normal
	f := &loop_span.FilterFields{FilterFields: []*loop_span.FilterField{{FieldName: loop_span.SpanFieldSpanType, FieldType: loop_span.FieldTypeString, Values: []string{"agent"}, QueryType: ptr.Of(loop_span.QueryTypeEnumIn)}}}
	repoMock.EXPECT().GetTrajectoryConfig(gomock.Any(), repo.GetTrajectoryConfigParam{WorkspaceId: 3}).Return(&entity.TrajectoryConfig{Filter: f}, nil)
	resp, err = svc.GetTrajectoryConfig(context.Background(), &GetTrajectoryConfigRequest{WorkspaceID: 3})
	assert.NoError(t, err)
	assert.Equal(t, f, resp.Filters)
}

func TestTraceServiceImpl_UpsertTrajectoryConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repoMock := repomocks.NewMockITraceRepo(ctrl)
	svc := &TraceServiceImpl{traceRepo: repoMock}
	filters := &loop_span.FilterFields{FilterFields: []*loop_span.FilterField{{FieldName: loop_span.SpanFieldSpanType, FieldType: loop_span.FieldTypeString, Values: []string{"model"}, QueryType: ptr.Of(loop_span.QueryTypeEnumIn)}}}
	repoMock.EXPECT().UpsertTrajectoryConfig(gomock.Any(), gomock.Any()).Return(nil)
	err := svc.UpsertTrajectoryConfig(context.Background(), &UpsertTrajectoryConfigRequest{WorkspaceID: 11, Filters: filters, UserID: "u"})
	assert.NoError(t, err)
}

func TestTraceServiceImpl_convertCustomNode(t *testing.T) {
	svc := &TraceServiceImpl{}
	spans := loop_span.SpanList{
		{SpanID: "1", SpanType: "graph"},
		{SpanID: "2", SpanType: "agent"},
		{SpanID: "3", SpanType: "tool"},
	}
	res := svc.convertCustomNode(spans)
	// graph -> agent
	assert.Equal(t, "agent", res[0].SpanType)
	assert.Equal(t, "agent", res[1].SpanType)
	assert.Equal(t, "tool", res[2].SpanType)
}

func TestTraceServiceImpl_getEvalTargetNextLevelSpanID(t *testing.T) {
	svc := &TraceServiceImpl{}
	all := &repo.ListSpansResult{Spans: loop_span.SpanList{
		{SpanID: "a", SpanName: "EvalTarget"},
		{SpanID: "b", ParentID: "a"},
		{SpanID: "c", ParentID: "x"},
		{SpanID: "d", ParentID: "a"},
	}}
	ids := svc.getEvalTargetNextLevelSpanID(all)
	assert.ElementsMatch(t, []string{"b", "d"}, ids)
}

func TestTraceServiceImpl_getSelectFilters(t *testing.T) {
	svc := &TraceServiceImpl{}
	traceIDs := []string{"t1", "t2"}
	cfg := (&GetTrajectoryConfigResponse{Filters: &loop_span.FilterFields{FilterFields: []*loop_span.FilterField{{FieldName: loop_span.SpanFieldSpanType, FieldType: loop_span.FieldTypeString, Values: []string{"agent"}, QueryType: ptr.Of(loop_span.QueryTypeEnumIn)}}}}).GetFiltersWithDefaultFilter()
	all := &repo.ListSpansResult{Spans: loop_span.SpanList{
		{SpanID: "a", SpanName: "EvalTarget"},
		{SpanID: "b", ParentID: "a"},
	}}
	filters := svc.getSelectFilters(traceIDs, &GetTrajectoryConfigResponse{Filters: cfg}, all)
	// top-level OR, contains two subfilters: trace+trajectory rule AND; trace+lowSpanIDs AND
	assert.Equal(t, loop_span.QueryAndOrEnumOr, *filters.QueryAndOr)
	assert.GreaterOrEqual(t, len(filters.FilterFields), 1)
}

func TestTraceServiceImpl_buildTrajectories(t *testing.T) {
	svc := &TraceServiceImpl{}
	traceID := "trace-1"
	root := &loop_span.Span{TraceID: traceID, SpanID: "root", ParentID: "0", SpanType: "agent", SpanName: "root-agent"}
	child1 := &loop_span.Span{TraceID: traceID, SpanID: "m1", ParentID: "root", SpanType: "model", SpanName: "model-1"}
	child2 := &loop_span.Span{TraceID: traceID, SpanID: "t1", ParentID: "root", SpanType: "tool", SpanName: "tool-1"}
	all := loop_span.SpanList{root, child1, child2}
	selected := loop_span.SpanList{child1}
	trajectoryRule := &loop_span.FilterFields{FilterFields: []*loop_span.FilterField{{FieldName: loop_span.SpanFieldSpanType, FieldType: loop_span.FieldTypeString, Values: []string{"agent", "model", "tool"}, QueryType: ptr.Of(loop_span.QueryTypeEnumIn)}}}
	res, err := svc.buildTrajectories(context.Background(), &all, &selected, trajectoryRule)
	assert.NoError(t, err)
	traj := res[traceID]
	assert.NotNil(t, traj)
	assert.NotNil(t, traj.RootStep)
	assert.Equal(t, "root-agent", *traj.RootStep.Name)
	assert.Equal(t, 1, len(traj.AgentSteps))
}

func TestTraceServiceImpl_GetTrajectories_and_ListTrajectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repoMock := repomocks.NewMockITraceRepo(ctrl)
	filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
	builder := NewTraceFilterProcessorBuilder(filterFactoryMock, []span_processor.Factory{span_processor.NewCheckProcessorFactory()}, nil, nil, nil, nil, nil)
	tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
	tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"tenant"}, nil).AnyTimes()
	// 配置查询：返回空，走默认规则
	repoMock.EXPECT().GetTrajectoryConfig(gomock.Any(), repo.GetTrajectoryConfigParam{WorkspaceId: 1}).Return(nil, nil).AnyTimes()

	svc := &TraceServiceImpl{traceRepo: repoMock, buildHelper: builder, tenantProvider: tenantProviderMock}
	// mock list all spans
	traceIDs := []string{"tid"}
	repoMock.EXPECT().ListSpansRepeat(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *repo.ListSpansParam) (*repo.ListSpansResult, error) {
		// 第一次：allSpans（带 SelectColumns），第二次：selectedSpans（不带 SelectColumns）
		return &repo.ListSpansResult{Spans: loop_span.SpanList{
			{TraceID: "tid", SpanID: "root", ParentID: "0", WorkspaceID: "1", SpanName: "root", SpanType: "agent"},
			{TraceID: "tid", SpanID: "m", ParentID: "root", WorkspaceID: "1", SpanName: "model", SpanType: "model"},
		}}, nil
	}).AnyTimes()
	// GetTrajectories
	res, err := svc.GetTrajectories(context.Background(), 1, traceIDs, time.Now().Add(-time.Minute).UnixMilli(), time.Now().UnixMilli(), loop_span.PlatformCozeLoop)
	assert.NoError(t, err)
	assert.NotNil(t, res["tid"])
	// ListTrajectory
	start := time.Now().Add(-time.Minute).UnixMilli()
	lt, err := svc.ListTrajectory(context.Background(), &ListTrajectoryRequest{PlatformType: loop_span.PlatformCozeLoop, WorkspaceID: 1, TraceIds: traceIDs, StartTime: &start})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(lt.Trajectories))
}
