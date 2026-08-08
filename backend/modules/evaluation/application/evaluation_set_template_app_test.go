// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	internalapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestEvaluationSetApplicationImpl_ListEvaluationSetTemplates(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	svc := servicemocks.NewMockIEvaluationSetService(ctrl)
	app := &EvaluationSetApplicationImpl{auth: auth, evaluationSetService: svc}
	nextPageToken := "next"
	total := int64(2)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	svc.EXPECT().ListEvaluationSetTemplates(gomock.Any(), &entity.ListEvaluationSetTemplatesParam{
		SpaceID:   10,
		PageSize:  gptr.Of(int32(20)),
		PageToken: gptr.Of("current"),
	}).Return([]*entity.EvaluationSetTemplate{{
		TemplateDatasetID:   100,
		TemplateDatasetName: "dialog",
		IsEditable:          true,
		EvaluationSetSchema: &entity.EvaluationSetSchema{FieldSchemas: []*entity.FieldSchema{{Key: "messages", Locked: true}}},
	}}, &total, &nextPageToken, nil)

	resp, err := app.ListEvaluationSetTemplates(context.Background(), &internalapi.ListEvaluationSetTemplatesRequest{
		WorkspaceID: 10,
		PageSize:    gptr.Of(int32(20)),
		PageToken:   gptr.Of("current"),
	})
	require.NoError(t, err)
	require.Len(t, resp.Templates, 1)
	assert.Equal(t, int64(100), resp.Templates[0].GetTemplateDatasetID())
	require.NotNil(t, resp.Templates[0].IsEditable)
	assert.True(t, resp.Templates[0].GetIsEditable())
	require.Len(t, resp.Templates[0].EvaluationSetSchema.FieldSchemas, 1)
	assert.True(t, resp.Templates[0].EvaluationSetSchema.FieldSchemas[0].GetLocked())
	assert.Equal(t, &total, resp.Total)
	assert.Equal(t, &nextPageToken, resp.NextPageToken)
}

func TestEvalOpenAPIApplication_ListEvaluationSetTemplatesOApi(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	svc := servicemocks.NewMockIEvaluationSetService(ctrl)
	metric := &fakeOpenAPIMetric{}
	app := &EvalOpenAPIApplication{auth: auth, evaluationSetService: svc, metric: metric}
	total := int64(1)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
	svc.EXPECT().ListEvaluationSetTemplates(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetTemplate{{
		TemplateDatasetID:   101,
		TemplateDatasetName: "qa",
		IsEditable:          false,
		EvaluationSetSchema: &entity.EvaluationSetSchema{FieldSchemas: []*entity.FieldSchema{{Key: "question", Locked: true}}},
	}}, &total, nil, nil)

	resp, err := app.ListEvaluationSetTemplatesOApi(context.Background(), &openapi.ListEvaluationSetTemplatesOApiRequest{WorkspaceID: gptr.Of(int64(11))})
	require.NoError(t, err)
	require.NotNil(t, resp.Data)
	require.Len(t, resp.Data.Templates, 1)
	assert.Equal(t, int64(101), resp.Data.Templates[0].GetTemplateDatasetID())
	require.NotNil(t, resp.Data.Templates[0].IsEditable)
	assert.False(t, resp.Data.Templates[0].GetIsEditable())
	assert.True(t, resp.Data.Templates[0].EvaluationSetSchema.FieldSchemas[0].GetLocked())
	assert.False(t, resp.Data.GetHasMore())
	assert.True(t, metric.called)
}
