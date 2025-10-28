// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluationset

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	dataset_domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	eval_set_domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/samber/lo"
)

//go:generate mockgen -source=evaluation_set.go -destination=mocks/mock_evaluation_set.go
//go:generate mockgen -package=mocks -destination=mocks/mock_evaluationsetservice_client.go github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluationsetservice Client

// Test helper functions
func createTestDataset() *entity.Dataset {
	return &entity.Dataset{
		ID:              1,
		WorkspaceID:     100,
		Name:            "test-dataset",
		Description:     "test description",
		DatasetCategory: entity.DatasetCategory_Evaluation,
		DatasetVersion: entity.DatasetVersion{
			DatasetSchema: entity.DatasetSchema{
				FieldSchemas: []entity.FieldSchema{
					{
						Key:         gptr.Of("input"),
						Name:        "Input",
						ContentType: entity.ContentType_Text,
					},
					{
						Key:         gptr.Of("output"),
						Name:        "Output",
						ContentType: entity.ContentType_Text,
					},
				},
			},
		},
	}
}

func createTestDatasetItems(count int) []*entity.DatasetItem {
	items := make([]*entity.DatasetItem, count)
	for i := 0; i < count; i++ {
		items[i] = &entity.DatasetItem{
			WorkspaceID: 100,
			DatasetID:   1,
			SpanID:      fmt.Sprintf("span-%d", i),
			FieldData: []*entity.FieldData{
				{
					Key:  "input",
					Name: "Input",
					Content: &entity.Content{
						ContentType: entity.ContentType_Text,
						Text:        fmt.Sprintf("test input %d", i),
					},
				},
				{
					Key:  "output",
					Name: "Output",
					Content: &entity.Content{
						ContentType: entity.ContentType_Text,
						Text:        fmt.Sprintf("test output %d", i),
					},
				},
			},
		}
	}
	return items
}

func TestEvaluationSetProvider_CreateDataset(t *testing.T) {
	t.Run("workspace ID is required", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		ctx := session.WithCtxUser(context.Background(), &session.User{ID: "12345"})
		dataset := createTestDataset()
		dataset.WorkspaceID = 0

		_, err := provider.CreateDataset(ctx, dataset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workspace ID is required")
	})

	t.Run("dataset name is required", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		ctx := session.WithCtxUser(context.Background(), &session.User{ID: "12345"})
		dataset := createTestDataset()
		dataset.Name = ""

		_, err := provider.CreateDataset(ctx, dataset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dataset name is required")
	})

	t.Run("user ID is required", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		ctx := context.Background() // no user ID in context
		dataset := createTestDataset()

		_, err := provider.CreateDataset(ctx, dataset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "userid is required")
	})
}

func TestEvaluationSetProvider_UpdateDatasetSchema(t *testing.T) {
	t.Run("workspace ID is required", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		dataset := createTestDataset()
		dataset.WorkspaceID = 0

		err := provider.UpdateDatasetSchema(context.Background(), dataset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workspace ID is required")
	})

	t.Run("dataset ID is required", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		dataset := createTestDataset()
		dataset.ID = 0

		err := provider.UpdateDatasetSchema(context.Background(), dataset)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dataset ID is required")
	})
}

func TestEvaluationSetProvider_GetDataset(t *testing.T) {
	t.Run("workspace ID is required", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		_, err := provider.GetDataset(context.Background(), 0, 1, entity.DatasetCategory_Evaluation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workspace ID is required")
	})

	t.Run("dataset ID is required", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		_, err := provider.GetDataset(context.Background(), 100, 0, entity.DatasetCategory_Evaluation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dataset ID is required")
	})
}

func TestEvaluationSetProvider_ClearDatasetItems(t *testing.T) {
	t.Run("workspace ID is required", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		err := provider.ClearDatasetItems(context.Background(), 0, 1, entity.DatasetCategory_Evaluation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workspace ID is required")
	})

	t.Run("dataset ID is required", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		err := provider.ClearDatasetItems(context.Background(), 100, 0, entity.DatasetCategory_Evaluation)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dataset ID is required")
	})
}

func TestEvaluationSetProvider_AddDatasetItems(t *testing.T) {
	t.Run("add empty items list", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		successItems, errorGroups, err := provider.AddDatasetItems(context.Background(), 1, entity.DatasetCategory_Evaluation, []*entity.DatasetItem{})
		assert.NoError(t, err)
		assert.Equal(t, 0, len(successItems))
		assert.Equal(t, 0, len(errorGroups))
	})

	t.Run("items belong to different workspace", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		items := createTestDatasetItems(2)
		items[1].WorkspaceID = 200 // 不同的workspace

		_, _, err := provider.AddDatasetItems(context.Background(), 1, entity.DatasetCategory_Evaluation, items)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "all items must belong to the same workspace and dataset")
	})

	t.Run("items belong to different dataset", func(t *testing.T) {
		provider := &EvaluationSetProvider{}
		items := createTestDatasetItems(2)
		items[1].DatasetID = 2 // 不同的dataset

		_, _, err := provider.AddDatasetItems(context.Background(), 1, entity.DatasetCategory_Evaluation, items)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "all items must belong to the same workspace and dataset")
	})
}

// Test data conversion functions
func TestDatasetSchemaDO2DTO(t *testing.T) {
	t.Run("nil schema", func(t *testing.T) {
		got := datasetSchemaDO2DTO(nil)
		assert.Nil(t, got)
	})

	t.Run("schema with field schemas", func(t *testing.T) {
		schema := &entity.DatasetSchema{
			ID:          1,
			WorkspaceID: 100,
			DatasetID:   10,
			FieldSchemas: []entity.FieldSchema{
				{
					Key:         gptr.Of("input"),
					Name:        "Input",
					Description: "Input field",
					ContentType: entity.ContentType_Text,
				},
			},
		}

		got := datasetSchemaDO2DTO(schema)
		assert.NotNil(t, got)
		assert.Equal(t, int64(1), got.GetID())
		assert.Equal(t, int64(100), got.GetWorkspaceID())
		assert.Equal(t, int64(10), got.GetEvaluationSetID())
		assert.Equal(t, 1, len(got.FieldSchemas))
	})
}

func TestFieldSchemaDO2DTO(t *testing.T) {
	fs := entity.FieldSchema{
		Key:         gptr.Of("test"),
		Name:        "Test Field",
		Description: "Test description",
		ContentType: entity.ContentType_Text,
		TextSchema:  "text schema",
	}

	got := fieldSchemaDO2DTO(fs)
	assert.Equal(t, "test", got.GetKey())
	assert.Equal(t, "Test Field", got.GetName())
	assert.Equal(t, "Test description", got.GetDescription())
	assert.Equal(t, common.ContentType(entity.ContentType_Text), got.GetContentType())
	assert.Equal(t, "text schema", got.GetTextSchema())
}

func TestEvaluationSetDTO2DO(t *testing.T) {
	t.Run("nil evaluation set", func(t *testing.T) {
		got := evaluationSetDTO2DO(nil)
		assert.Nil(t, got)
	})

	t.Run("basic evaluation set", func(t *testing.T) {
		evalSet := &eval_set_domain.EvaluationSet{
			ID:          gptr.Of(int64(1)),
			WorkspaceID: gptr.Of(int64(100)),
			Name:        gptr.Of("test-dataset"),
			Description: gptr.Of("test description"),
		}

		got := evaluationSetDTO2DO(evalSet)
		assert.NotNil(t, got)
		assert.Equal(t, int64(1), got.ID)
		assert.Equal(t, int64(100), got.WorkspaceID)
		assert.Equal(t, "test-dataset", got.Name)
		assert.Equal(t, "test description", got.Description)
		assert.Equal(t, entity.DatasetCategory_Evaluation, got.DatasetCategory)
	})

	t.Run("evaluation set with biz category", func(t *testing.T) {
		evalSet := &eval_set_domain.EvaluationSet{
			ID:          gptr.Of(int64(1)),
			WorkspaceID: gptr.Of(int64(100)),
			Name:        gptr.Of("test-dataset"),
			Description: gptr.Of("test description"),
			BizCategory: lo.ToPtr(eval_set_domain.BizCategory("QA")),
		}

		got := evaluationSetDTO2DO(evalSet)
		assert.NotNil(t, got)
		assert.Equal(t, int64(1), got.ID)
		assert.Equal(t, int64(100), got.WorkspaceID)
		assert.Equal(t, "test-dataset", got.Name)
		assert.Equal(t, "test description", got.Description)
		assert.Equal(t, entity.DatasetCategory_Evaluation, got.DatasetCategory)
		assert.NotNil(t, got.EvaluationBizCategory)
		assert.Equal(t, entity.EvaluationBizCategory("QA"), *got.EvaluationBizCategory)
	})
}

func TestFieldDisplayFormatDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		df   entity.FieldDisplayFormat
		want dataset_domain.FieldDisplayFormat
	}{
		{
			name: "plain text",
			df:   entity.FieldDisplayFormat_PlainText,
			want: dataset_domain.FieldDisplayFormat_PlainText,
		},
		{
			name: "markdown",
			df:   entity.FieldDisplayFormat_Markdown,
			want: dataset_domain.FieldDisplayFormat_Markdown,
		},
		{
			name: "json",
			df:   entity.FieldDisplayFormat_JSON,
			want: dataset_domain.FieldDisplayFormat_JSON,
		},
		{
			name: "yaml",
			df:   entity.FieldDisplayFormat_YAML,
			want: dataset_domain.FieldDisplayFormat_YAML,
		},
		{
			name: "code",
			df:   entity.FieldDisplayFormat_Code,
			want: dataset_domain.FieldDisplayFormat_Code,
		},
		{
			name: "unknown format defaults to plain text",
			df:   entity.FieldDisplayFormat(999),
			want: dataset_domain.FieldDisplayFormat_PlainText,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FieldDisplayFormatDO2DTO(tt.df)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDatasetItemsDO2DTO(t *testing.T) {
	t.Run("empty items", func(t *testing.T) {
		got := datasetItemsDO2DTO([]*entity.DatasetItem{})
		assert.Equal(t, 0, len(got))
	})

	t.Run("nil items", func(t *testing.T) {
		got := datasetItemsDO2DTO(nil)
		assert.Equal(t, 0, len(got))
	})

	t.Run("items with nil element", func(t *testing.T) {
		items := []*entity.DatasetItem{nil, createTestDatasetItems(1)[0]}
		got := datasetItemsDO2DTO(items)
		assert.Equal(t, 1, len(got)) // nil items should be skipped
	})

	t.Run("valid items", func(t *testing.T) {
		items := createTestDatasetItems(2)
		got := datasetItemsDO2DTO(items)
		assert.Equal(t, 2, len(got))

		// Verify structure for non-empty results
		for _, item := range got {
			assert.NotNil(t, item)
			assert.NotNil(t, item.WorkspaceID)
			assert.NotNil(t, item.EvaluationSetID)
		}
	})

	t.Run("items with empty field data", func(t *testing.T) {
		items := []*entity.DatasetItem{
			{
				WorkspaceID: 100,
				DatasetID:   1,
				SpanID:      "span-1",
				FieldData:   []*entity.FieldData{}, // empty field data
			},
		}
		got := datasetItemsDO2DTO(items)
		assert.Equal(t, 1, len(got)) // item should still be included but without turns
	})
}

func TestConvertContentDO2DTO(t *testing.T) {
	t.Run("nil content", func(t *testing.T) {
		got := convertContentDO2DTO(nil)
		assert.Nil(t, got)
	})

	t.Run("text content", func(t *testing.T) {
		content := &entity.Content{
			ContentType: entity.ContentType_Text,
			Text:        "test text",
		}

		got := convertContentDO2DTO(content)
		assert.NotNil(t, got)
		assert.Equal(t, entity.CommonContentTypeDO2DTO(entity.ContentType_Text), got.ContentType)
		assert.Equal(t, "test text", got.GetText())
	})

	t.Run("content with multipart", func(t *testing.T) {
		content := &entity.Content{
			ContentType: entity.ContentType_MultiPart,
			MultiPart: []*entity.Content{
				{
					ContentType: entity.ContentType_Text,
					Text:        "part1",
				},
			},
		}

		got := convertContentDO2DTO(content)
		assert.NotNil(t, got)
		assert.Equal(t, entity.CommonContentTypeDO2DTO(entity.ContentType_MultiPart), got.ContentType)
		assert.Equal(t, 1, len(got.MultiPart))
		assert.Equal(t, "part1", got.MultiPart[0].GetText())
	})
}
