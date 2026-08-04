// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type fakeEvaluationSetTemplateService struct {
	templates     []*entity.EvaluationSetTemplate
	total         *int64
	nextPageToken *string
	builtSchema   *entity.EvaluationSetSchema
	buildParam    *entity.BuildEvaluationSetSchemaFromTemplateParam
	validateParam *entity.ValidateEvaluationSetSchemaUpdateParam
}

func (f *fakeEvaluationSetTemplateService) ListEvaluationSetTemplates(_ context.Context, _ *entity.ListEvaluationSetTemplatesParam) ([]*entity.EvaluationSetTemplate, *int64, *string, error) {
	return f.templates, f.total, f.nextPageToken, nil
}

func (f *fakeEvaluationSetTemplateService) BuildEvaluationSetSchemaFromTemplate(_ context.Context, param *entity.BuildEvaluationSetSchemaFromTemplateParam) (*entity.EvaluationSetSchema, error) {
	f.buildParam = param
	return f.builtSchema, nil
}

func (f *fakeEvaluationSetTemplateService) ValidateEvaluationSetSchemaUpdate(_ context.Context, param *entity.ValidateEvaluationSetSchemaUpdateParam) error {
	f.validateParam = param
	return nil
}

func TestEvaluationSetServiceImpl_CreateEvaluationSetClearsClientFieldLocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := rpcmocks.NewMockIDatasetRPCAdapter(ctrl)
	svc := &EvaluationSetServiceImpl{
		datasetRPCAdapter: adapter,
		templateService:   NewNoopEvaluationSetTemplateService(),
	}
	schema := &entity.EvaluationSetSchema{FieldSchemas: []*entity.FieldSchema{{Key: "input", Locked: true}}}
	adapter.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *rpc.CreateDatasetParam) (int64, error) {
		require.Len(t, param.EvaluationSetItems.FieldSchemas, 1)
		assert.False(t, param.EvaluationSetItems.FieldSchemas[0].Locked)
		assert.Nil(t, param.TemplateDatasetID)
		return int64(100), nil
	})

	id, err := svc.CreateEvaluationSet(context.Background(), &entity.CreateEvaluationSetParam{
		SpaceID:             1,
		Name:                "plain",
		EvaluationSetSchema: schema,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(100), id)
}

func TestEvaluationSetServiceImpl_CreateEvaluationSetFromTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	adapter := rpcmocks.NewMockIDatasetRPCAdapter(ctrl)
	templateID := int64(200)
	builtSchema := &entity.EvaluationSetSchema{FieldSchemas: []*entity.FieldSchema{{Key: "messages", Locked: true}}}
	templateSvc := &fakeEvaluationSetTemplateService{builtSchema: builtSchema}
	svc := &EvaluationSetServiceImpl{datasetRPCAdapter: adapter, templateService: templateSvc}
	adapter.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *rpc.CreateDatasetParam) (int64, error) {
		assert.Equal(t, &templateID, param.TemplateDatasetID)
		assert.Same(t, builtSchema, param.EvaluationSetItems)
		return int64(300), nil
	})

	id, err := svc.CreateEvaluationSet(context.Background(), &entity.CreateEvaluationSetParam{
		SpaceID:           1,
		Name:              "templated",
		TemplateDatasetID: &templateID,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(300), id)
	require.NotNil(t, templateSvc.buildParam)
	assert.Equal(t, templateID, templateSvc.buildParam.TemplateDatasetID)
}

func TestNoopEvaluationSetTemplateService(t *testing.T) {
	svc := NewNoopEvaluationSetTemplateService()
	templates, total, nextPageToken, err := svc.ListEvaluationSetTemplates(context.Background(), &entity.ListEvaluationSetTemplatesParam{})
	require.NoError(t, err)
	assert.Empty(t, templates)
	assert.Equal(t, int64(0), *total)
	assert.Nil(t, nextPageToken)

	_, err = svc.BuildEvaluationSetSchemaFromTemplate(context.Background(), &entity.BuildEvaluationSetSchemaFromTemplateParam{TemplateDatasetID: 1})
	assert.Error(t, err)
}
