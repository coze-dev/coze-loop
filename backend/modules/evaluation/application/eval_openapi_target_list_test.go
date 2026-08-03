// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaincommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domainevaltarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	openapicommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/common"
	openapievaltarget "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/eval_target"
	evaltargetapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
)

type fakeSourceEvalTargetLister struct {
	req    *evaltargetapi.ListSourceEvalTargetsRequest
	resp   *evaltargetapi.ListSourceEvalTargetsResponse
	err    error
	called bool
}

func (f *fakeSourceEvalTargetLister) ListSourceEvalTargets(_ context.Context, req *evaltargetapi.ListSourceEvalTargetsRequest) (*evaltargetapi.ListSourceEvalTargetsResponse, error) {
	f.called = true
	f.req = req
	return f.resp, f.err
}

func TestEvalOpenAPIApplication_ListEvalTargetsOApi(t *testing.T) {
	t.Run("invalid request", func(t *testing.T) {
		app := &EvalOpenAPIApplication{}

		resp, err := app.ListEvalTargetsOApi(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("missing target type", func(t *testing.T) {
		lister := &fakeSourceEvalTargetLister{}
		app := &EvalOpenAPIApplication{
			metric:                 &fakeOpenAPIMetric{},
			sourceEvalTargetLister: lister,
		}

		resp, err := app.ListEvalTargetsOApi(context.Background(), &openapi.ListEvalTargetsOApiRequest{
			WorkspaceID: gptr.Of(int64(1001)),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.False(t, lister.called)
	})

	t.Run("downstream error", func(t *testing.T) {
		lister := &fakeSourceEvalTargetLister{err: errors.New("list failed")}
		app := &EvalOpenAPIApplication{
			metric:                 &fakeOpenAPIMetric{},
			sourceEvalTargetLister: lister,
		}

		resp, err := app.ListEvalTargetsOApi(context.Background(), &openapi.ListEvalTargetsOApiRequest{
			WorkspaceID:    gptr.Of(int64(1001)),
			EvalTargetType: gptr.Of(openapievaltarget.EvalTargetTypeCozeLoopPrompt),
		})

		require.EqualError(t, err, "list failed")
		assert.Nil(t, resp)
	})

	t.Run("success", func(t *testing.T) {
		sourceSpaceID := int64(2002)
		lister := &fakeSourceEvalTargetLister{
			resp: &evaltargetapi.ListSourceEvalTargetsResponse{
				EvalTargets: []*domainevaltarget.EvalTarget{
					{
						WorkspaceID:    gptr.Of(sourceSpaceID),
						SourceTargetID: gptr.Of("3003"),
						EvalTargetType: gptr.Of(domainevaltarget.EvalTargetType_CozeLoopPrompt),
						EvalTargetVersion: &domainevaltarget.EvalTargetVersion{
							WorkspaceID:         gptr.Of(sourceSpaceID),
							SourceTargetVersion: gptr.Of("1.0.0"),
							SharedInfo: &domaincommon.SharedResourceInfo{
								IsShared:      gptr.Of(true),
								SourceSpaceID: gptr.Of(sourceSpaceID),
								AccessLevel:   gptr.Of("readable"),
								VersionPolicy: gptr.Of("latest"),
							},
						},
						SharedInfo: &domaincommon.SharedResourceInfo{
							IsShared:      gptr.Of(true),
							SourceSpaceID: gptr.Of(sourceSpaceID),
							AccessLevel:   gptr.Of("readable"),
							VersionPolicy: gptr.Of("latest"),
						},
					},
				},
				HasMore:       gptr.Of(true),
				NextPageToken: gptr.Of("next-token"),
			},
		}
		metric := &fakeOpenAPIMetric{}
		app := &EvalOpenAPIApplication{
			metric:                 metric,
			sourceEvalTargetLister: lister,
		}
		req := &openapi.ListEvalTargetsOApiRequest{
			WorkspaceID:    gptr.Of(int64(1001)),
			EvalTargetType: gptr.Of(openapievaltarget.EvalTargetTypeCozeLoopPrompt),
			SearchName:     gptr.Of("prompt"),
			SharedOption: &openapicommon.SharedResourceOption{
				IsShared:      gptr.Of(true),
				SourceSpaceID: gptr.Of(sourceSpaceID),
			},
			PageToken: gptr.Of("page-token"),
			PageSize:  gptr.Of(int32(20)),
		}

		resp, err := app.ListEvalTargetsOApi(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotNil(t, resp.Data)
		require.Len(t, resp.Data.EvalTargets, 1)
		target := resp.Data.EvalTargets[0]
		assert.Equal(t, sourceSpaceID, target.GetWorkspaceID())
		assert.Equal(t, "3003", target.GetSourceTargetID())
		assert.Equal(t, openapievaltarget.EvalTargetTypeCozeLoopPrompt, target.GetEvalTargetType())
		require.NotNil(t, target.GetSharedInfo())
		assert.Equal(t, sourceSpaceID, target.GetSharedInfo().GetSourceSpaceID())
		require.NotNil(t, target.GetEvalTargetVersion())
		assert.Equal(t, sourceSpaceID, target.GetEvalTargetVersion().GetWorkspaceID())
		assert.Equal(t, "latest", target.GetEvalTargetVersion().GetSharedInfo().GetVersionPolicy())
		assert.True(t, resp.Data.GetHasMore())
		assert.Equal(t, "next-token", resp.Data.GetNextPageToken())

		require.NotNil(t, lister.req)
		assert.Equal(t, int64(1001), lister.req.GetWorkspaceID())
		assert.Equal(t, domainevaltarget.EvalTargetType_CozeLoopPrompt, lister.req.GetTargetType())
		assert.Equal(t, "prompt", lister.req.GetName())
		assert.True(t, lister.req.GetSharedOption().GetIsShared())
		assert.Equal(t, sourceSpaceID, lister.req.GetSharedOption().GetSourceSpaceID())
		assert.Equal(t, "page-token", lister.req.GetPageToken())
		assert.Equal(t, int32(20), lister.req.GetPageSize())
		assert.True(t, metric.called)
	})

	t.Run("unsupported response target type", func(t *testing.T) {
		lister := &fakeSourceEvalTargetLister{
			resp: &evaltargetapi.ListSourceEvalTargetsResponse{
				EvalTargets: []*domainevaltarget.EvalTarget{{
					SourceTargetID: gptr.Of("3003"),
					EvalTargetType: gptr.Of(domainevaltarget.EvalTargetType_WebAgent),
				}},
			},
		}
		app := &EvalOpenAPIApplication{
			metric:                 &fakeOpenAPIMetric{},
			sourceEvalTargetLister: lister,
		}

		resp, err := app.ListEvalTargetsOApi(context.Background(), &openapi.ListEvalTargetsOApiRequest{
			WorkspaceID:    gptr.Of(int64(1001)),
			EvalTargetType: gptr.Of(openapievaltarget.EvalTargetTypeCozeLoopPrompt),
		})

		require.Error(t, err)
		assert.Nil(t, resp)
	})
}
