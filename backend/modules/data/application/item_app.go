// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/collection/set"
	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/pkg/errors"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	idl "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	convertor "github.com/coze-dev/coze-loop/backend/modules/data/application/convertor/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/service"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/consts"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func (h *DatasetApplicationImpl) BatchCreateDatasetItems(ctx context.Context, req *dataset.BatchCreateDatasetItemsRequest) (resp *dataset.BatchCreateDatasetItemsResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionEdit)
	if err != nil {
		return nil, err
	}
	rc, err := h.prepare(ctx, req)
	if err != nil {
		return
	}
	if rc != nil && rc.ds != nil {
		rc.ds.UpdatedBy = session.UserIDInCtxOrEmpty(ctx)
	}
	if len(rc.goodItems) == 0 {
		return h.buildResp(rc, nil), nil
	}

	added, err := h.svc.BatchCreateItems(ctx, rc.ds, rc.goodItems, &service.MAddItemOpt{PartialAdd: req.GetAllowPartialAdd()})
	if err != nil {
		return nil, err
	}
	return h.buildResp(rc, added), nil
}

func (h *DatasetApplicationImpl) UpdateDatasetItem(ctx context.Context, req *dataset.UpdateDatasetItemRequest) (resp *dataset.UpdateDatasetItemResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionEdit)
	if err != nil {
		return nil, err
	}
	ds, err := h.svc.GetDataset(ctx, req.GetWorkspaceID(), req.GetDatasetID())
	if err != nil {
		return nil, err
	}
	ds.UpdatedBy = session.UserIDInCtxOrEmpty(ctx)
	item, err := h.svc.GetItem(ctx, req.GetWorkspaceID(), req.GetDatasetID(), req.GetItemID())
	if err != nil {
		return nil, err
	}

	var (
		oldID   = item.ID
		inPlace = item.AddVN == ds.NextVersionNum
	)

	h.buildItem(ctx, req, ds, item, inPlace)
	if err := service.ValidateItem(ds, item); err != nil {
		return nil, err
	}
	if inPlace {
		logs.CtxInfo(ctx, "update item in place, id=%d, item_id=%d", ds.ID, item.ID)
		err = h.svc.UpdateItem(ctx, ds, item)
	} else {
		logs.CtxInfo(ctx, "archive and create item, id=%d, item_id=%d", ds.ID, item.ID)
		err = h.svc.ArchiveAndCreateItem(ctx, ds, oldID, item)
	}
	if err != nil {
		return nil, err
	}

	return &dataset.UpdateDatasetItemResponse{}, nil
}

func (h *DatasetApplicationImpl) DeleteDatasetItem(ctx context.Context, req *dataset.DeleteDatasetItemRequest) (resp *dataset.DeleteDatasetItemResponse, err error) {
	var (
		spaceID   = req.GetWorkspaceID()
		datasetID = req.GetDatasetID()
		itemID    = req.GetItemID()
	)
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionEdit)
	if err != nil {
		return nil, err
	}
	ds, err := h.svc.GetDataset(ctx, spaceID, datasetID)
	if err != nil {
		return nil, err
	}
	ds.UpdatedBy = session.UserIDInCtxOrEmpty(ctx)
	item, err := h.svc.GetItem(ctx, spaceID, datasetID, itemID)
	if err != nil {
		return nil, err
	}
	if err := h.svc.BatchDeleteItems(ctx, ds, item); err != nil {
		return nil, err
	}

	logs.CtxInfo(ctx, "delete dataset item success, space_id=%d, dataset_id=%d, item_id=%d", spaceID, datasetID, itemID)
	return &dataset.DeleteDatasetItemResponse{}, nil
}

func (h *DatasetApplicationImpl) BatchDeleteDatasetItems(ctx context.Context, req *dataset.BatchDeleteDatasetItemsRequest) (resp *dataset.BatchDeleteDatasetItemsResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionEdit)
	if err != nil {
		return nil, err
	}
	ds, err := h.svc.GetDataset(ctx, req.GetWorkspaceID(), req.GetDatasetID())
	if err != nil {
		return nil, err
	}
	ds.UpdatedBy = session.UserIDInCtxOrEmpty(ctx)
	items, err := h.svc.BatchGetItems(ctx, req.GetWorkspaceID(), req.GetDatasetID(), req.GetItemIds())
	if err != nil {
		return nil, err
	}

	if err := h.svc.BatchDeleteItems(ctx, ds, items...); err != nil {
		return nil, err
	}
	return &dataset.BatchDeleteDatasetItemsResponse{}, nil
}

func (h *DatasetApplicationImpl) ListDatasetItems(ctx context.Context, req *dataset.ListDatasetItemsRequest) (resp *dataset.ListDatasetItemsResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionRead)
	if err != nil {
		return nil, err
	}
	ds, err := h.svc.GetDataset(ctx, req.GetWorkspaceID(), req.DatasetID)
	if err != nil {
		return nil, err
	}

	total, err := h.repo.GetItemCount(ctx, req.DatasetID)
	if err != nil {
		return nil, err
	}
	defer func() { h.fixTotal(req, resp) }()
	orderBy := &service.OrderBy{}
	if len(req.GetOrderBys()) != 0 {
		orderBy = &service.OrderBy{
			Field: gptr.Of(req.GetOrderBys()[0].GetField()),
			IsAsc: gptr.Of(req.GetOrderBys()[0].GetIsAsc()),
		}
	}
	// TODO: 改为超过固定数值不允许 offset 访问
	query := repo.NewListItemsParamsOfDataset(req.GetWorkspaceID(), req.GetDatasetID(), func(p *repo.ListItemsParams) {
		p.Paginator = pagination.New(
			repo.ItemOrderBy(gptr.Indirect(orderBy.Field)),
			pagination.WithOrderByAsc(gptr.Indirect(orderBy.IsAsc)),
			pagination.WithPrePage(req.PageNumber, req.PageSize, req.PageToken),
		)
	})
	items, pr, err := h.repo.ListItems(ctx, query)
	if err != nil {
		return nil, err
	}
	if err := h.svc.LoadItemData(ctx, items...); err != nil {
		return nil, err
	}

	service.SanitizeOutputItem(ds.Schema, items)
	return &dataset.ListDatasetItemsResponse{
		Items:         gslice.Map(items, convertor.ItemDO2DTO),
		Total:         gptr.Of(total),
		NextPageToken: gptr.Of(pr.Cursor),
	}, nil
}

func (h *DatasetApplicationImpl) ListDatasetItemsByVersion(ctx context.Context, req *dataset.ListDatasetItemsByVersionRequest) (resp *dataset.ListDatasetItemsByVersionResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionRead)
	if err != nil {
		return nil, err
	}
	version, err := h.repo.GetVersion(ctx, req.GetWorkspaceID(), req.GetVersionID())
	if err != nil {
		return nil, errors.WithMessage(err, "repo.GetVersion")
	}

	items, pr, err := h.listItemsByVersion(ctx, req, version)
	if err != nil {
		return nil, errors.WithMessage(err, "repo.ListItems")
	}
	if err := h.svc.LoadItemData(ctx, items...); err != nil {
		return nil, errors.WithMessage(err, "svc.LoadItemData")
	}

	schema, err := h.repo.GetSchema(ctx, req.GetWorkspaceID(), version.SchemaID)
	if err != nil {
		return nil, err
	}
	service.SanitizeOutputItem(schema, items)

	total, err := h.svc.GetOrSetItemCountOfVersion(ctx, version)
	if err != nil {
		return nil, err
	}
	return &dataset.ListDatasetItemsByVersionResponse{
		Items:         gslice.Map(items, convertor.ItemDO2DTO),
		Total:         gptr.Of(total),
		NextPageToken: gptr.Of(pr.Cursor),
	}, nil
}

func (h *DatasetApplicationImpl) GetDatasetItem(ctx context.Context, req *dataset.GetDatasetItemRequest) (resp *dataset.GetDatasetItemResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionRead)
	if err != nil {
		return nil, err
	}
	item, err := h.svc.GetItem(ctx, req.GetWorkspaceID(), req.GetDatasetID(), req.GetItemID())
	if err != nil {
		return nil, err
	}

	if err := h.svc.LoadItemData(ctx, item); err != nil {
		return nil, err
	}
	return &dataset.GetDatasetItemResponse{Item: convertor.ItemDO2DTO(item)}, nil
}

func (h *DatasetApplicationImpl) BatchGetDatasetItems(ctx context.Context, req *dataset.BatchGetDatasetItemsRequest) (resp *dataset.BatchGetDatasetItemsResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionRead)
	if err != nil {
		return nil, err
	}
	ds, err := h.svc.GetDataset(ctx, req.GetWorkspaceID(), req.DatasetID)
	if err != nil {
		return nil, err
	}
	if ds == nil {
		return nil, errno.NotFoundErrorf("dataset=%d of space=%d is not found", req.DatasetID, req.GetWorkspaceID())
	}
	if len(req.ItemIds) == 0 {
		return &dataset.BatchGetDatasetItemsResponse{}, nil
	}

	query := repo.NewListItemsParamsOfDataset(req.GetWorkspaceID(), req.GetDatasetID(), func(p *repo.ListItemsParams) {
		p.ItemIDs = req.ItemIds
	})
	items, _, err := h.repo.ListItems(ctx, query)
	if err != nil {
		return nil, err
	}

	if err := h.svc.LoadItemData(ctx, items...); err != nil {
		return nil, err
	}
	service.SanitizeOutputItem(ds.Schema, items)
	return &dataset.BatchGetDatasetItemsResponse{Items: gslice.Map(items, convertor.ItemDO2DTO)}, nil
}

func (h *DatasetApplicationImpl) BatchGetDatasetItemsByVersion(ctx context.Context, req *dataset.BatchGetDatasetItemsByVersionRequest) (resp *dataset.BatchGetDatasetItemsByVersionResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionRead)
	if err != nil {
		return nil, err
	}
	version, err := h.repo.GetVersion(ctx, req.GetWorkspaceID(), req.GetVersionID())
	if err != nil {
		return nil, err
	}
	if version == nil {
		return nil, errno.NotFoundErrorf("version=%d of space=%d is not found", req.GetVersionID(), req.GetWorkspaceID())
	}
	if len(req.ItemIds) <= 0 {
		return &dataset.BatchGetDatasetItemsByVersionResponse{}, nil
	}

	query := repo.NewListItemsParamsFromVersion(version, func(q *repo.ListItemsParams) {
		q.ItemIDs = req.ItemIds
	})
	items, _, err := h.repo.ListItems(ctx, query)
	if err != nil {
		return nil, err
	}
	if err := h.svc.LoadItemData(ctx, items...); err != nil {
		return nil, err
	}
	schema, err := h.repo.GetSchema(ctx, version.SpaceID, version.SchemaID)
	if err != nil {
		return nil, err
	}
	service.SanitizeOutputItem(schema, items)
	return &dataset.BatchGetDatasetItemsByVersionResponse{Items: gslice.Map(items, convertor.ItemDO2DTO)}, nil
}

func (h *DatasetApplicationImpl) ClearDatasetItem(ctx context.Context, req *dataset.ClearDatasetItemRequest) (r *dataset.ClearDatasetItemResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionEdit)
	if err != nil {
		return nil, err
	}
	ds, err := h.svc.GetDataset(ctx, req.GetWorkspaceID(), req.GetDatasetID())
	if err != nil {
		return nil, err
	}
	ds.UpdatedBy = session.UserIDInCtxOrEmpty(ctx)
	err = h.svc.ClearDataset(ctx, ds)
	if err != nil {
		return nil, err
	}
	return &dataset.ClearDatasetItemResponse{}, nil
}

func (h *DatasetApplicationImpl) fixTotal(req *dataset.ListDatasetItemsRequest, r *dataset.ListDatasetItemsResponse) {
	if r == nil {
		return
	}

	seen := int64(len(r.Items)) + int64(req.GetPageNumber()-1)*int64(req.GetPageSize())
	if r.GetTotal() < seen {
		r.Total = gptr.Of(seen)
	}
}

func (h *DatasetApplicationImpl) listItemsByVersion(ctx context.Context, req *dataset.ListDatasetItemsByVersionRequest, version *entity.DatasetVersion) ([]*entity.Item, *pagination.PageResult, error) {
	if version.SnapshotStatus == entity.SnapshotStatusCompleted {
		// list items from snapshot
		items, pr, err := h.listSnapshotsByVersion(ctx, req)
		if err == nil {
			return items, pr, nil
		}
		logs.CtxError(ctx, "list items from snapshot failed, query from item table instead. version_id=%d, err=%v", version.ID, err)
	}

	query := repo.NewListItemsParamsFromVersion(version, func(q *repo.ListItemsParams) {
		q.Paginator = pagination.New(pagination.WithPrePage(req.PageNumber, req.PageSize, req.PageToken))
	})
	items, pr, err := h.repo.ListItems(ctx, query)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "repo.ListItems")
	}
	return items, pr, nil
}

func (h *DatasetApplicationImpl) listSnapshotsByVersion(ctx context.Context, req *dataset.ListDatasetItemsByVersionRequest) ([]*entity.Item, *pagination.PageResult, error) {
	orderBy := &service.OrderBy{}
	if len(req.GetOrderBys()) != 0 {
		orderBy = &service.OrderBy{
			Field: gptr.Of(req.GetOrderBys()[0].GetField()),
			IsAsc: gptr.Of(req.GetOrderBys()[0].GetIsAsc()),
		}
	}
	pg := pagination.New(
		repo.ItemSnapshotOrderBy(gptr.Indirect(orderBy.Field)),
		pagination.WithOrderByAsc(gptr.Indirect(orderBy.IsAsc)),
		pagination.WithPrePage(req.PageNumber, req.PageSize, req.PageToken),
	)
	snapshots, pr, err := h.repo.ListItemSnapshots(ctx, &repo.ListItemSnapshotsParams{
		SpaceID:   req.GetWorkspaceID(),
		VersionID: req.VersionID,
		Paginator: pg,
	})
	if err != nil {
		return nil, nil, err
	}
	return gslice.Map(snapshots, func(ss *entity.ItemSnapshot) *entity.Item { return ss.Snapshot }), pr, nil
}

func (h *DatasetApplicationImpl) prepare(ctx context.Context, req *dataset.BatchCreateDatasetItemsRequest) (*batchCreateDatasetItemsReqContext, error) {
	keys := gslice.FilterMap(req.Items, func(i *idl.DatasetItem) (string, bool) { return i.GetItemKey(), len(i.GetItemKey()) > 0 })
	if dup := gslice.Dup(keys); len(dup) > 0 {
		return nil, errno.BadReqErrorf(`duplicate item keys found: %s`, dup)
	}

	// check dataset
	ds, err := h.svc.GetDataset(ctx, req.GetWorkspaceID(), req.DatasetID)
	if err != nil {
		return nil, err
	}
	if !ds.CanWriteItem() {
		return nil, errno.DatasetNotEditableCodeError("dataset_status=%s", ds.Status)
	}

	// check item size and schema
	items := gslice.Map(req.Items, convertor.ItemDTO2DO)
	service.SanitizeInputItem(ds, items...)
	goodItems, badItems := service.ValidateItems(ds, items)
	if len(badItems) > 0 && !req.GetSkipInvalidItems() {
		return nil, h.errorGroupsToError(badItems)
	}

	// check item count
	total, err := h.repo.GetItemCount(ctx, req.DatasetID)
	if err != nil {
		return nil, err
	}
	remaining := ds.Spec.MaxItemCount - total
	diff := remaining - int64(len(goodItems))
	if diff < 0 && !req.GetAllowPartialAdd() {
		return nil, errno.Errorf(errno.DatasetCapacityFullCode, "capacity=%d, current=%d, to_add=%d", ds.Spec.MaxItemCount, total, len(goodItems))
	}
	if diff < 0 {
		skipped := &entity.ItemErrorGroup{
			Type:    gptr.Of(entity.ItemErrorType_ExceedDatasetCapacity),
			Summary: gptr.Of(fmt.Sprintf("capacity=%d, current=%d, to_add=%d", ds.Spec.MaxItemCount, total, len(goodItems))),
			Details: gslice.Map(goodItems[remaining:], func(i *service.IndexedItem) *entity.ItemErrorDetail {
				return &entity.ItemErrorDetail{Index: gptr.Of(int32(i.Index))}
			}),
		}
		goodItems = goodItems[:remaining]
		badItems = append(badItems, skipped)
	}

	// fill items
	user := session.UserIDInCtxOrEmpty(ctx)
	gslice.ForEach(goodItems, func(item *service.IndexedItem) {
		item.CreatedBy = user
		item.UpdatedBy = user
	})
	rc := &batchCreateDatasetItemsReqContext{
		ds:        ds,
		goodItems: goodItems,
		badItems:  badItems,
		itemCount: total,
	}
	return rc, nil
}

func (h *DatasetApplicationImpl) buildResp(rc *batchCreateDatasetItemsReqContext, added []*service.IndexedItem) *dataset.BatchCreateDatasetItemsResponse {
	badItems := rc.badItems
	if len(rc.goodItems) != len(added) {
		si, ok := gslice.Find(badItems, func(e *entity.ItemErrorGroup) bool {
			return gptr.Indirect(e.Type) == entity.ItemErrorType_ExceedDatasetCapacity
		}).Get()
		if !ok {
			si = &entity.ItemErrorGroup{Type: gptr.Of(entity.ItemErrorType_ExceedDatasetCapacity)}
			badItems = append(badItems, si)
		}
		indices := set.New(gslice.Map(added, func(i *service.IndexedItem) int { return i.Index })...)
		for _, i := range rc.goodItems {
			if !indices.Contains(i.Index) {
				si.Details = append(si.Details, &entity.ItemErrorDetail{Index: gptr.Of(int32(i.Index))})
			}
		}
	}

	gslice.ForEach(badItems, func(e *entity.ItemErrorGroup) { entity.SanitizeItemErrorGroup(e, 5) })
	r := &dataset.BatchCreateDatasetItemsResponse{
		AddedItems: gslice.ToMap(added, func(i *service.IndexedItem) (int64, int64) { return int64(i.Index), i.ItemID }),
		Errors:     gslice.Map(badItems, func(e *entity.ItemErrorGroup) *idl.ItemErrorGroup { return convertor.ItemErrorGroupDO2DTO(e) }),
	}

	return r
}

func (h *DatasetApplicationImpl) errorGroupsToError(egs []*entity.ItemErrorGroup) error {
	gslice.ForEach(egs, func(e *entity.ItemErrorGroup) { entity.SanitizeItemErrorGroup(e, 5) })
	msgs := gslice.Map(egs, func(e *entity.ItemErrorGroup) string {
		details := gslice.Map(e.Details, func(d *entity.ItemErrorDetail) string { return entity.ItemErrorDetailToString(d) })
		return fmt.Sprintf("type=%v, count=%d, details=%v", gptr.Indirect(e.Type), gptr.Indirect(e.ErrorCount), details)
	})
	return errno.BadReqErrorf(`invalid items, errors=%v`, msgs)
}

func (h *DatasetApplicationImpl) buildItem(ctx context.Context, req *dataset.UpdateDatasetItemRequest, ds *service.DatasetWithSchema, item *entity.Item, inPlace bool) {
	patch := &entity.Item{
		Data:         gslice.Map(req.Data, convertor.FieldDataDTO2DO),
		RepeatedData: gslice.Map(req.RepeatedData, convertor.ItemDataDTO2DO),
	}
	service.SanitizeInputItem(ds, patch)
	userID := session.UserIDInCtxOrEmpty(ctx)

	item.UpdatedBy = userID
	item.Data = patch.Data
	item.RepeatedData = patch.RepeatedData
	item.UpdatedAt = time.Now()
	item.BuildProperties()

	if inPlace {
		return
	}
	item.AddVN = ds.NextVersionNum
	item.DelVN = consts.MaxVersionNum
}

func (h *DatasetApplicationImpl) ValidateDatasetItems(ctx context.Context, req *dataset.ValidateDatasetItemsReq) (r *dataset.ValidateDatasetItemsResp, err error) {
	// 鉴权
	if req.GetDatasetID() > 0 {
		err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionEdit)
	} else {
		err = h.auth.Authorization(ctx, &rpc.AuthorizationParam{
			ObjectID:      strconv.FormatInt(req.GetWorkspaceID(), 10),
			SpaceID:       req.GetDatasetID(),
			ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionListLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
		})
	}
	if err != nil {
		return nil, err
	}

	fields, err := gslice.TryMap(req.GetDatasetFields(), convertor.FieldSchemaDTO2DO).Get()
	if err != nil {
		return nil, errno.BadReqErr(err, "convert dataset fields")
	}

	param := &service.ValidateDatasetItemsParam{
		SpaceID:                req.GetWorkspaceID(),
		DatasetID:              req.GetDatasetID(),
		DatasetCategory:        convertor.ConvertCategoryDTO2DO(req.GetDatasetCategory()),
		DatasetFields:          fields,
		Items:                  gslice.Map(req.GetItems(), convertor.ItemDTO2DO),
		IgnoreCurrentItemCount: req.GetIgnoreCurrentItemCount(),
	}

	results, err := h.svc.ValidateDatasetItems(ctx, param)
	if err != nil {
		return nil, err
	}

	return &dataset.ValidateDatasetItemsResp{
		ValidItemIndices: results.ValidItemIndices,
		Errors:           gslice.Map(results.ErrorGroups, func(e *entity.ItemErrorGroup) *idl.ItemErrorGroup { return convertor.ItemErrorGroupDO2DTO(e) }),
	}, nil
}
