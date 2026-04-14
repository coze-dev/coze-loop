// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	datasetdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	domain_dataset "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/rpc/data/mocks"
)

//go:generate mockgen -package=mocks -destination=mocks/mock_datasetservice_client.go github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/datasetservice Client

func newTestAdapter(ctrl *gomock.Controller) (*DatasetRPCAdapter, *mocks.MockClient) {
	mockClient := mocks.NewMockClient(ctrl)
	adapter := &DatasetRPCAdapter{client: mockClient}
	return adapter, mockClient
}

func TestValidateMultiPartData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter, _ := newTestAdapter(ctrl)

	strategy := entity.MultiModalStoreStrategyStore
	result, err := adapter.ValidateMultiPartData(ctx, 1, []string{"data1"}, &entity.MultiModalStoreOption{
		MultiModalStoreStrategy: &strategy,
	})
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ValidateMultiPartData not implemented")
}

func TestBatchCreateDatasetItems(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success_without_field_write_options", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().BatchCreateDatasetItems(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, req *datasetdto.BatchCreateDatasetItemsRequest, opts ...interface{}) (*datasetdto.BatchCreateDatasetItemsResponse, error) {
				assert.Equal(t, int64(100), *req.WorkspaceID)
				assert.Equal(t, int64(200), req.DatasetID)
				assert.Nil(t, req.FieldWriteOptions)
				return &datasetdto.BatchCreateDatasetItemsResponse{
					AddedItems: map[int64]int64{0: 1},
					BaseResp:   &base.BaseResp{StatusCode: 0},
				}, nil
			})

		param := &rpc.BatchCreateDatasetItemsParam{
			SpaceID:         100,
			EvaluationSetID: 200,
			Items:           []*entity.EvaluationSetItem{},
		}
		idMap, errGroup, _, err := adapter.BatchCreateDatasetItems(ctx, param)
		assert.NoError(t, err)
		assert.Equal(t, map[int64]int64{0: 1}, idMap)
		assert.Nil(t, errGroup)
	})

	t.Run("success_with_field_write_options", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		strategy := entity.MultiModalStoreStrategyStore
		mockClient.EXPECT().BatchCreateDatasetItems(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, req *datasetdto.BatchCreateDatasetItemsRequest, opts ...interface{}) (*datasetdto.BatchCreateDatasetItemsResponse, error) {
				assert.NotNil(t, req.FieldWriteOptions)
				assert.Len(t, req.FieldWriteOptions, 1)
				assert.Equal(t, "field1", *req.FieldWriteOptions[0].FieldName)
				assert.Equal(t, domain_dataset.MultiModalStoreStrategyStore, *req.FieldWriteOptions[0].MultiModalStoreOpt.MultiModalStoreStrategy)
				return &datasetdto.BatchCreateDatasetItemsResponse{
					AddedItems: map[int64]int64{0: 10},
					BaseResp:   &base.BaseResp{StatusCode: 0},
				}, nil
			})

		param := &rpc.BatchCreateDatasetItemsParam{
			SpaceID:         100,
			EvaluationSetID: 200,
			Items:           []*entity.EvaluationSetItem{},
			FieldWriteOptions: []*entity.FieldWriteOption{
				{
					FieldName: gptr.Of("field1"),
					FieldKey:  gptr.Of("key1"),
					MultiModalStoreOpt: &entity.MultiModalStoreOption{
						MultiModalStoreStrategy: &strategy,
						ContentType:             gptr.Of(entity.ContentTypeImage),
					},
				},
			},
		}
		idMap, _, _, err := adapter.BatchCreateDatasetItems(ctx, param)
		assert.NoError(t, err)
		assert.Equal(t, map[int64]int64{0: 10}, idMap)
	})

	t.Run("rpc_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().BatchCreateDatasetItems(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("rpc error"))

		param := &rpc.BatchCreateDatasetItemsParam{
			SpaceID:         100,
			EvaluationSetID: 200,
			Items:           []*entity.EvaluationSetItem{},
		}
		_, _, _, err := adapter.BatchCreateDatasetItems(ctx, param)
		assert.Error(t, err)
	})

	t.Run("nil_response", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().BatchCreateDatasetItems(gomock.Any(), gomock.Any()).
			Return(nil, nil)

		param := &rpc.BatchCreateDatasetItemsParam{
			SpaceID:         100,
			EvaluationSetID: 200,
			Items:           []*entity.EvaluationSetItem{},
		}
		_, _, _, err := adapter.BatchCreateDatasetItems(ctx, param)
		assert.Error(t, err)
	})

	t.Run("base_resp_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().BatchCreateDatasetItems(gomock.Any(), gomock.Any()).
			Return(&datasetdto.BatchCreateDatasetItemsResponse{
				BaseResp: &base.BaseResp{StatusCode: 1001, StatusMessage: "invalid param"},
			}, nil)

		param := &rpc.BatchCreateDatasetItemsParam{
			SpaceID:         100,
			EvaluationSetID: 200,
			Items:           []*entity.EvaluationSetItem{},
		}
		_, _, _, err := adapter.BatchCreateDatasetItems(ctx, param)
		assert.Error(t, err)
	})
}

func TestUpdateDatasetItem(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success_without_field_write_options", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().UpdateDatasetItem(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, req *datasetdto.UpdateDatasetItemRequest, opts ...interface{}) (*datasetdto.UpdateDatasetItemResponse, error) {
				assert.Equal(t, int64(100), *req.WorkspaceID)
				assert.Equal(t, int64(200), req.DatasetID)
				assert.Equal(t, int64(300), req.ItemID)
				assert.Nil(t, req.FieldWriteOptions)
				return &datasetdto.UpdateDatasetItemResponse{
					BaseResp: &base.BaseResp{StatusCode: 0},
				}, nil
			})

		turns := []*entity.Turn{
			{
				FieldDataList: []*entity.FieldData{
					{
						Key:  "k1",
						Name: "n1",
						Content: &entity.Content{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("hello"),
						},
					},
				},
			},
		}
		err := adapter.UpdateDatasetItem(ctx, 100, 200, 300, turns, nil)
		assert.NoError(t, err)
	})

	t.Run("success_with_field_write_options", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		strategy := entity.MultiModalStoreStrategyPassthrough
		mockClient.EXPECT().UpdateDatasetItem(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, req *datasetdto.UpdateDatasetItemRequest, opts ...interface{}) (*datasetdto.UpdateDatasetItemResponse, error) {
				assert.NotNil(t, req.FieldWriteOptions)
				assert.Len(t, req.FieldWriteOptions, 1)
				assert.Equal(t, "field1", *req.FieldWriteOptions[0].FieldName)
				assert.Equal(t, domain_dataset.MultiModalStoreStrategyPassthrough, *req.FieldWriteOptions[0].MultiModalStoreOpt.MultiModalStoreStrategy)
				assert.Equal(t, domain_dataset.ContentType_Image, *req.FieldWriteOptions[0].MultiModalStoreOpt.ContentType)
				return &datasetdto.UpdateDatasetItemResponse{
					BaseResp: &base.BaseResp{StatusCode: 0},
				}, nil
			})

		turns := []*entity.Turn{
			{
				FieldDataList: []*entity.FieldData{
					{
						Key:  "k1",
						Name: "n1",
						Content: &entity.Content{
							ContentType: gptr.Of(entity.ContentTypeText),
							Text:        gptr.Of("hello"),
						},
					},
				},
			},
		}
		fieldWriteOptions := []*entity.FieldWriteOption{
			{
				FieldName: gptr.Of("field1"),
				FieldKey:  gptr.Of("key1"),
				MultiModalStoreOpt: &entity.MultiModalStoreOption{
					MultiModalStoreStrategy: &strategy,
					ContentType:             gptr.Of(entity.ContentTypeImage),
				},
			},
		}
		err := adapter.UpdateDatasetItem(ctx, 100, 200, 300, turns, fieldWriteOptions)
		assert.NoError(t, err)
	})

	t.Run("rpc_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().UpdateDatasetItem(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("rpc error"))

		turns := []*entity.Turn{
			{FieldDataList: []*entity.FieldData{{Key: "k1", Name: "n1", Content: &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("text")}}}},
		}
		err := adapter.UpdateDatasetItem(ctx, 100, 200, 300, turns, nil)
		assert.Error(t, err)
	})

	t.Run("nil_response", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().UpdateDatasetItem(gomock.Any(), gomock.Any()).
			Return(nil, nil)

		turns := []*entity.Turn{
			{FieldDataList: []*entity.FieldData{{Key: "k1", Name: "n1", Content: &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("text")}}}},
		}
		err := adapter.UpdateDatasetItem(ctx, 100, 200, 300, turns, nil)
		assert.Error(t, err)
	})

	t.Run("base_resp_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().UpdateDatasetItem(gomock.Any(), gomock.Any()).
			Return(&datasetdto.UpdateDatasetItemResponse{
				BaseResp: &base.BaseResp{StatusCode: 1001, StatusMessage: "server error"},
			}, nil)

		turns := []*entity.Turn{
			{FieldDataList: []*entity.FieldData{{Key: "k1", Name: "n1", Content: &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("text")}}}},
		}
		err := adapter.UpdateDatasetItem(ctx, 100, 200, 300, turns, nil)
		assert.Error(t, err)
	})
}

func TestImportDataset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success_with_option", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().ImportDataset(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, req *datasetdto.ImportDatasetRequest, opts ...interface{}) (*datasetdto.ImportDatasetResponse, error) {
				assert.Equal(t, int64(100), *req.WorkspaceID)
				assert.Equal(t, int64(200), req.DatasetID)
				assert.NotNil(t, req.Option)
				assert.Equal(t, gptr.Of(true), req.Option.OverwriteDataset)
				assert.Len(t, req.Option.FieldWriteOptions, 1)
				return &datasetdto.ImportDatasetResponse{
					JobID:    gptr.Of(int64(999)),
					BaseResp: &base.BaseResp{StatusCode: 0},
				}, nil
			})

		param := &rpc.ImportDatasetParam{
			WorkspaceID: 100,
			DatasetID:   200,
			File: &entity.DatasetIOFile{
				Path: "path/to/file",
			},
			FieldMappings: []*entity.FieldMapping{{Source: "s", Target: "t"}},
			Option: &entity.DatasetIOJobOption{
				OverwriteDataset: gptr.Of(true),
				FieldWriteOptions: []*entity.FieldWriteOption{
					{FieldName: gptr.Of("f1")},
				},
			},
		}
		jobID, err := adapter.ImportDataset(ctx, param)
		assert.NoError(t, err)
		assert.Equal(t, int64(999), jobID)
	})

	t.Run("rpc_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().ImportDataset(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("rpc error"))

		param := &rpc.ImportDatasetParam{
			WorkspaceID: 100,
			DatasetID:   200,
		}
		_, err := adapter.ImportDataset(ctx, param)
		assert.Error(t, err)
	})

	t.Run("nil_response", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().ImportDataset(gomock.Any(), gomock.Any()).
			Return(nil, nil)

		param := &rpc.ImportDatasetParam{
			WorkspaceID: 100,
			DatasetID:   200,
		}
		_, err := adapter.ImportDataset(ctx, param)
		assert.Error(t, err)
	})
}

func TestNewDatasetRPCAdapter(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	adapter := NewDatasetRPCAdapter(mockClient)
	assert.NotNil(t, adapter)
}

func TestCreateDataset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, req *datasetdto.CreateDatasetRequest, opts ...interface{}) (*datasetdto.CreateDatasetResponse, error) {
				assert.Equal(t, int64(100), req.WorkspaceID)
				assert.Equal(t, "test_dataset", req.Name)
				assert.Equal(t, domain_dataset.DatasetCategory_Evaluation, *req.Category)
				return &datasetdto.CreateDatasetResponse{
					DatasetID: gptr.Of(int64(999)),
					BaseResp:  &base.BaseResp{StatusCode: 0},
				}, nil
			})

		param := &rpc.CreateDatasetParam{
			SpaceID: 100,
			Name:    "test_dataset",
			EvaluationSetItems: &entity.EvaluationSetSchema{
				AppID: 1,
				FieldSchemas: []*entity.FieldSchema{
					{Key: "k1", Name: "n1", ContentType: "Text"},
				},
			},
		}
		id, err := adapter.CreateDataset(ctx, param)
		assert.NoError(t, err)
		assert.Equal(t, int64(999), id)
	})

	t.Run("rpc_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("rpc error"))

		param := &rpc.CreateDatasetParam{
			SpaceID: 100,
			Name:    "test",
			EvaluationSetItems: &entity.EvaluationSetSchema{
				AppID: 1,
			},
		}
		_, err := adapter.CreateDataset(ctx, param)
		assert.Error(t, err)
	})

	t.Run("nil_response", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().CreateDataset(gomock.Any(), gomock.Any()).
			Return(nil, nil)

		param := &rpc.CreateDatasetParam{
			SpaceID: 100,
			Name:    "test",
			EvaluationSetItems: &entity.EvaluationSetSchema{
				AppID: 1,
			},
		}
		_, err := adapter.CreateDataset(ctx, param)
		assert.Error(t, err)
	})
}

func TestParseImportSourceFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter, _ := newTestAdapter(ctrl)

	result, err := adapter.ParseImportSourceFile(ctx, &entity.ParseImportSourceFileParam{})
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ParseImportSourceFile not implemented")
}

func TestGetDatasetItemField(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter, _ := newTestAdapter(ctrl)

	result, err := adapter.GetDatasetItemField(ctx, &rpc.GetDatasetItemFieldParam{
		SpaceID:         1,
		EvaluationSetID: 2,
		ItemPK:          3,
		FieldName:       "test",
		FieldKey:        gptr.Of("key"),
	})
	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestQueryItemSnapshotMappings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter, _ := newTestAdapter(ctrl)

	mappings, syncDate, err := adapter.QueryItemSnapshotMappings(ctx, &rpc.QueryItemSnapshotMappingRequest{
		SpaceID:        1,
		DatasetID:      2,
		IsDraftVersion: false,
		VersionID:      gptr.Of(int64(3)),
	})
	assert.Nil(t, mappings)
	assert.Equal(t, "", syncDate)
	assert.NoError(t, err)
}

func TestCreateDatasetWithImport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	adapter, _ := newTestAdapter(ctrl)

	id, jobID, err := adapter.CreateDatasetWithImport(ctx, &rpc.CreateDatasetWithImportParam{
		SpaceID: 1,
		Name:    "test",
		Option: &entity.DatasetIOJobOption{
			OverwriteDataset: gptr.Of(true),
		},
	})
	assert.Equal(t, int64(0), id)
	assert.Equal(t, int64(0), jobID)
	assert.NoError(t, err)
}

func TestBatchUpdateDatasetItems(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nil_param", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, _ := newTestAdapter(ctrl)

		_, _, err := adapter.BatchUpdateDatasetItems(ctx, nil)
		assert.Error(t, err)
	})

	t.Run("not_implemented", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, _ := newTestAdapter(ctrl)

		_, _, err := adapter.BatchUpdateDatasetItems(ctx, &rpc.BatchUpdateDatasetItemsParam{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "BatchUpdateDatasetItems not implemented")
	})
}

func TestUpdateDataset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().UpdateDataset(gomock.Any(), gomock.Any()).
			Return(&datasetdto.UpdateDatasetResponse{
				BaseResp: &base.BaseResp{StatusCode: 0},
			}, nil)

		err := adapter.UpdateDataset(ctx, 1, 2, gptr.Of("name"), gptr.Of("desc"))
		assert.NoError(t, err)
	})

	t.Run("rpc_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().UpdateDataset(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("rpc error"))

		err := adapter.UpdateDataset(ctx, 1, 2, gptr.Of("name"), nil)
		assert.Error(t, err)
	})
}

func TestDeleteDataset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().DeleteDataset(gomock.Any(), gomock.Any()).
			Return(&datasetdto.DeleteDatasetResponse{
				BaseResp: &base.BaseResp{StatusCode: 0},
			}, nil)

		err := adapter.DeleteDataset(ctx, 1, 2)
		assert.NoError(t, err)
	})
}

func TestGetDataset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().GetDataset(gomock.Any(), gomock.Any()).
			Return(&datasetdto.GetDatasetResponse{
				Dataset:  &domain_dataset.Dataset{ID: 123, Name: gptr.Of("test")},
				BaseResp: &base.BaseResp{StatusCode: 0},
			}, nil)

		set, err := adapter.GetDataset(ctx, gptr.Of(int64(1)), 123, nil)
		assert.NoError(t, err)
		assert.NotNil(t, set)
		assert.Equal(t, int64(123), set.ID)
	})
}

func TestBatchDeleteDatasetItems(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().BatchDeleteDatasetItems(gomock.Any(), gomock.Any()).
			Return(&datasetdto.BatchDeleteDatasetItemsResponse{
				BaseResp: &base.BaseResp{StatusCode: 0},
			}, nil)

		err := adapter.BatchDeleteDatasetItems(ctx, 1, 2, []int64{3, 4})
		assert.NoError(t, err)
	})

	t.Run("rpc_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().BatchDeleteDatasetItems(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("rpc error"))

		err := adapter.BatchDeleteDatasetItems(ctx, 1, 2, []int64{3})
		assert.Error(t, err)
	})
}

func TestGetDatasetIOJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().GetDatasetIOJob(gomock.Any(), gomock.Any()).
			Return(&datasetdto.GetDatasetIOJobResponse{
				BaseResp: &base.BaseResp{StatusCode: 0},
			}, nil)

		job, err := adapter.GetDatasetIOJob(ctx, 1, 2)
		assert.NoError(t, err)
		assert.Nil(t, job)
	})

	t.Run("nil_response", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().GetDatasetIOJob(gomock.Any(), gomock.Any()).
			Return(nil, nil)

		_, err := adapter.GetDatasetIOJob(ctx, 1, 2)
		assert.Error(t, err)
	})
}

func TestClearEvaluationSetDraftItem(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().ClearDatasetItem(gomock.Any(), gomock.Any()).
			Return(&datasetdto.ClearDatasetItemResponse{}, nil)

		err := adapter.ClearEvaluationSetDraftItem(ctx, 1, 2)
		assert.NoError(t, err)
	})

	t.Run("rpc_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		adapter, mockClient := newTestAdapter(ctrl)

		mockClient.EXPECT().ClearDatasetItem(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("rpc error"))

		err := adapter.ClearEvaluationSetDraftItem(ctx, 1, 2)
		assert.Error(t, err)
	})
}
