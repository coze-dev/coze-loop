// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"strings"

	"github.com/bytedance/gg/gcond"
	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	idl "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	convertor "github.com/coze-dev/coze-loop/backend/modules/data/application/convertor/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/service"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/encoding"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type (
	IJobRunMsgHandler interface {
		RunSnapshotItemJob(ctx context.Context, msg *entity.JobRunMessage) error
		RunIOJob(ctx context.Context, msg *entity.JobRunMessage) error
	}

	IDatasetApplication interface {
		data.DatasetService
		IJobRunMsgHandler
	}
)

func NewDatasetApplicationImpl(auth rpc.IAuthProvider, svc service.IDatasetAPI, repo repo.IDatasetAPI, auditClient audit.IAuditService) IDatasetApplication {
	return &DatasetApplicationImpl{
		auth:        auth,
		svc:         svc,
		repo:        repo,
		auditClient: auditClient,
	}
}

type DatasetApplicationImpl struct {
	auth        rpc.IAuthProvider
	svc         service.IDatasetAPI
	repo        repo.IDatasetAPI
	auditClient audit.IAuditService
}

func (h *DatasetApplicationImpl) RunSnapshotItemJob(ctx context.Context, msg *entity.JobRunMessage) error {
	return h.svc.RunSnapshotItemJob(ctx, msg)
}

func (h *DatasetApplicationImpl) RunIOJob(ctx context.Context, msg *entity.JobRunMessage) error {
	return h.svc.RunIOJob(ctx, msg)
}

type batchCreateDatasetItemsReqContext struct {
	ds        *service.DatasetWithSchema
	goodItems []*service.IndexedItem
	badItems  []*entity.ItemErrorGroup
	itemCount int64
}

func (h *DatasetApplicationImpl) CreateDataset(ctx context.Context, req *dataset.CreateDatasetRequest) (resp *dataset.CreateDatasetResponse, err error) {
	userID := session.UserIDInCtxOrEmpty(ctx)
	appID := session.AppIDInCtxOrEmpty(ctx)
	// 鉴权
	err = h.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionCreateLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	set := &entity.Dataset{
		AppID:         gcond.If(appID == 0, gptr.Indirect(req.AppID), appID),
		SpaceID:       req.GetWorkspaceID(),
		Name:          req.GetName(),
		Description:   gptr.Of(req.GetDescription()),
		Category:      convertor.ConvertCategoryDTO2DO(req.GetCategory()),
		BizCategory:   req.GetBizCategory(),
		SecurityLevel: convertor.SecurityLevelDTO2DO(req.GetSecurityLevel()),
		Visibility:    convertor.VisibilityDTO2DO(req.GetVisibility()),
		Spec:          convertor.SpecDTO2DO(req.GetSpec()),
		Features:      convertor.FeaturesDTO2DO(req.GetFeatures()),
		LastOperation: entity.DatasetOpTypeCreateDataset,
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}
	fields, err := gslice.TryMap(req.Fields, convertor.FieldSchemaDTO2DO).Get()
	if err != nil {
		return nil, err
	}
	for _, f := range fields {
		f.Status = entity.FieldStatusAvailable
		f.Key = ""
	}
	fieldNames := gslice.FilterMap(fields, func(f *entity.FieldSchema) (string, bool) { return f.Name, f.Name != "" })
	fieldDescs := gslice.FilterMap(fields, func(f *entity.FieldSchema) (string, bool) { return f.Description, f.Description != "" })
	data := map[string]string{
		"texts": strings.Join(gslice.Union(fieldNames, fieldDescs, []string{set.Name, set.GetDescription()}), ","),
	}
	record, err := h.auditClient.Audit(ctx, audit.AuditParam{
		ObjectID:  0,
		AuditType: audit.AuditType_CozeLoopDatasetModify,
		AuditData: data,
		ReqID:     encoding.Encode(ctx, data),
	})
	if err != nil {
		logs.CtxError(ctx, "call BatchAuditDatas failed, err=%v", err)
	}
	if record.AuditStatus != audit.AuditStatus_Approved {
		return nil, errno.GetContentAuditFailedErrorf("reason=%s", gptr.Indirect(record.FailedReason))
	}
	if err := h.svc.CreateDataset(ctx, set, fields); err != nil {
		return nil, err
	}
	return &dataset.CreateDatasetResponse{
		DatasetID: gptr.Of(set.ID),
	}, nil
}

func (h *DatasetApplicationImpl) UpdateDataset(ctx context.Context, req *dataset.UpdateDatasetRequest) (resp *dataset.UpdateDatasetResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionEdit)
	if err != nil {
		return nil, err
	}
	data := map[string]string{
		"texts": strings.Join([]string{req.GetName(), req.GetDescription()}, ","),
	}
	record, err := h.auditClient.Audit(ctx, audit.AuditParam{
		ObjectID:  req.GetDatasetID(),
		AuditType: audit.AuditType_CozeLoopDatasetModify,
		AuditData: data,
		ReqID:     encoding.Encode(ctx, data),
	})
	if err != nil {
		logs.CtxError(ctx, "call BatchAuditDatas failed, err=%v", err)
	}
	if record.AuditStatus != audit.AuditStatus_Approved {
		return nil, errno.GetContentAuditFailedErrorf("reason=%s", gptr.Indirect(record.FailedReason))
	}
	err = h.svc.UpdateDataset(ctx, &service.UpdateDatasetParam{
		SpaceID:     req.GetWorkspaceID(),
		DatasetID:   req.GetDatasetID(),
		Name:        req.GetName(),
		Description: gptr.Of(req.GetDescription()),
		UpdatedBy:   session.UserIDInCtxOrEmpty(ctx),
	})
	if err != nil {
		return nil, err
	}
	return &dataset.UpdateDatasetResponse{}, nil
}

func (h *DatasetApplicationImpl) DeleteDataset(ctx context.Context, req *dataset.DeleteDatasetRequest) (resp *dataset.DeleteDatasetResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionEdit)
	if err != nil {
		return nil, err
	}
	err = h.svc.DeleteDataset(ctx, req.GetWorkspaceID(), req.GetDatasetID())
	if err != nil {
		return nil, err
	}
	return &dataset.DeleteDatasetResponse{}, nil
}

func (h *DatasetApplicationImpl) ListDatasets(ctx context.Context, req *dataset.ListDatasetsRequest) (resp *dataset.ListDatasetsResponse, err error) {
	// 鉴权
	err = h.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionListLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	// 批量获取 dataset 和 schema
	orderBy := &service.OrderBy{}
	if len(req.GetOrderBys()) != 0 {
		orderBy = &service.OrderBy{
			Field: gptr.Of(req.GetOrderBys()[0].GetField()),
			IsAsc: gptr.Of(req.GetOrderBys()[0].GetIsAsc()),
		}
	}
	res, err := h.svc.SearchDataset(ctx, &service.SearchDatasetsParam{
		SpaceID:      req.GetWorkspaceID(),
		DatasetIDs:   req.DatasetIds,
		Category:     convertor.ConvertCategoryDTO2DO(gptr.Indirect(req.Category)),
		Name:         req.Name,
		CreatedBys:   req.CreatedBys,
		Page:         req.PageNumber,
		PageSize:     req.PageSize,
		Cursor:       req.PageToken,
		OrderBy:      orderBy,
		BizCategorys: req.BizCategorys,
	})
	if err != nil {
		return nil, err
	}
	res.DatasetWithSchemas = gslice.Map(res.DatasetWithSchemas, func(ds *service.DatasetWithSchema) *service.DatasetWithSchema {
		ds.Schema.Fields = ds.Schema.AvailableFields()
		return ds
	})
	dtos, err := gslice.TryMap(res.DatasetWithSchemas, func(dataset *service.DatasetWithSchema) (*idl.Dataset, error) {
		return convertor.DatasetDO2DTO(dataset.Dataset, dataset.Schema)
	}).Get()
	if err != nil {
		return nil, err
	}
	resp = &dataset.ListDatasetsResponse{
		Datasets:      dtos,
		NextPageToken: gptr.Of(res.NextCursor),
		Total:         gptr.Of(res.Total),
	}
	if len(dtos) <= 0 {
		return resp, nil
	}
	// 获取对应的 item_count
	itemCountsM, err := h.repo.MGetItemCount(ctx, gslice.Map(res.DatasetWithSchemas, func(dataset *service.DatasetWithSchema) int64 { return dataset.ID })...)
	if err != nil {
		return nil, err
	}
	for _, dto := range dtos {
		dto.ItemCount = gptr.Of(itemCountsM[dto.ID])
	}
	return resp, nil
}

func (h *DatasetApplicationImpl) GetDataset(ctx context.Context, req *dataset.GetDatasetRequest) (resp *dataset.GetDatasetResponse, err error) {
	// 鉴权
	err = h.authByDatasetID(ctx, req.GetWorkspaceID(), req.GetDatasetID(), rpc.CommonActionRead)
	if err != nil {
		return nil, err
	}
	dsWithSchema, err := h.svc.GetDatasetWithOpt(ctx, req.GetWorkspaceID(), req.GetDatasetID(), service.WithDeleted(req.GetWithDeleted()))
	if err != nil {
		return nil, err
	}
	if !req.GetWithDeleted() {
		dsWithSchema.Schema.Fields = dsWithSchema.Schema.AvailableFields()
	}
	dto, err := convertor.DatasetDO2DTO(dsWithSchema.Dataset, dsWithSchema.Schema)
	if err != nil {
		return nil, err
	}
	// 获取当前数据条数
	itemCnt, err := h.repo.GetItemCount(ctx, req.GetDatasetID())
	if err != nil {
		return nil, err
	}
	dto.ItemCount = gptr.Of(itemCnt)
	return &dataset.GetDatasetResponse{Dataset: dto}, nil
}

func (h *DatasetApplicationImpl) BatchGetDatasets(ctx context.Context, req *dataset.BatchGetDatasetsRequest) (r *dataset.BatchGetDatasetsResponse, err error) {
	// 鉴权
	err = h.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionListLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	ds, err := h.svc.BatchGetDatasetWithOpt(ctx, req.GetWorkspaceID(), req.GetDatasetIds(), &service.GetOpt{WithDeleted: req.GetWithDeleted()})
	if err != nil {
		return nil, err
	}
	dtos, err := gslice.TryMap(ds, func(ds *service.DatasetWithSchema) (*idl.Dataset, error) {
		ds.Schema.Fields = ds.Schema.AvailableFields()
		return convertor.DatasetDO2DTO(ds.Dataset, ds.Schema)
	}).Get()
	if err != nil {
		return nil, err
	}
	return &dataset.BatchGetDatasetsResponse{Datasets: dtos}, nil
}

func (h *DatasetApplicationImpl) authByDatasetID(ctx context.Context, spaceID, datasetID int64, action string) error {
	// 获取dataset owner信息
	ds, err := h.repo.GetDataset(ctx, spaceID, datasetID, repo.WithDeleted())
	if err != nil {
		return err
	}
	if ds == nil {
		return errno.NotFoundErrorf("dataset %d not found, space_id=%d", datasetID, spaceID)
	}
	// 鉴权
	err = h.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
		ObjectID:        strconv.FormatInt(datasetID, 10),
		SpaceID:         spaceID,
		ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(action), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationSet)}},
		OwnerID:         &ds.CreatedBy,
		ResourceSpaceID: ds.SpaceID,
	})
	if err != nil {
		return err
	}
	return nil
}

func (h *DatasetApplicationImpl) authByVersionID(ctx context.Context, spaceID, versionID int64, action string) error {
	// 获取version信息
	version, err := h.repo.GetVersion(ctx, spaceID, versionID)
	if err != nil {
		return err
	}
	if version == nil {
		return errno.NotFoundErrorf("version %d not found", versionID)
	}
	// 获取dataset owner信息
	return h.authByDatasetID(ctx, spaceID, version.DatasetID, action)
}

func (h *DatasetApplicationImpl) authByJobID(ctx context.Context, spaceID, jobID int64, action string) error {
	// 获取job信息
	job, err := h.repo.GetIOJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return errno.NotFoundErrorf("job %d not found", jobID)
	}
	return h.authByDatasetID(ctx, spaceID, job.DatasetID, action)
}
