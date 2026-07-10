// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/bytedance/gg/gcond"
	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

func (s *DatasetServiceImpl) CreateDataset(ctx context.Context, dataset *entity.Dataset, fields []*entity.FieldSchema) error {
	s.buildNewDataset(dataset)
	genFieldKeys(fields)

	if err := validateSchema(dataset, fields); err != nil {
		return errno.InvalidParamErr(err)
	}
	if err := s.repo.CreateDatasetAndSchema(ctx, dataset, fields); err != nil {
		return err
	}
	return nil
}

func (s *DatasetServiceImpl) UpdateDataset(ctx context.Context, param *UpdateDatasetParam) error {
	ds, err := s.repo.GetDataset(ctx, param.SpaceID, param.DatasetID)
	if err != nil {
		return err
	}
	if ds == nil {
		return errno.NotFoundErrorf("dataset %d not found", param.DatasetID)
	}
	patch := &entity.Dataset{
		Name:        gcond.If(param.Name != "", param.Name, ds.Name),
		Description: gcond.If(param.Description != nil, param.Description, ds.Description),
		UpdatedBy:   param.UpdatedBy,
	}

	if err = s.repo.PatchDataset(ctx, patch, &entity.Dataset{SpaceID: ds.SpaceID, ID: ds.ID}); err != nil {
		return err
	}
	return nil
}

func (s *DatasetServiceImpl) DeleteDataset(ctx context.Context, spaceID, id int64) error {
	dataset, err := s.repo.GetDataset(ctx, spaceID, id)
	if err != nil {
		return err
	}
	if dataset == nil {
		return errno.NotFoundErrorf("dataset=%d is not found", id)
	}
	if err = s.repo.DeleteDataset(ctx, spaceID, id); err != nil {
		return err
	}
	return nil
}

func (s *DatasetServiceImpl) GetDataset(ctx context.Context, spaceID, id int64) (*DatasetWithSchema, error) {
	return s.GetDatasetWithOpt(ctx, spaceID, id, &GetOpt{})
}

func (s *DatasetServiceImpl) BatchGetDataset(ctx context.Context, spaceID int64, ids []int64) ([]*DatasetWithSchema, error) {
	return s.BatchGetDatasetWithOpt(ctx, spaceID, ids, &GetOpt{})
}

func (s *DatasetServiceImpl) GetDatasetWithOpt(ctx context.Context, spaceID, id int64, opt *GetOpt) (*DatasetWithSchema, error) {
	// 1. 获取对应的 dataset
	var opts []repo.Option
	if opt.WithDeleted {
		opts = append(opts, repo.WithDeleted())
	}
	dataset, err := s.repo.GetDataset(ctx, spaceID, id, opts...)
	if err != nil {
		return nil, err
	}

	// 2. 根据 dataset 获取对应的 schema
	schema, err := s.repo.GetSchema(ctx, spaceID, dataset.SchemaID, opts...)
	if err != nil {
		return nil, err
	}

	return &DatasetWithSchema{Dataset: dataset, Schema: schema}, nil
}

func (s *DatasetServiceImpl) BatchGetDatasetWithOpt(ctx context.Context, spaceID int64, ids []int64, opt *GetOpt) ([]*DatasetWithSchema, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	const batchGetLimit = 100
	if len(ids) > batchGetLimit {
		return nil, errno.InternalErrorf("exceed the batch get limit of dataset, len(ids)=%d, batchGetLimit=%d", len(ids), batchGetLimit)
	}
	var opts []repo.Option
	if opt.WithDeleted {
		opts = append(opts, repo.WithDeleted())
	}
	datasets, err := s.repo.MGetDatasets(ctx, spaceID, ids, opts...)
	if err != nil {
		return nil, err
	}
	schemaIDs := gslice.Map(datasets, func(d *entity.Dataset) int64 { return d.SchemaID })
	schemas, err := s.repo.MGetSchema(ctx, spaceID, schemaIDs)
	if err != nil {
		return nil, err
	}
	schemaM := gslice.ToMap(schemas, func(s *entity.DatasetSchema) (int64, *entity.DatasetSchema) { return s.ID, s })

	return gslice.Map(datasets, func(d *entity.Dataset) *DatasetWithSchema {
		return &DatasetWithSchema{Schema: schemaM[d.SchemaID], Dataset: d}
	}), nil
}

func (s *DatasetServiceImpl) SearchDataset(ctx context.Context, req *SearchDatasetsParam) (*SearchDatasetsResults, error) {
	datasetsParam := s.buildSearchDatasetParam(req)
	datasets, pr, err := s.repo.ListDatasets(ctx, datasetsParam)
	if err != nil {
		return nil, err
	}
	total, err := s.repo.CountDatasets(ctx, datasetsParam)
	if err != nil {
		return nil, err
	}
	res := &SearchDatasetsResults{
		NextCursor: pr.Cursor,
		Total:      total,
	}
	if len(datasets) == 0 {
		return res, nil
	}
	// 获取对应的 schema
	schemaIDs := gslice.Map(datasets, func(dataset *entity.Dataset) int64 { return dataset.SchemaID })
	schemas, err := s.repo.MGetSchema(ctx, req.SpaceID, schemaIDs)
	if err != nil {
		return nil, err
	}
	schemaM := gslice.ToMap(schemas, func(schema *entity.DatasetSchema) (int64, *entity.DatasetSchema) { return schema.ID, schema })
	res.DatasetWithSchemas = gslice.Map(datasets, func(dataset *entity.Dataset) *DatasetWithSchema {
		return &DatasetWithSchema{
			Dataset: dataset,
			Schema:  schemaM[dataset.SchemaID],
		}
	})
	return res, nil
}

func (s *DatasetServiceImpl) buildSearchDatasetParam(req *SearchDatasetsParam) *repo.ListDatasetsParams {
	pg := pagination.New(
		repo.DatasetOrderBy(gptr.Indirect(req.OrderBy.Field)),
		pagination.WithOrderByAsc(gptr.Indirect(req.OrderBy.IsAsc)),
		pagination.WithPrePage(req.Page, req.PageSize, req.Cursor),
	)
	param := &repo.ListDatasetsParams{
		IDs:          req.DatasetIDs,
		SpaceID:      req.SpaceID,
		Category:     req.Category,
		CreatedBys:   req.CreatedBys,
		NameLike:     gptr.Indirect(req.Name),
		Paginator:    pg,
		BizCategorys: req.BizCategorys,
	}
	return param
}

const (
	// countDatasetsPageSize 有界游标分页每页固定大小：每次只 SELECT id 一页，峰值内存只限单页。
	countDatasetsPageSize = 100
	// countDatasetsMaxScan 单次统计允许扫描的评测集 id 总量安全上限。
	// 触顶必须显式报错，不得返回截断后的不精确 count。
	countDatasetsMaxScan = 100000
)

// CountDatasetsAboveItemCount 统计空间内 item_count 严格大于阈值的数据集数量。
// 有界游标分页枚举 id（每页只 SELECT id），逐页批量读取 Redis item_count，
// 内存中按阈值严格大于过滤并增量累加，不物化完整 Dataset DTO、不持有全量 id。
func (s *DatasetServiceImpl) CountDatasetsAboveItemCount(ctx context.Context, req *CountDatasetsParam) (int64, error) {
	if req == nil || req.SpaceID <= 0 {
		return 0, errno.InvalidParamErrorf("invalid workspace_id for count datasets")
	}
	if req.ItemCountGt < 0 {
		return 0, errno.InvalidParamErrorf("invalid item_count_gt for count datasets: %d", req.ItemCountGt)
	}

	var (
		count   int64
		scanned int64
		cursor  string
	)
	for {
		pg := pagination.New(
			pagination.WithOrderBy(pagination.ColumnID),
			pagination.WithLimit(countDatasetsPageSize),
			pagination.WithCursor(cursor),
		)
		params := &repo.ListDatasetsParams{
			SpaceID:   req.SpaceID,
			Category:  req.Category,
			Paginator: pg,
		}
		ids, pr, err := s.repo.ListDatasetIDs(ctx, params)
		if err != nil {
			return 0, err
		}
		if len(ids) == 0 {
			break
		}
		scanned += int64(len(ids))
		if scanned > countDatasetsMaxScan {
			// 触顶显式报错，避免返回不精确的截断 count。
			return 0, errno.InternalErrorf("count datasets exceeds max scan limit %d, space_id=%d", countDatasetsMaxScan, req.SpaceID)
		}

		itemCounts, err := s.repo.MGetItemCount(ctx, ids...)
		if err != nil {
			return 0, err
		}
		for _, id := range ids {
			if itemCounts[id] > req.ItemCountGt {
				count++
			}
		}

		if pr == nil || pr.Cursor == "" {
			break
		}
		cursor = pr.Cursor
	}
	return count, nil
}

func (s *DatasetServiceImpl) buildNewDataset(d *entity.Dataset) {
	d.Status = gcond.If(d.Status == "", entity.DatasetStatusAvailable, d.Status)
	d.Visibility = gcond.If(d.Visibility == "", entity.DatasetVisibilitySpace, d.Visibility)
	d.SecurityLevel = gcond.If(d.SecurityLevel == "", entity.SecurityLevelL2, d.SecurityLevel)
	d.Features = gcond.If(d.Features == nil, s.featConfig().GetFeatureByCategory(d.Category), d.Features)
	d.Spec = gcond.If(d.Spec == nil, s.specConfig().GetSpecByCategory(d.Category), d.Spec)
}
