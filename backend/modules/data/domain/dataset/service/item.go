// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"

	"gorm.io/gorm"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo"
	common_entity "github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/consts"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func (s *DatasetServiceImpl) LoadItemData(ctx context.Context, items ...*entity.Item) error {
	// load data
	byStorage := gslice.GroupBy(items, func(i *entity.Item) common_entity.Provider {
		props := i.GetOrBuildProperties()
		return props.Storage
	}) // ignore_security_alert SQL_INJECTION

	for provider, items := range byStorage {
		switch provider {
		case common_entity.ProviderS3, common_entity.ProviderAbase:
			if err := s.repo.MGetItemData(ctx, items, provider); err != nil {
				return errors.WithMessage(err, "get item data in os")
			}
		default:
		}
	}

	// todo: sign attachments url
	return nil
}

func (s *DatasetServiceImpl) ArchiveAndCreateItem(ctx context.Context, ds *DatasetWithSchema, oldID int64, item *entity.Item) error {
	release, err := s.withWriteItemBarrier(ctx, ds.ID, 1)
	if err != nil {
		return err
	}
	defer func() { release() }()

	if err := s.saveItemData(ctx, item); err != nil {
		return err
	}
	if err := s.txDB.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []repo.Option{repo.WithTransaction(tx), repo.WithMaster()}

		if err := s.touchDatasetForWriteItem(ctx, ds, opts); err != nil {
			return err
		}

		if err := s.repo.ArchiveItems(ctx, ds.SpaceID, ds.NextVersionNum, []int64{oldID}, opts...); err != nil {
			return errors.WithMessage(err, "archive item")
		}

		item.ID = 0
		if _, err := s.repo.MCreateItems(ctx, []*entity.Item{item}, opts...); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *DatasetServiceImpl) UpdateItem(ctx context.Context, ds *DatasetWithSchema, item *entity.Item) error {
	release, err := s.withWriteItemBarrier(ctx, ds.ID, 1)
	if err != nil {
		return err
	}
	defer func() { release() }()

	if err := s.saveItemData(ctx, item); err != nil {
		return err
	}
	if err := s.txDB.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []repo.Option{repo.WithTransaction(tx), repo.WithMaster()}

		if err := s.touchDatasetForWriteItem(ctx, ds, opts); err != nil {
			return err
		}

		if err := s.repo.UpdateItem(ctx, item, opts...); err != nil {
			return errno.DBErr(err, "update item")
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *DatasetServiceImpl) BatchDeleteItems(ctx context.Context, ds *DatasetWithSchema, items ...*entity.Item) error {
	idsToArchive := gslice.FilterMap(items, func(item *entity.Item) (int64, bool) {
		return item.ID, item.AddVN != ds.NextVersionNum
	})
	idsToDelete := gslice.FilterMap(items, func(item *entity.Item) (int64, bool) {
		return item.ID, item.AddVN == ds.NextVersionNum
	})

	release, err := s.withWriteItemBarrier(ctx, ds.ID, int64(len(idsToArchive)+len(idsToDelete)))
	if err != nil {
		return err
	}
	defer func() { release() }()

	if err := s.txDB.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []repo.Option{repo.WithTransaction(tx), repo.WithMaster()}

		if err := s.touchDatasetForWriteItem(ctx, ds, opts); err != nil {
			return err
		}

		if err := s.repo.ArchiveItems(ctx, ds.SpaceID, ds.NextVersionNum, idsToArchive, opts...); err != nil {
			return errors.WithMessage(err, "archive items")
		}

		if err := s.repo.DeleteItems(ctx, ds.SpaceID, idsToDelete, opts...); err != nil {
			return errors.WithMessage(err, "delete items")
		}
		return nil
	}); err != nil {
		return err
	}

	n, err := s.repo.IncrItemCount(ctx, ds.ID, -int64(len(items)))
	if err != nil {
		return err
	}

	logs.CtxInfo(ctx, "delete %d items, archive_ids=%v, delete_ids=%v, item_count=%d", len(items), idsToArchive, idsToDelete, n)
	return nil
}

func (s *DatasetServiceImpl) ClearDataset(ctx context.Context, ds *DatasetWithSchema) error {
	logs.CtxInfo(ctx, "dataset %d will be cleared, space_id=%d, name=%s, vn=%d", ds.ID, ds.SpaceID, ds.Name, ds.NextVersionNum)
	release, err := s.withWriteItemBarrier(ctx, ds.ID, 1) // todo: nice item count
	if err != nil {
		return err
	}
	defer func() { release() }()

	if err := s.txDB.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []repo.Option{repo.WithTransaction(tx), repo.WithMaster()}

		patch := &entity.Dataset{
			LastOperation: entity.DatasetOpTypeClearDataset,
			UpdatedBy:     ds.UpdatedBy,
		}
		if err := s.repo.PatchDataset(ctx, patch, repo.NewDatasetWhere(ds.SpaceID, ds.ID), opts...); err != nil {
			return err
		}

		if _, err := s.repo.ClearDataset(ctx, ds.SpaceID, ds.ID, ds.NextVersionNum); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	if err := s.repo.SetItemCount(ctx, ds.ID, 0); err != nil {
		return err
	}
	return nil
}

func (s *DatasetServiceImpl) GetItem(ctx context.Context, spaceID, datasetID, itemID int64) (*entity.Item, error) {
	query := repo.NewListItemsParamsOfDataset(spaceID, datasetID, func(p *repo.ListItemsParams) { p.ItemIDs = []int64{itemID} })
	items, _, err := s.repo.ListItems(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errno.NotFoundErrorf(`item %d not found`, itemID)
	}
	return items[0], nil
}

func (s *DatasetServiceImpl) BatchGetItems(ctx context.Context, spaceID, datasetID int64, itemIDs []int64) ([]*entity.Item, error) {
	query := repo.NewListItemsParamsOfDataset(spaceID, datasetID, func(p *repo.ListItemsParams) { p.ItemIDs = itemIDs })
	items, _, err := s.repo.ListItems(ctx, query)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *DatasetServiceImpl) BatchCreateItems(ctx context.Context, ds *DatasetWithSchema, iitems []*IndexedItem, opt *MAddItemOpt) (added []*IndexedItem, err error) {
	if len(iitems) == 0 {
		return nil, nil
	}

	items := gslice.Map(iitems, func(i *IndexedItem) *entity.Item { return i.Item })
	if err := s.buildNewItems(ctx, ds, items); err != nil {
		return nil, err
	}
	n, err := s.acquireItemCount(ctx, ds, int64(len(items)), opt.PartialAdd)
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		return nil, nil
	}
	// todo: 幂等 key 与历史版本的 item 冲突处理
	added = iitems[:n]
	count, err := s.mCreateItems(ctx, ds, items[:n])
	if err != nil {
		added = nil // err 不为空时也需返还 count，此处不可 return
	}
	if diff := n - count; diff > 0 {
		if _, err := s.repo.IncrItemCount(ctx, ds.ID, -diff); err != nil {
			return added, errors.WithMessage(err, "decrease item count by %d")
		}
	}

	return added, err
}

func (s *DatasetServiceImpl) buildNewItems(ctx context.Context, ds *DatasetWithSchema, items []*entity.Item) error {
	ids, err := s.idgen.GenMultiIDs(ctx, len(items))
	if err != nil {
		return errors.Wrap(err, "generate item ids")
	}
	if len(ids) != len(items) {
		return errors.Errorf("generate %d ids, got %d", len(items), len(ids))
	}

	for i, item := range items {
		item.ID = ids[i]
		item.AppID = ds.AppID
		item.SpaceID = ds.SpaceID
		item.DatasetID = ds.ID
		item.SchemaID = ds.Schema.ID
		item.ItemID = item.ID
		item.AddVN = ds.NextVersionNum
		item.DelVN = consts.MaxVersionNum
		item.BuildProperties()
		if item.ItemKey == "" {
			item.ItemKey = strconv.FormatInt(item.ItemID, 10) // 使用 ItemID 作为幂等 key，避免 DB UK 字段为空。
		}
	}
	return nil
}

func (s *DatasetServiceImpl) acquireItemCount(ctx context.Context, ds *DatasetWithSchema, want int64, partial bool) (got int64, _ error) {
	total, err := s.repo.IncrItemCount(ctx, ds.ID, want)
	if err != nil {
		return 0, errors.WithMessagef(err, "incr item count by %d", want)
	}
	if debt := total - ds.Spec.MaxItemCount; debt > 0 { // 超限需返还
		if debt > want {
			debt = want
		}
		if !partial {
			debt = want
		}
		logs.CtxInfo(ctx, "dataset capacity exceeded, decrease by %d, dataset_id=%d", debt, ds.ID)
		_, err := s.repo.IncrItemCount(ctx, ds.ID, -debt)
		if err != nil {
			return 0, errors.WithMessage(err, "decrease item count by %d")
		}
		return want - debt, nil
	}
	return want, nil
}

func (s *DatasetServiceImpl) mCreateItems(ctx context.Context, ds *DatasetWithSchema, items []*entity.Item) (added int64, err error) {
	release, err := s.withWriteItemBarrier(ctx, ds.ID, int64(len(items)))
	if err != nil {
		return 0, err
	}
	defer func() { release() }()

	added, err = s.saveItems(ctx, ds, items)
	if err != nil {
		return 0, err
	}
	if int64(len(items)) != added {
		logs.CtxInfo(ctx, "add %d items, %d added, conflict keys may happened, item reloaded", len(items), added)
		if err := s.reloadConflictItems(ctx, ds, items); err != nil {
			return added, err
		}
	}
	return added, nil
}

func (s *DatasetServiceImpl) saveItems(ctx context.Context, ds *DatasetWithSchema, items []*entity.Item) (count int64, err error) {
	if err := s.saveItemData(ctx, items...); err != nil {
		return 0, err
	}

	if err := s.txDB.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []repo.Option{repo.WithTransaction(tx), repo.WithMaster()}
		if err := s.touchDatasetForWriteItem(ctx, ds, opts); err != nil {
			return err
		}

		count, err = s.repo.MCreateItems(ctx, items, opts...)
		if err != nil {
			return errors.WithMessagef(err, "create items, dataset_id=%d", ds.ID)
		}
		return nil
	}); err != nil {
		return 0, errno.MaybeDBErr(err)
	}
	return count, nil
}

func (s *DatasetServiceImpl) saveItemData(ctx context.Context, items ...*entity.Item) error {
	oldConf := s.storageConfig()
	cfg := &conf.DatasetItemStorage{}
	err := copier.Copy(cfg, oldConf)
	if err != nil {
		return err
	}
	gslice.SortBy(cfg.Providers, func(c1 *conf.DatasetItemProviderConfig, c2 *conf.DatasetItemProviderConfig) bool {
		return c1.MaxSize < c2.MaxSize
	})
	m := make(map[common_entity.Provider][]*entity.Item)
	for i, item := range items {
		size := item.GetOrBuildProperties().Bytes
		p, ok := gslice.Find(cfg.Providers, func(p *conf.DatasetItemProviderConfig) bool { return p.MaxSize >= size }).Get()
		if !ok {
			return errors.Errorf(`no provider found for item, index=%d, size=%d`, i, size)
		}
		props := item.GetOrBuildProperties()
		props.Storage = p.Provider
		if p.Provider != common_entity.ProviderRDS {
			props.StorageKey = FormatDatasetItemDataKey(item.DatasetID, item.ItemID, item.AddVN)
		}
		m[p.Provider] = append(m[p.Provider], item)
	}

	for provider, items := range m {
		switch provider {
		case common_entity.ProviderS3, common_entity.ProviderAbase:
			if _, err := s.repo.MSetItemData(ctx, items, provider); err != nil {
				return errors.WithMessage(err, "repo.MSetItemData")
			}
			for _, item := range items {
				item.ClearData()
			}
		case common_entity.ProviderRDS:
			// 无需提前处理
		default:
			return errors.Errorf(`unsupported item storage provider '%s'`, provider)
		}
	}

	return nil
}

func (s *DatasetServiceImpl) touchDatasetForWriteItem(ctx context.Context, ds *DatasetWithSchema, opts []repo.Option) error {
	where := repo.NewDatasetWhere(ds.SpaceID, ds.ID, func(d *entity.Dataset) {
		d.NextVersionNum = ds.NextVersionNum // 防止版本更新
	})
	patch := &entity.Dataset{LastOperation: entity.DatasetOpTypeWriteItem, UpdatedBy: ds.UpdatedBy}
	if err := s.repo.PatchDataset(ctx, patch, where, opts...); err != nil {
		return err
	}

	// check schema
	schema, err := s.repo.GetSchema(ctx, ds.SpaceID, ds.Schema.ID, opts...)
	if err != nil {
		return errors.WithMessagef(err, "get schema, dataset_id=%d, schema_id=%d", ds.ID, ds.Schema.ID)
	}
	if schema.UpdateVersion != ds.Schema.UpdateVersion {
		return errno.Errorf(errno.ConcurrentDatasetOperationsCode, `schema updated during item updating, pre_version=%d, cur_version=%d`, ds.Schema.UpdateVersion, schema.UpdateVersion)
	}
	return nil
}

func (s *DatasetServiceImpl) reloadConflictItems(ctx context.Context, ds *DatasetWithSchema, items []*entity.Item) error {
	keys := gslice.FilterMap(items, func(i *entity.Item) (string, bool) {
		itemID := strconv.FormatInt(i.ItemID, 10)
		return i.ItemKey, itemID != i.ItemKey // 系统指定的 ItemKey 为 ItemID，此处仅查询用户指定 ItemKey 的 item
	})

	loaded, _, err := s.repo.ListItems(ctx, &repo.ListItemsParams{
		SpaceID:   ds.SpaceID,
		DatasetID: ds.ID,
		ItemKeys:  keys,
		AddVNEq:   ds.NextVersionNum,
	})
	if err != nil {
		return errors.WithMessage(err, "reload conflict items")
	}
	logs.CtxInfo(ctx, "reload %d conflict items, dataset_id=%d", len(loaded), ds.ID)

	m := gslice.ToMap(loaded, func(i *entity.Item) (string, *entity.Item) { return i.ItemKey, i })
	for _, item := range items {
		l, ok := m[item.ItemKey]
		if ok {
			item.ItemID = l.ItemID
			item.ID = l.ID
		}
	}
	return nil
}

func (s *DatasetServiceImpl) ValidateDatasetItems(ctx context.Context, param *ValidateDatasetItemsParam) (*ValidateDatasetItemsResult, error) {
	if len(param.Items) == 0 {
		return &ValidateDatasetItemsResult{}, nil
	}

	ds, err := s.buildDatasetForValidate(ctx, param)
	if err != nil {
		return nil, err
	}

	// 格式校验
	SanitizeInputItem(ds, param.Items...)
	goodItems, badItems := ValidateItems(ds, param.Items)

	// 容量校验
	var used int64
	if param.DatasetID > 0 && !param.IgnoreCurrentItemCount && len(goodItems) > 0 {
		used, err = s.repo.GetItemCount(ctx, param.DatasetID)
		if err != nil {
			return nil, err
		}
	}

	remaining := ds.Spec.MaxItemCount - used
	diff := remaining - int64(len(goodItems))
	if diff < 0 {
		eg := &entity.ItemErrorGroup{
			Type:       gptr.Of(entity.ItemErrorType_ExceedDatasetCapacity),
			Summary:    gptr.Of(fmt.Sprintf("capacity=%d, current=%d, to_add=%d", ds.Spec.MaxItemCount, used, len(goodItems))),
			ErrorCount: gptr.Of(int32(-diff)),
			Details: gslice.Map(goodItems[remaining:], func(i *IndexedItem) *entity.ItemErrorDetail {
				return &entity.ItemErrorDetail{Index: gptr.Of(int32(i.Index))}
			}),
		}
		badItems = append(badItems, eg)
		goodItems = goodItems[:remaining]
	}

	return &ValidateDatasetItemsResult{
		ValidItemIndices: gslice.Map(goodItems, func(i *IndexedItem) int32 { return int32(i.Index) }),
		ErrorGroups:      badItems,
	}, nil
}

func (s *DatasetServiceImpl) buildDatasetForValidate(ctx context.Context, param *ValidateDatasetItemsParam) (*DatasetWithSchema, error) {
	if param.DatasetID == 0 {
		if param.DatasetCategory == "" {
			return nil, errno.BadReqErrorf("dataset_id is required")
		}
		if len(param.DatasetFields) == 0 {
			return nil, errno.BadReqErrorf("dataset_fields is required")
		}

		ds := &DatasetWithSchema{
			Dataset: &entity.Dataset{
				SpaceID:  param.SpaceID,
				Category: param.DatasetCategory,
			},
			Schema: &entity.DatasetSchema{
				Fields: param.DatasetFields,
			},
		}
		s.buildNewDataset(ds.Dataset)
		return ds, nil
	}

	dataset, err := s.repo.GetDataset(ctx, param.SpaceID, param.DatasetID)
	if err != nil {
		return nil, errors.WithMessagef(err, "get dataset, space_id=%d, dataset_id=%d", param.SpaceID, param.DatasetID)
	}

	if len(param.DatasetFields) > 0 {
		return &DatasetWithSchema{
			Dataset: dataset,
			Schema:  &entity.DatasetSchema{Fields: param.DatasetFields},
		}, nil
	}
	schema, err := s.repo.GetSchema(ctx, param.SpaceID, dataset.SchemaID)
	if err != nil {
		return nil, errors.WithMessagef(err, "get schema, space_id=%d, dataset_id=%d, schema_id=%d", param.SpaceID, param.DatasetID, dataset.SchemaID)
	}
	return &DatasetWithSchema{Dataset: dataset, Schema: schema}, nil
}
