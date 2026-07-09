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

// CountDataset 统计空间内 item_count 严格大于阈值(req.ItemCountGt)的数据集数量。
//
// 注意: item_count 存储在 Redis(见 IDatasetRepo.MGetItemCount),不是 MySQL 列,
// 因此无法直接用 SQL COUNT(item_count > ?) 聚合。这里的实现为:
//  1. 按 space + category 等条件分页拉取匹配的数据集主键(MySQL,已带软删除过滤);
//  2. 一次性 MGetItemCount 从 Redis 批量取 item_count(无 N+1);
//  3. 在内存中统计 item_count > 阈值 的数量。
func (s *DatasetServiceImpl) CountDataset(ctx context.Context, req *CountDatasetsParam) (int64, error) {
	if req == nil || req.SpaceID <= 0 {
		return 0, errno.InvalidParamErrorf("invalid workspace_id")
	}
	// 阈值严格大于语义: item_count > ItemCountGt;缺省 0 -> item_count > 0
	threshold := req.ItemCountGt
	if threshold < 0 {
		return 0, errno.InvalidParamErrorf("invalid item_count_gt=%d", threshold)
	}

	// 分页拉取空间内匹配的数据集主键。pageSize 取上限 200,循环跟随 cursor 直到取尽。
	const pageSize int32 = 200
	var (
		datasetIDs []int64
		cursor     string
	)
	for {
		pg := pagination.New(
			repo.DatasetOrderBy(""),
			pagination.WithPrePage(nil, gptr.Of(pageSize), gcond.If(cursor == "", (*string)(nil), gptr.Of(cursor))),
		)
		listParam := &repo.ListDatasetsParams{
			SpaceID:      req.SpaceID,
			IDs:          req.DatasetIDs,
			Category:     req.Category,
			CreatedBys:   req.CreatedBys,
			NameLike:     gptr.Indirect(req.Name),
			BizCategorys: req.BizCategorys,
			Paginator:    pg,
		}
		datasets, pr, err := s.repo.ListDatasets(ctx, listParam)
		if err != nil {
			return 0, err
		}
		for _, ds := range datasets {
			datasetIDs = append(datasetIDs, ds.ID)
		}
		if pr == nil || pr.Cursor == "" || len(datasets) == 0 {
			break
		}
		cursor = pr.Cursor
	}
	if len(datasetIDs) == 0 {
		return 0, nil
	}

	itemCountM, err := s.repo.MGetItemCount(ctx, datasetIDs...)
	if err != nil {
		return 0, err
	}
	var count int64
	for _, id := range datasetIDs {
		if itemCountM[id] > threshold {
			count++
		}
	}
	return count, nil
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

func (s *DatasetServiceImpl) buildNewDataset(d *entity.Dataset) {
	d.Status = gcond.If(d.Status == "", entity.DatasetStatusAvailable, d.Status)
	d.Visibility = gcond.If(d.Visibility == "", entity.DatasetVisibilitySpace, d.Visibility)
	d.SecurityLevel = gcond.If(d.SecurityLevel == "", entity.SecurityLevelL2, d.SecurityLevel)
	d.Features = gcond.If(d.Features == nil, s.featConfig().GetFeatureByCategory(d.Category), d.Features)
	d.Spec = gcond.If(d.Spec == nil, s.specConfig().GetSpecByCategory(d.Category), d.Spec)
}
