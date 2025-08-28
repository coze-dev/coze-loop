// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/bytedance/gg/gvalue"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	tag2 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/userinfo"
	entity2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
	repo2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/repo"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/service"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type TagApplicationImpl struct {
	tagSvc          service.ITagService
	tagRepo         repo2.ITagAPI
	auth            rpc.IAuthProvider
	userInfoService userinfo.UserInfoService
}

func NewTagApplicationImpl(tagSvc service.ITagService, tagRepo repo2.ITagAPI, auth rpc.IAuthProvider, userInfoService userinfo.UserInfoService) tag.TagService {
	return &TagApplicationImpl{
		tagSvc:          tagSvc,
		tagRepo:         tagRepo,
		auth:            auth,
		userInfoService: userInfoService,
	}
}

func (t *TagApplicationImpl) CreateTag(ctx context.Context, req *tag.CreateTagRequest) (r *tag.CreateTagResponse, err error) {
	resp := tag.NewCreateTagResponse()
	// auth check
	err = t.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionCreateLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	tagKey := &entity2.TagKey{
		TagKeyName:     req.GetTagKeyName(),
		Description:    req.Description,
		Status:         entity2.TagStatusActive,
		TagType:        entity2.TagTypeTag,
		TagContentType: entity2.NewTagContentTypeFromDTO(req.GetTagContentType()),
		TagTargetType:  gslice.Map(req.TagDomainTypes, entity2.NewTagTargetTypeFromDTO),
		TagValues: gslice.Map(req.TagValues, func(val *tag2.TagValue) *entity2.TagValue {
			return entity2.NewTagValueFromDTO(val, func(v *entity2.TagValue) {
				v.Status = entity2.TagStatusActive
			})
		}),
		ContentSpec: entity2.NewTagContentSpec(req.GetTagContentSpec()),
		Version:     req.Version,
	}
	if req.TagType != nil {
		tagKey.TagType = entity2.NewTagTypeFromDTO(*req.TagType)
	}
	tagKeyID, err := t.tagSvc.CreateTag(ctx, req.GetWorkspaceID(), tagKey)
	if err != nil {
		return nil, err
	}
	resp.SetTagKeyID(gptr.Of(tagKeyID))
	return resp, nil
}

func (t *TagApplicationImpl) UpdateTag(ctx context.Context, req *tag.UpdateTagRequest) (r *tag.UpdateTagResponse, err error) {
	resp := tag.NewUpdateTagResponse()

	// auth check
	err = t.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionCreateLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	oldTag, err := t.tagSvc.GetLatestTag(ctx, req.GetWorkspaceID(), req.GetTagKeyID(), db.WithMaster())
	if err != nil {
		logs.CtxWarn(ctx, "[UpdateTag] get latest tag failed, err: %v", err)
		return nil, err
	}
	if oldTag == nil {
		logs.CtxError(ctx, "[UpdateTag] tag is not existed, spaceID: %v, tagKeyID: %v", req.GetWorkspaceID(), req.GetTagKeyID())
		return nil, errno.InvalidParamErrorf("tag is not existed")
	}
	switch oldTag.TagType {
	case entity2.TagTypeTag:
		tagKey := &entity2.TagKey{
			TagKeyName:     req.GetTagKeyName(),
			TagKeyID:       req.GetTagKeyID(),
			Description:    req.Description,
			TagType:        entity2.TagTypeTag,
			Status:         oldTag.Status,
			TagContentType: entity2.NewTagContentTypeFromDTO(req.GetTagContentType()),
			TagTargetType:  gslice.Map(req.TagDomainTypes, entity2.NewTagTargetTypeFromDTO),
			TagValues: gslice.Map(req.TagValues, func(val *tag2.TagValue) *entity2.TagValue {
				return entity2.NewTagValueFromDTO(val, func(v *entity2.TagValue) {
					if v == nil {
						return
					}
					if v.Status == entity2.TagStatusUndefined {
						v.Status = entity2.TagStatusActive
					}
				})
			}),
			ContentSpec: entity2.NewTagContentSpec(req.GetTagContentSpec()),
			Version:     req.Version,
		}
		err = t.tagSvc.UpdateTag(ctx, req.GetWorkspaceID(), req.GetTagKeyID(), tagKey)
	case entity2.TagTypeOption:
		tagKey := &entity2.TagKey{
			TagKeyID:       req.GetTagKeyID(),
			TagType:        entity2.TagTypeOption,
			TagContentType: oldTag.TagContentType,
			Status:         oldTag.Status,
			TagValues: gslice.Map(req.TagValues, func(val *tag2.TagValue) *entity2.TagValue {
				return entity2.NewTagValueFromDTO(val, func(v *entity2.TagValue) {
					if v == nil {
						return
					}
					if v.Status == entity2.TagStatusUndefined {
						v.Status = entity2.TagStatusActive
					}
				})
			}),
		}
		err = t.tagSvc.UpdateOptionTag(ctx, req.GetWorkspaceID(), req.GetTagKeyID(), tagKey)
	default:
		err = errno.InvalidParamErrorf("tag type is undefained")
	}
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (t *TagApplicationImpl) BatchUpdateTagStatus(ctx context.Context, req *tag.BatchUpdateTagStatusRequest) (r *tag.BatchUpdateTagStatusResponse, err error) {
	resp := tag.NewBatchUpdateTagStatusResponse()

	// auth check
	err = t.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionCreateLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	toStatus := entity2.NewTagStatusFromDTO(gptr.Of(req.ToStatus))
	errInfo, err := t.tagSvc.BatchUpdateTagStatus(ctx, req.GetWorkspaceID(), req.GetTagKeyIds(), toStatus)
	if err != nil {
		return nil, err
	}
	resp.SetErrInfo(errInfo)
	return resp, nil
}

func (t *TagApplicationImpl) SearchTags(ctx context.Context, req *tag.SearchTagsRequest) (r *tag.SearchTagsResponse, err error) {
	resp := tag.NewSearchTagsResponse()

	if gvalue.IsNotZero(req.GetTagKeyName()) && gvalue.IsNotZero(req.GetTagKeyNameLike()) {
		return nil, errno.InvalidParamErrorf("tag_key_name and tag_key_name_like can not be set at the same time")
	}

	// auth check
	err = t.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionListLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	status := []entity2.TagStatus{entity2.TagStatusActive, entity2.TagStatusInactive}
	param := &entity2.MGetTagKeyParam{
		Paginator: pagination.New(
			pagination.WithCursor(req.GetPageToken()),
			pagination.WithLimit(int(req.GetPageSize())),
			pagination.WithPage(req.GetPageNumber(), req.GetPageSize()),
			repo2.TagKeyOrderBy(req.OrderBy.GetField()),
			pagination.WithOrderByAsc(req.OrderBy.GetIsAsc()),
		),
		SpaceID:         req.GetWorkspaceID(),
		TagType:         gptr.Of(entity2.TagTypeTag),
		Status:          status,
		CreatedBys:      req.GetCreatedBys(),
		TagKeyNameLike:  req.GetTagKeyNameLike(),
		TagKeyName:      req.TagKeyName,
		TagDomainTypes:  gslice.Map(req.GetDomainTypes(), entity2.NewTagTargetTypeFromDTO),
		TagContentTypes: gslice.Map(req.GetContentTypes(), entity2.NewTagContentTypeFromDTO),
	}

	tagKeys, pr, err := t.tagSvc.SearchTags(ctx, req.GetWorkspaceID(), param)
	if err != nil {
		logs.CtxWarn(ctx, "[SearchTagsHandler] get tag keys failed, param: %v, err: %+v", json.MarshalStringIgnoreErr(param), err)
		return nil, err
	}
	dtos := gslice.Map(tagKeys, (*entity2.TagKey).ToTagInfoDTO)
	t.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(dtos))
	resp.SetTagInfos(dtos)
	resp.SetTotal(gptr.Of(pr.Total))
	resp.SetNextPageToken(gptr.Of(pr.Cursor))
	return resp, nil
}

func (t *TagApplicationImpl) GetTagDetail(ctx context.Context, req *tag.GetTagDetailRequest) (r *tag.GetTagDetailResponse, err error) {
	resp := tag.NewGetTagDetailResponse()

	// auth check
	err = t.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionListLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	detail, err := t.tagSvc.GetTagDetail(ctx, req.GetWorkspaceID(), &entity2.GetTagDetailReq{
		PageSize:  req.GetPageSize(),
		PageNum:   req.GetPageNumber(),
		PageToken: req.GetPageToken(),
		TagKeyID:  req.GetTagKeyID(),
		OrderBy:   req.GetOrderBy().GetField(),
		IsAsc:     req.GetOrderBy().GetIsAsc(),
	})
	if err != nil {
		return nil, err
	}
	dtos := gslice.Map(detail.TagKeys, (*entity2.TagKey).ToTagInfoDTO)
	t.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(dtos))
	resp.SetTags(dtos)
	resp.SetTotal(gptr.Of(detail.Total))
	resp.SetNextPageToken(gptr.Of(detail.NextPageToken))
	return resp, nil
}

func (t *TagApplicationImpl) GetTagSpec(ctx context.Context, req *tag.GetTagSpecRequest) (r *tag.GetTagSpecResponse, err error) {
	resp := tag.NewGetTagSpecResponse()
	// auth check
	err = t.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionListLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	height, width, total, err := t.tagSvc.GetTagSpec(ctx, req.GetWorkspaceID())
	if err != nil {
		return nil, err
	}
	resp.SetMaxHeight(gptr.Of(height))
	resp.SetMaxWidth(gptr.Of(width))
	resp.SetMaxTotal(gptr.Of(total))
	return resp, nil
}

func (t *TagApplicationImpl) BatchGetTags(ctx context.Context, req *tag.BatchGetTagsRequest) (r *tag.BatchGetTagsResponse, err error) {
	resp := tag.NewBatchGetTagsResponse()
	// auth check
	err = t.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionListLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}
	tagKeys, err := t.tagSvc.BatchGetTagsByTagKeyIDs(ctx, req.GetWorkspaceID(), req.GetTagKeyIds())
	if err != nil {
		return nil, err
	}
	dtos := gslice.Map(tagKeys, (*entity2.TagKey).ToTagInfoDTO)
	t.userInfoService.PackUserInfo(ctx, userinfo.BatchConvertDTO2UserInfoCarrier(dtos))
	resp.SetTagInfoList(dtos)
	return resp, nil
}

func (t *TagApplicationImpl) ArchiveOptionTag(ctx context.Context, req *tag.ArchiveOptionTagRequest) (r *tag.ArchiveOptionTagResponse, err error) {
	resp := tag.NewArchiveOptionTagResponse()
	// auth check
	err = t.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(req.WorkspaceID, 10),
		SpaceID:       req.WorkspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(rpc.CozeActionCreateLoopEvaluationSet), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
	if err != nil {
		return nil, err
	}

	// get latest tag key
	tagKey, err := t.tagSvc.GetLatestTag(ctx, req.GetWorkspaceID(), req.GetTagKeyID(), db.WithMaster())
	if err != nil {
		return nil, err
	}
	if tagKey.TagType != entity2.TagTypeOption {
		logs.CtxError(ctx, "[ArchiveOptionTag] tag key is not option tag, spaceID: %d, tagKeyID %d",
			req.GetWorkspaceID(), req.GetTagKeyID())
		return nil, errno.InvalidParamErrorf("tag key is not option tag")
	}
	tagKey.TagType = entity2.TagTypeTag
	tagKey.SetUpdatedBy(session.UserIDInCtxOrEmpty(ctx))
	tagKey.SetUpdatedAt(time.Now())
	tagKey.Description = req.Description
	tagKey.TagKeyName = req.GetName()
	tagKey.Version = gptr.Of("0.0.1")
	err = t.tagSvc.ArchiveOptionTag(ctx, req.GetWorkspaceID(), req.GetTagKeyID(), tagKey)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
