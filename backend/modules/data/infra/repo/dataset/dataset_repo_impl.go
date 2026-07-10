// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"

	"github.com/bytedance/gg/gslice"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/dataset/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/dataset/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/dataset/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func (d *DatasetRepo) SetItemCount(ctx context.Context, datasetID int64, n int64) error {
	return d.datasetRedisDAO.SetItemCount(ctx, datasetID, n)
}

func (d *DatasetRepo) IncrItemCount(ctx context.Context, datasetID int64, n int64) (int64, error) {
	return d.datasetRedisDAO.IncrItemCount(ctx, datasetID, n)
}

func (d *DatasetRepo) GetItemCount(ctx context.Context, datasetID int64) (int64, error) {
	return d.datasetRedisDAO.GetItemCount(ctx, datasetID)
}

func (d *DatasetRepo) MGetItemCount(ctx context.Context, datasetID ...int64) (map[int64]int64, error) {
	return d.datasetRedisDAO.MGetItemCount(ctx, datasetID...)
}

func (d *DatasetRepo) CreateDatasetAndSchema(ctx context.Context, dataset *entity.Dataset, fields []*entity.FieldSchema) error {
	var (
		ids, _  = d.idGen.GenMultiIDs(ctx, 2)
		idGenOk = len(ids) == 2
	)
	if idGenOk {
		dataset.ID = ids[0]
		dataset.SchemaID = ids[1]
	}

	if err := d.txDB.Transaction(ctx, func(tx *gorm.DB) error {
		opt := repo.WithTransaction(tx)
		// 插入 dataset
		po, err := convertor.DatasetDO2PO(dataset)
		if err != nil {
			return err
		}
		if err := d.datasetDAO.CreateDataset(ctx, po, Opt2DBOpt(opt)...); err != nil {
			return err
		}
		dataset.ID = po.ID
		dataset.CreatedAt, dataset.UpdatedAt = po.CreatedAt, po.UpdatedAt

		// 插入 schema
		schema := newSchemaOfDataset(dataset, fields)
		schemaPO, err := convertor.SchemaDO2PO(schema)
		if err != nil {
			return err
		}
		if err := d.schemaDAO.CreateSchema(ctx, schemaPO, Opt2DBOpt(opt)...); err != nil {
			return err
		}
		schema.ID, schema.CreatedAt = schemaPO.ID, schemaPO.CreatedAt

		if idGenOk {
			return nil
		}

		// idgen 未成功，更新 dataset 的 schemaID
		logs.CtxInfo(ctx, "previous idgen failed, update dataset schemaID to %d", schema.ID)
		patch := &model.Dataset{SchemaID: schema.ID, UpdatedBy: session.UserIDInCtxOrEmpty(ctx)}
		if err := d.datasetDAO.PatchDataset(ctx, patch, &model.Dataset{SpaceID: dataset.SpaceID, ID: dataset.ID}, Opt2DBOpt(opt)...); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (d *DatasetRepo) GetDataset(ctx context.Context, spaceID, id int64, opt ...repo.Option) (*entity.Dataset, error) {
	po, err := d.datasetDAO.GetDataset(ctx, spaceID, id, Opt2DBOpt(opt...)...)
	if err != nil {
		return nil, err
	}
	do, err := convertor.DatasetPO2DO(po)
	if err != nil {
		return nil, err
	}
	return do, nil
}

func (d *DatasetRepo) MGetDatasets(ctx context.Context, spaceID int64, ids []int64, opt ...repo.Option) ([]*entity.Dataset, error) {
	pos, err := d.datasetDAO.MGetDatasets(ctx, spaceID, ids, Opt2DBOpt(opt...)...)
	if err != nil {
		return nil, err
	}
	dos, err := gslice.TryMap(pos, convertor.DatasetPO2DO).Get()
	if err != nil {
		return nil, err
	}
	return dos, nil
}

func (d *DatasetRepo) PatchDataset(ctx context.Context, patch, where *entity.Dataset, opt ...repo.Option) error {
	patchPO, err := convertor.DatasetDO2PO(patch)
	if err != nil {
		return err
	}
	wherePO, err := convertor.DatasetDO2PO(where)
	if err != nil {
		return err
	}
	if err = d.datasetDAO.PatchDataset(ctx, patchPO, wherePO, Opt2DBOpt(opt...)...); err != nil {
		return err
	}
	return nil
}

func (d *DatasetRepo) DeleteDataset(ctx context.Context, spaceID, id int64, opt ...repo.Option) error {
	err := d.datasetDAO.DeleteDataset(ctx, spaceID, id, Opt2DBOpt(opt...)...)
	if err != nil {
		return err
	}
	return nil
}

func (d *DatasetRepo) ListDatasets(ctx context.Context, params *repo.ListDatasetsParams, opt ...repo.Option) ([]*entity.Dataset, *pagination.PageResult, error) {
	datasets, pr, err := d.datasetDAO.ListDatasets(ctx, &mysql.ListDatasetsParams{
		Paginator:    params.Paginator,
		SpaceID:      params.SpaceID,
		IDs:          params.IDs,
		Category:     string(params.Category),
		CreatedBys:   params.CreatedBys,
		NameLike:     params.NameLike,
		BizCategorys: params.BizCategorys,
	})
	if err != nil {
		return nil, nil, err
	}
	dos, err := gslice.TryMap(datasets, convertor.DatasetPO2DO).Get()
	if err != nil {
		return nil, nil, err
	}
	return dos, pr, nil
}

func (d *DatasetRepo) CountDatasets(ctx context.Context, params *repo.ListDatasetsParams, opt ...repo.Option) (int64, error) {
	count, err := d.datasetDAO.CountDatasets(ctx, &mysql.ListDatasetsParams{
		Paginator:    params.Paginator,
		SpaceID:      params.SpaceID,
		IDs:          params.IDs,
		Category:     string(params.Category),
		CreatedBys:   params.CreatedBys,
		NameLike:     params.NameLike,
		BizCategorys: params.BizCategorys,
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (d *DatasetRepo) ListDatasetIDs(ctx context.Context, params *repo.ListDatasetsParams, opt ...repo.Option) ([]int64, *pagination.PageResult, error) {
	ids, pr, err := d.datasetDAO.ListDatasetIDs(ctx, &mysql.ListDatasetsParams{
		Paginator:    params.Paginator,
		SpaceID:      params.SpaceID,
		IDs:          params.IDs,
		Category:     string(params.Category),
		CreatedBys:   params.CreatedBys,
		NameLike:     params.NameLike,
		BizCategorys: params.BizCategorys,
	})
	if err != nil {
		return nil, nil, err
	}
	return ids, pr, nil
}

func newSchemaOfDataset(dataset *entity.Dataset, fields []*entity.FieldSchema) *entity.DatasetSchema {
	immutable := false
	if dataset.Features != nil {
		immutable = !dataset.Features.EditSchema
	}

	return &entity.DatasetSchema{
		ID:        dataset.SchemaID,
		AppID:     dataset.AppID,
		SpaceID:   dataset.SpaceID,
		DatasetID: dataset.ID,
		Fields:    fields,
		Immutable: immutable,
		CreatedBy: dataset.CreatedBy,
		UpdatedBy: dataset.CreatedBy,
	}
}
