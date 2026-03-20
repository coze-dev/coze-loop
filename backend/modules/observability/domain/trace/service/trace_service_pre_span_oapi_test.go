// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	filtermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestTraceServiceImpl_ListPreSpanOApi_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITraceRepo(ctrl)
	filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
	buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)

	// 预先从Redis取到两个pre响应顺序
	repoMock.EXPECT().GetPreSpanIDs(gomock.Any(), gomock.Any()).Return(
		[]string{"pre-span-1", "pre-span-2"},
		[]string{"resp-2", "resp-1"},
		nil,
	)
	// batch从CK取到当前span和两个pre span
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ *repo.ListSpansParam) (*repo.ListSpansResult, error) {
			return &repo.ListSpansResult{
				Spans: loop_span.SpanList{
					// current span（校验使用previous_response_id）
					{
						TraceID:     "trace-1",
						SpanID:      "cur-span",
						WorkspaceID: "1",
						SystemTagsString: map[string]string{
							"previous_response_id": "prev-1",
						},
					},
					// pre span with response_id = resp-1
					{
						TraceID:     "trace-1",
						SpanID:      "pre-span-1",
						WorkspaceID: "1",
						SystemTagsString: map[string]string{
							"response_id": "resp-1",
						},
					},
					// pre span with response_id = resp-2
					{
						TraceID:     "trace-1",
						SpanID:      "pre-span-2",
						WorkspaceID: "1",
						SystemTagsString: map[string]string{
							"response_id": "resp-2",
						},
					},
				},
			}, nil
		}).Times(1)

	r, _ := NewTraceServiceImpl(
		repoMock,
		nil, nil, nil, nil,
		buildHelper,
		nil, nil, nil, nil,
	)

	req := &ListPreSpanOApiReq{
		WorkspaceID:           1,
		ThirdPartyWorkspaceID: "",
		StartTime:             time.Now().UnixMilli(),
		TraceID:               "trace-1",
		SpanID:                "cur-span",
		PreviousResponseID:    "prev-1",
		PlatformType:          loop_span.PlatformCozeLoop,
		Tenants:               []string{"tenant-1"},
	}

	resp, err := r.ListPreSpanOApi(context.Background(), req)
	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		// 顺序应按 RespIDByOrder：resp-2 在前、resp-1 在后
		got := make([]string, 0, len(resp.Spans))
		for _, s := range resp.Spans {
			got = append(got, s.SpanID)
		}
		assert.Equal(t, []string{"pre-span-2", "pre-span-1"}, got)
	}
}

func TestTraceServiceImpl_ListPreSpanOApi_GetPreSpanIDsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITraceRepo(ctrl)
	filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
	buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)

	repoMock.EXPECT().GetPreSpanIDs(gomock.Any(), gomock.Any()).Return(nil, nil, assert.AnError)

	r, _ := NewTraceServiceImpl(
		repoMock,
		nil, nil, nil, nil,
		buildHelper,
		nil, nil, nil, nil,
	)

	req := &ListPreSpanOApiReq{
		WorkspaceID:        1,
		TraceID:            "t",
		SpanID:             "s",
		PreviousResponseID: "p",
		StartTime:          time.Now().UnixMilli(),
		PlatformType:       loop_span.PlatformCozeLoop,
		Tenants:            []string{"tenant"},
	}
	resp, err := r.ListPreSpanOApi(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestTraceServiceImpl_ListPreSpanOApi_BatchGetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITraceRepo(ctrl)
	filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
	buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)

	// GetPreSpanIDs 正常
	repoMock.EXPECT().GetPreSpanIDs(gomock.Any(), gomock.Any()).Return([]string{"a"}, []string{"r"}, nil)
	// ListSpans 返回错误，触发 batchGetPreSpan 错误路径
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	r, _ := NewTraceServiceImpl(
		repoMock,
		nil, nil, nil, nil,
		buildHelper,
		nil, nil, nil, nil,
	)

	req := &ListPreSpanOApiReq{
		WorkspaceID:           1,
		ThirdPartyWorkspaceID: "2",
		TraceID:               "t",
		SpanID:                "s",
		PreviousResponseID:    "p",
		StartTime:             time.Now().UnixMilli(),
		PlatformType:          loop_span.PlatformCozeLoop,
		Tenants:               []string{"tenant"},
	}
	resp, err := r.ListPreSpanOApi(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestTraceServiceImpl_ListPreSpanOApi_AuthFail_NoWorkspaceAccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITraceRepo(ctrl)
	filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
	buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)

	// GetPreSpanIDs 正常
	repoMock.EXPECT().GetPreSpanIDs(gomock.Any(), gomock.Any()).Return([]string{"pre-1"}, []string{"resp-1"}, nil)
	// batch取回的span，当前span不在目标workspace，且其他span也不在目标workspace
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
		Spans: loop_span.SpanList{
			{
				TraceID:     "t",
				SpanID:      "s",
				WorkspaceID: "2", // 与req.WorkspaceID=1不一致
				SystemTagsString: map[string]string{
					"previous_response_id": "p",
				},
			},
			{
				TraceID:     "t",
				SpanID:      "pre-1",
				WorkspaceID: "2",
				SystemTagsString: map[string]string{
					"response_id": "resp-1",
				},
			},
		},
	}, nil).Times(1)
	// checkGetPreSpanAuth 会进一步查询trace下是否有在目标workspace的span，这里返回空以触发无权限错误
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{}}, nil).Times(1)

	r, _ := NewTraceServiceImpl(
		repoMock,
		nil, nil, nil, nil,
		buildHelper,
		nil, nil, nil, nil,
	)

	req := &ListPreSpanOApiReq{
		WorkspaceID:        1,
		TraceID:            "t",
		SpanID:             "s",
		PreviousResponseID: "p",
		StartTime:          time.Now().UnixMilli(),
		PlatformType:       loop_span.PlatformCozeLoop,
		Tenants:            []string{"tenant"},
	}
	resp, err := r.ListPreSpanOApi(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestTraceServiceImpl_ListPreSpanOApi_AuthSuccess_SameSpace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITraceRepo(ctrl)
	filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
	buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)

	// GetPreSpanIDs 正常
	repoMock.EXPECT().GetPreSpanIDs(gomock.Any(), gomock.Any()).Return([]string{"pre-1"}, []string{"resp-1"}, nil)
	// batch取回的span，当前span不在目标workspace，且其他span也不在目标workspace
	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
		Spans: loop_span.SpanList{
			{
				TraceID:     "t",
				SpanID:      "s",
				WorkspaceID: "2", // 与req.WorkspaceID=1不一致
				SystemTagsString: map[string]string{
					"previous_response_id": "p",
				},
			},
			{
				TraceID:     "t",
				SpanID:      "pre-1",
				WorkspaceID: "2",
				SystemTagsString: map[string]string{
					"response_id": "resp-1",
				},
			},
		},
	}, nil).Times(1)

	r, _ := NewTraceServiceImpl(
		repoMock,
		nil, nil, nil, nil,
		buildHelper,
		nil, nil, nil, nil,
	)

	req := &ListPreSpanOApiReq{
		WorkspaceID:        2,
		TraceID:            "t",
		SpanID:             "s",
		PreviousResponseID: "p",
		StartTime:          time.Now().UnixMilli(),
		PlatformType:       loop_span.PlatformCozeLoop,
		Tenants:            []string{"tenant"},
	}
	resp, err := r.ListPreSpanOApi(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}
