// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gvalue"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	entity2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/repo"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type TagServiceImpl struct {
	tagRepo repo.ITagAPI
	db      db.Provider
	locker  lock.ILocker
	config  conf.IConfig

	tagSpecConfig func() *conf.TagSpec
}

func NewTagServiceImpl(tagRepo repo.ITagAPI, db db.Provider, locker lock.ILocker, config conf.IConfig) ITagService {
	return &TagServiceImpl{
		tagRepo:       tagRepo,
		db:            db,
		locker:        locker,
		config:        config,
		tagSpecConfig: config.GetTagSpec,
	}
}

func (s *TagServiceImpl) CreateTag(ctx context.Context, spaceID int64, val *entity2.TagKey, opts ...db.Option) (int64, error) {
	// validate tag
	if err := val.Validate(s.tagSpecConfig().GetSpecBySpace(spaceID)); err != nil {
		return 0, err
	}

	// 计算更新日志
	changeLogs, err := val.CalculateChangeLogs(nil)
	if err != nil {
		logs.CtxError(ctx, "[CreateTag] get change logs failed, tag key: %v, err: %v", json.MarshalStringIgnoreErr(val), err)
		return 0, err
	}
	val.ChangeLogs = changeLogs

	userID := session.UserIDInCtxOrEmpty(ctx)
	appID := session.AppIDInCtxOrEmpty(ctx)
	ts := time.Now()
	val.SetCreatedAt(ts)
	val.SetUpdatedAt(ts)
	val.SetAppID(appID)
	val.SetCreatedBy(userID)
	val.SetUpdatedBy(userID)
	val.SetVersionNum(0)
	val.SetSpaceID(spaceID)

	// 加锁
	// 只有tag类型才加锁 & 检测是否有同名
	if val.TagType == entity2.TagTypeTag {
		locked, err := s.locker.Lock(ctx, FormatCreateTagKey(val.SpaceID, val.TagKeyName), 10*time.Second)
		if err != nil {
			logs.CtxError(ctx, "[CreateTag] lock failed, err: %v", err)
			return 0, err
		}
		if !locked {
			logs.CtxWarn(ctx, "[CreateTag] other create operation is processing, spaceID: %d, tagKayName: %v", val.SpaceID, val.TagKeyName)
			return 0, errno.InvalidParamErrorf("other create operation is processing")
		}
		defer func() {
			_, _ = s.locker.Unlock(FormatCreateTagKey(val.SpaceID, val.TagKeyName))
		}()

		// check tag key name
		exist, err := s.isTagNameExisted(ctx, spaceID, 0, val.TagKeyName)
		if err != nil {
			logs.CtxError(ctx, "[CreateTag] check tag name existed failed, err: %v", err)
			return 0, err
		}
		if exist {
			logs.CtxError(ctx, "[CreateTag] tag name is already existed")
			return 0, errno.InvalidParamErrorf("tag name is already existed")
		}
		// check version
		if val.Version != nil {
			if err = ValidateVersion("", *val.Version); err != nil {
				logs.CtxError(ctx, "[CreateTag] validate version failed, err: %v", err)
				return 0, err
			}
		}
	}
	// insert tag key and tag value
	var tagKeyID int64
	err = s.db.Transaction(ctx, func(tx *gorm.DB) error {
		innerOpt := db.WithTransaction(tx)
		err := s.tagRepo.MCreateTagKeys(ctx, []*entity2.TagKey{val}, innerOpt)
		if err != nil {
			logs.CtxError(ctx, "[CreateTag] create tag keys failed, err: %v", err)
			return err
		}
		tagKeyID = val.TagKeyID
		now := val.TagValues
		for len(now) != 0 {
			// assign tag key id
			for _, v := range now {
				value := v
				value.TagKeyID = tagKeyID
			}
			// insert level tag values
			err := s.tagRepo.MCreateTagValues(ctx, now, innerOpt)
			if err != nil {
				logs.CtxError(ctx, "[CreateTag] create tag values failed, err: %v", err)
				return err
			}

			// get next level & assigned parent tag value id
			var next []*entity2.TagValue
			for _, v := range now {
				value := v
				for _, vv := range value.Children {
					value2 := vv
					value2.ParentValueID = value.TagValueID
				}
				next = append(next, value.Children...)
			}
			now = next
		}
		return nil
	}, opts...)
	if err != nil {
		logs.CtxError(ctx, "[CreateTag] insert tag key and tag value failed, err: %v", err)
		return 0, err
	}
	return tagKeyID, nil
}

func (s *TagServiceImpl) isTagNameExisted(ctx context.Context, spaceID, tagKeyID int64, tagName string) (bool, error) {
	// check tag key name
	tagKeys, _, err := s.tagRepo.MGetTagKeys(ctx, &entity2.MGetTagKeyParam{
		Paginator:  pagination.New(pagination.WithLimit(2)),
		SpaceID:    spaceID,
		TagKeyName: gptr.Of(tagName),
		Status:     []entity2.TagStatus{entity2.TagStatusInactive, entity2.TagStatusActive},
	}, db.WithMaster())
	if err != nil {
		logs.CtxError(ctx, "[isTagNameExisted] MGetTagKeys failed, err: %v", err)
		return false, err
	}
	if gvalue.IsZero(tagKeyID) {
		return len(tagKeys) > 0, nil
	}
	if len(tagKeys) > 1 {
		return true, nil
	} else if len(tagKeys) == 1 {
		return tagKeys[0].TagKeyID != tagKeyID, nil
	}
	return false, nil
}

func (s *TagServiceImpl) GetAllTagKeyVersionsByKeyID(ctx context.Context, spaceID, tagKeyID int64, opts ...db.Option) ([]*entity2.TagKey, error) {
	var (
		res    []*entity2.TagKey
		cursor string
	)
	for {
		tagKeys, pr, err := s.tagRepo.MGetTagKeys(ctx, &entity2.MGetTagKeyParam{
			Paginator: pagination.New(pagination.WithLimit(100), pagination.WithCursor(cursor)),
			SpaceID:   spaceID,
			TagKeyIDs: []int64{tagKeyID},
		}, opts...)
		if err != nil {
			logs.CtxError(ctx, "[GetAllTagKeyVersionsByKeyID] get tag keys failed, err: %v", err)
			return nil, err
		}
		res = append(res, tagKeys...)
		if gvalue.IsZero(pr.Cursor) {
			break
		}
		cursor = pr.Cursor
	}
	return res, nil
}

func (s *TagServiceImpl) GetAndBuildTagValues(ctx context.Context, spaceID, tagKeyID int64, versionNum int32, opts ...db.Option) ([]*entity2.TagValue, error) {
	var (
		cursor    string
		tagValues []*entity2.TagValue
	)
	// get tag values
	for {
		val, pr, err := s.tagRepo.MGetTagValue(ctx, &entity2.MGetTagValueParam{
			Paginator: pagination.New(pagination.WithLimit(100), pagination.WithCursor(cursor), pagination.WithOrderByAsc(true)),
			SpaceID:   spaceID,
			TagKeyID:  gptr.Of(tagKeyID),
			Version:   gptr.Of(versionNum),
		}, opts...)
		if err != nil {
			logs.CtxError(ctx, "[GetAndBuildTagValues] get tag values failed, spaceID: %d, tagKeyID: %d, versionNum: %d, err: %v", spaceID, tagKeyID, versionNum, err)
			return nil, err
		}
		tagValues = append(tagValues, val...)
		if gvalue.IsZero(pr.Cursor) {
			break
		}
		cursor = pr.Cursor
	}
	// build tree
	var res []*entity2.TagValue
	valueMap := make(map[int64]*entity2.TagValue, len(tagValues))
	for _, v := range tagValues {
		val := v
		valueMap[val.TagValueID] = val
	}
	for _, v := range tagValues {
		item := v
		if gvalue.IsZero(item.ParentValueID) {
			res = append(res, item)
		} else {
			if _, ok := valueMap[item.ParentValueID]; ok {
				valueMap[item.ParentValueID].Children = append(valueMap[item.ParentValueID].Children, item)
			}
		}
	}
	return res, nil
}

func (s *TagServiceImpl) GetLatestTag(ctx context.Context, spaceID, tagKeyID int64, opts ...db.Option) (*entity2.TagKey, error) {
	// get tag key
	tagKeys, _, err := s.tagRepo.MGetTagKeys(ctx, &entity2.MGetTagKeyParam{
		Paginator: pagination.New(pagination.WithLimit(1)),
		SpaceID:   spaceID,
		TagKeyIDs: []int64{tagKeyID},
		Status:    []entity2.TagStatus{entity2.TagStatusActive, entity2.TagStatusInactive},
	}, opts...)
	if err != nil {
		logs.CtxError(ctx, "[GetLatestTag] get tag key failed, spaceID: %d, tagKeyID: %d, err: %v", spaceID, tagKeyID, err)
		return nil, err
	}
	if len(tagKeys) == 0 {
		logs.CtxWarn(ctx, "[GetLatestTag] tag key is not exist, spaceID: %d, tagKeyID: %d", spaceID, tagKeyID)
		return nil, errno.InvalidParamErrorf("tag key is not exist")
	}

	// get tag values
	res := tagKeys[0]
	values, err := s.GetAndBuildTagValues(ctx, spaceID, tagKeyID, *res.VersionNum, opts...)
	if err != nil {
		logs.CtxError(ctx, "[GetLatestTag] get tag values failed, err: %v", err)
		return nil, err
	}
	res.TagValues = values
	return res, nil
}

func (s *TagServiceImpl) UpdateTag(ctx context.Context, spaceID, tagKeyID int64, val *entity2.TagKey, opts ...db.Option) error {
	// validate tag
	if err := val.Validate(s.config.GetTagSpec().GetSpecBySpace(spaceID)); err != nil {
		return err
	}

	val.SetUpdatedAt(time.Now())
	val.SetUpdatedBy(session.UserIDInCtxOrEmpty(ctx))
	val.SetAppID(session.AppIDInCtxOrEmpty(ctx))
	val.SetSpaceID(spaceID)

	nameExisted, err := s.isTagNameExisted(ctx, spaceID, tagKeyID, val.TagKeyName)
	if err != nil {
		logs.CtxError(ctx, "[UpdateTag] check tag name existed failed, err: %v", err)
		return err
	}
	if nameExisted {
		logs.CtxError(ctx, "[UpdateTag] tag name is already existed")
		return errno.InvalidParamErrorf("tag name is already existed")
	}

	// 加锁
	locked, err := s.locker.Lock(ctx, FormatUpdateTagKey(spaceID, tagKeyID), 10*time.Second)
	if err != nil {
		logs.CtxError(ctx, "[UpdateTag] lock failed, spaceID: %d, tagKeyID: %d, err: %v", spaceID, tagKeyID, err)
		return err
	}
	if !locked {
		logs.CtxWarn(ctx, "[UpdateTag] other updating operation is processing, spaceID: %d, tagKeyID: %d", spaceID, tagKeyID)
		return errno.InvalidParamErrorf("other updating operation is processing")
	}
	defer func() {
		_, _ = s.locker.Unlock(FormatUpdateTagKey(spaceID, tagKeyID))
	}()
	// get lastest tag
	preTagKey, err := s.GetLatestTag(ctx, spaceID, tagKeyID, append(opts, db.WithMaster())...)
	if err != nil {
		logs.CtxError(ctx, "[UpdateTag] get latest tag failed, spaceID: %d, tagKeyID: %d, err: %v", spaceID, tagKeyID, err)
		return err
	}
	// 检查version
	if val.Version != nil && preTagKey.Version != nil {
		if err = ValidateVersion(*preTagKey.Version, *val.Version); err != nil {
			logs.CtxError(ctx, "[UpdateTag] validate version failed, err: %v", err)
			return err
		}
	}
	val.SetCreatedBy(*preTagKey.CreatedBy)
	val.SetCreatedAt(preTagKey.CreatedAt)
	// 计算更新日志
	changeLogs, err := val.CalculateChangeLogs(preTagKey)
	if err != nil {
		logs.CtxError(ctx, "[UpdateTag] calculate change logs failed, err: %v", err)
		return err
	}
	val.ChangeLogs = changeLogs
	// 更新tag key和 tag value的版本信息
	val.SetVersionNum(*preTagKey.VersionNum + 1)
	// 落库
	return s.db.Transaction(ctx, func(tx *gorm.DB) error {
		// 分片库不支持嵌套事务，所以这里显示关闭嵌套事务
		// disable nested transaction
		tx.DisableNestedTransaction = true
		innerOpt := db.WithTransaction(tx)

		// disable old tag
		err := s.UpdateTagStatus(ctx, spaceID, tagKeyID, *preTagKey.VersionNum, entity2.TagStatusDeprecated, false, false, innerOpt)
		if err != nil {
			logs.CtxError(ctx, "[UpdateTag] disable old tag failed, spaceID: %d, tagKeyID:%d, versionNum: %d, err: %v", spaceID, tagKeyID, preTagKey.VersionNum, err)
			return err
		}
		// insert tag keys
		err = s.tagRepo.MCreateTagKeys(ctx, []*entity2.TagKey{val}, innerOpt)
		if err != nil {
			logs.CtxError(ctx, "[UpdateTag] insert tag key failed, err: %v", err)
			return err
		}
		// insert tag values
		now := val.TagValues
		for len(now) != 0 {
			var next []*entity2.TagValue
			for _, v := range now {
				value := v
				value.TagKeyID = val.TagKeyID
			}
			err := s.tagRepo.MCreateTagValues(ctx, now, innerOpt)
			if err != nil {
				logs.CtxError(ctx, "[UpdateTag] insert tag values failed, err: %v", err)
				return err
			}
			for _, v := range now {
				value := v
				for _, vv := range value.Children {
					value2 := vv
					value2.ParentValueID = value.TagValueID
				}
				next = append(next, value.Children...)
			}
			now = next
		}
		return nil
	}, opts...)
}

func (s *TagServiceImpl) UpdateTagStatus(ctx context.Context, spaceID, tagKeyID int64, versionNum int32, status entity2.TagStatus, needLock, updatedInfo bool, opts ...db.Option) error {
	// 加锁
	if needLock {
		locked, err := s.locker.Lock(ctx, FormatUpdateTagKey(spaceID, tagKeyID), 10*time.Second)
		if err != nil {
			logs.CtxError(ctx, "[UpdateTagStatus] lock failed, spaceID: %d, tagKeyID: %d, err: %v", spaceID, tagKeyID, err)
			return err
		}
		if !locked {
			logs.CtxWarn(ctx, "[UpdateTagStatus] other updating operation is processing, spaceID: %d, tagKeyID: %d", spaceID, tagKeyID)
			return errno.InvalidParamErrorf("other updating operation is processing")
		}
		defer func() {
			_, _ = s.locker.Unlock(FormatUpdateTagKey(spaceID, tagKeyID))
		}()
	}

	// 更新
	return s.db.Transaction(ctx, func(tx *gorm.DB) error {
		innerOpt := db.WithTransaction(tx)
		// disable tag key
		err := s.tagRepo.UpdateTagKeysStatus(ctx, spaceID, tagKeyID, versionNum, status, updatedInfo, innerOpt)
		if err != nil {
			logs.CtxError(ctx, "[UpdateTagStatus] update tag status failed, spaceID: %d, tagKeyID: %d, versionNum: %d, err: %v", spaceID, tagKeyID, versionNum, err)
			return err
		}
		// disable tag values
		return s.tagRepo.UpdateTagValuesStatus(ctx, spaceID, tagKeyID, versionNum, status, updatedInfo, innerOpt)
	}, opts...)
}

func (s *TagServiceImpl) UpdateTagStatusWithNewVersion(ctx context.Context, spaceID, tagKeyID int64, status entity2.TagStatus) error {
	// 加锁
	locked, err := s.locker.Lock(ctx, FormatUpdateTagKey(spaceID, tagKeyID), 10*time.Second)
	if err != nil {
		logs.CtxError(ctx, "[UpdateTagStatusWithNewVersion] lock failed, spaceID: %d, tagKeyID: %d, err: %v", spaceID, tagKeyID, err)
		return err
	}
	if !locked {
		logs.CtxWarn(ctx, "[UpdateTagStatusWithNewVersion] other updating operation is processing, spaceID: %d, tagKeyID: %d", spaceID, tagKeyID)
		return errno.InvalidParamErrorf("other udpating operation is processing")
	}
	defer func() {
		_, _ = s.locker.Unlock(FormatUpdateTagKey(spaceID, tagKeyID))
	}()
	// get latest tag
	preTagKey, err := s.GetLatestTag(ctx, spaceID, tagKeyID, db.WithMaster())
	if err != nil {
		logs.CtxWarn(ctx, "[UpdateTagStatusWithNewVersion] get latest tag failed, spaceID: %d, tagKyeID: %d, err: %v", spaceID, tagKeyID, err)
		return err
	}
	if preTagKey.Status == status {
		logs.CtxError(ctx, "[UpdateTagStatusWithNewVersion] no need to update status")
		return errno.InvalidParamErrorf("no need to update status")
	}
	nowTagKey := &entity2.TagKey{}
	if err = copier.Copy(nowTagKey, preTagKey); err != nil {
		logs.CtxError(ctx, "[UpdateTagStatusWithNewVersion] copy tag failed, err: %v", err)
		return err
	}
	nowTagKey.SetUpdatedAt(time.Now())
	nowTagKey.SetUpdatedBy(session.UserIDInCtxOrEmpty(ctx))
	nowTagKey.SetAppID(session.AppIDInCtxOrEmpty(ctx))
	nowTagKey.SetSpaceID(spaceID)
	nowTagKey.Status = status
	if nowTagKey.Version != nil {
		nextVersion, err := SimpleIncrementVersion(*nowTagKey.Version)
		if err != nil {
			logs.CtxError(ctx, "[UpdateTagStatusWithNewVersion] increase version failed, err: %v", err)
			return err
		}
		nowTagKey.Version = gptr.Of(nextVersion)
	}
	changeLogs, err := nowTagKey.CalculateChangeLogs(preTagKey)
	if err != nil {
		logs.CtxError(ctx, "[UpdateTagStatusWithNewVersion] calculate change logs failed, err: %v", err)
		return err
	}
	nowTagKey.ChangeLogs = changeLogs
	nowTagKey.SetVersionNum(*preTagKey.VersionNum + 1)
	return s.db.Transaction(ctx, func(tx *gorm.DB) error {
		tx.DisableNestedTransaction = true
		innerOpt := db.WithTransaction(tx)
		// disable old tag
		err := s.UpdateTagStatus(ctx, spaceID, tagKeyID, *preTagKey.VersionNum, entity2.TagStatusDeprecated, false, false, innerOpt)
		if err != nil {
			logs.CtxError(ctx, "[UpdateTag] disable old tag failed, spaceID: %d, tagKeyID:%d, versionNum: %d, err: %v", spaceID, tagKeyID, preTagKey.VersionNum, err)
			return err
		}
		// insert tag keys
		err = s.tagRepo.MCreateTagKeys(ctx, []*entity2.TagKey{nowTagKey}, innerOpt)
		if err != nil {
			logs.CtxError(ctx, "[UpdateTag] insert tag key failed, err: %v", err)
			return err
		}
		// insert tag values
		now := nowTagKey.TagValues
		for len(now) != 0 {
			var next []*entity2.TagValue
			for _, v := range now {
				value := v
				value.TagKeyID = nowTagKey.TagKeyID
			}
			err := s.tagRepo.MCreateTagValues(ctx, now, innerOpt)
			if err != nil {
				logs.CtxError(ctx, "[UpdateTag] insert tag values failed, err: %v", err)
				return err
			}
			for _, v := range now {
				value := v
				for _, vv := range value.Children {
					value2 := vv
					value2.ParentValueID = value.TagValueID
				}
				next = append(next, value.Children...)
			}
			now = next
		}
		return nil
	})
}

func (s *TagServiceImpl) GetTagSpec(ctx context.Context, spaceID int64) (maxHeight, maxWidth, macTotal int64, err error) {
	tagSpec := s.config.GetTagSpec().GetSpecBySpace(spaceID)
	if tagSpec == nil {
		return 1, 20, 20, nil
	}
	return int64(tagSpec.MaxHeight), int64(tagSpec.MaxWidth), int64(tagSpec.MaxWidth * tagSpec.MaxHeight), nil
}

func (s *TagServiceImpl) BatchUpdateTagStatus(ctx context.Context, spaceID int64, tagKeyIDs []int64, toStatus entity2.TagStatus) (map[int64]string, error) {
	if toStatus == entity2.TagStatusUndefined || toStatus == entity2.TagStatusDeprecated {
		logs.CtxError(ctx, "[BatchUpdateTagStatus] toStatus is illegal: %s", toStatus)
		return nil, errno.InvalidParamErrorf("to_status is illegal")
	}
	if len(tagKeyIDs) == 0 {
		logs.CtxError(ctx, "[BatchUpdateTagStatus] tag key ids is empty")
		return nil, errno.InvalidParamErrorf("tag key ids is empty")
	}

	errInfo := make(map[int64]string, 0)
	queryTagKeyMap := make(map[int64]bool, 0)
	for _, tagKeyID := range tagKeyIDs {
		queryTagKeyMap[tagKeyID] = true
		if err := s.UpdateTagStatusWithNewVersion(ctx, spaceID, tagKeyID, toStatus); err != nil {
			logs.CtxWarn(ctx, "[BatchUpdateTagStatus] update tag status failed, spaceID: %d, tagKeyID: %d, err: %v", spaceID, tagKeyID, err)
			errInfo[tagKeyID] = errno.GetInternalErrorMsg(err)
		}
	}
	return errInfo, nil
}

func (s *TagServiceImpl) SearchTags(ctx context.Context, spaceID int64, param *entity2.MGetTagKeyParam) ([]*entity2.TagKey, *pagination.PageResult, error) {
	if param == nil {
		logs.CtxError(ctx, "[SearchTags] param is nil")
		return nil, nil, errno.InvalidParamErrorf("param is nil")
	}
	tagKeys, pr, err := s.tagRepo.MGetTagKeys(ctx, param, db.WithMaster())
	if err != nil {
		logs.CtxWarn(ctx, "[SearchTags] get tag keys failed, param: %v, err: %+v", json.MarshalStringIgnoreErr(param), err)
		return nil, nil, err
	}
	for _, v := range tagKeys {
		item := v
		tagValues, err := s.GetAndBuildTagValues(ctx, spaceID, item.TagKeyID, *item.VersionNum, db.WithMaster())
		if err != nil {
			logs.CtxError(ctx, "[SearchTags] get tag values failed, spaceID: %d, tagKeyID: %d, versionNum: %d, err: %+v", spaceID, item.TagKeyID, item.VersionNum, err)
			return nil, nil, err
		}
		item.TagValues = tagValues
	}
	total, err := s.tagRepo.CountTagKeys(ctx, param, db.WithMaster())
	if err != nil {
		logs.CtxWarn(ctx, "[SearchTags] count tag keys failed, err :%+v", err)
		return nil, nil, err
	}
	pr.Total = total
	return tagKeys, pr, nil
}

func (s *TagServiceImpl) GetTagDetail(ctx context.Context, spaceID int64, param *entity2.GetTagDetailReq) (*entity2.GetTagDetailResp, error) {
	if param == nil {
		logs.CtxError(ctx, "[GetTagDetail] param is nil")
		return nil, errno.InvalidParamErrorf("param is nil")
	}
	resp := &entity2.GetTagDetailResp{}
	var (
		tagKeys []*entity2.TagKey
		pr      *pagination.PageResult
		total   int64
		err     error
	)

	// 兼容逻辑
	if gvalue.IsZero(param.PageSize) {
		tagKeys, err = s.GetAllTagKeyVersionsByKeyID(ctx, spaceID, param.TagKeyID, db.WithMaster())
		if err != nil {
			logs.CtxError(ctx, "[GetTagDetail] get all tag keys by key id failed, spaceID: %d, tagKeyID: %d, err: %+v",
				spaceID, param.TagKeyID, err)
			return nil, err
		}
		resp.Total = int64(len(tagKeys))
	} else {
		queryParam := &entity2.MGetTagKeyParam{
			Paginator: pagination.New(
				pagination.WithLimit(int(param.PageSize)),
				pagination.WithCursor(param.PageToken),
				pagination.WithPage(param.PageNum, param.PageSize),
				repo.TagKeyOrderBy(param.OrderBy),
				pagination.WithOrderByAsc(param.IsAsc),
			),
			SpaceID:   spaceID,
			TagKeyIDs: []int64{param.TagKeyID},
		}
		tagKeys, pr, err = s.tagRepo.MGetTagKeys(ctx, queryParam, db.WithMaster())
		if err != nil {
			logs.CtxWarn(ctx, "[GetTagDetail] MGetTagKeys, spaceID: %d, tagKeyID: %d, err: %v",
				spaceID, param.TagKeyID, err)
			return nil, err
		}
		total, err = s.tagRepo.CountTagKeys(ctx, queryParam, db.WithMaster())
		if err != nil {
			logs.CtxWarn(ctx, "[GetTagDetail] count tag keys failed, spaceID: %d, tagKeyID: %d, err: %v",
				spaceID, param.TagKeyID, err)
			return nil, err
		}
		resp.Total = total
		resp.NextPageToken = pr.Cursor
	}
	// 填充tag values
	for _, v := range tagKeys {
		item := v
		tagValues, err := s.GetAndBuildTagValues(ctx, spaceID, item.TagKeyID, *item.VersionNum, db.WithMaster())
		if err != nil {
			logs.CtxWarn(ctx, "[GetTagDetail] get & build tag values failed, spaceID: %d, tagKeyID: %d, versionNum: %d, err: %v",
				item.SpaceID, item.TagKeyID, item.VersionNum, err)
			return nil, err
		}
		item.TagValues = tagValues
	}
	resp.TagKeys = tagKeys
	return resp, nil
}

func (s *TagServiceImpl) BatchGetTagsByTagKeyIDs(ctx context.Context, spaceID int64, tagKeyIDs []int64) ([]*entity2.TagKey, error) {
	if len(tagKeyIDs) == 0 {
		logs.CtxError(ctx, "[BatchGetTagsByTagKeyIDs] tag key list is empty")
		return nil, errno.InvalidParamErrorf("tag key list is empty")
	}
	params := &entity2.MGetTagKeyParam{
		Paginator: pagination.New(pagination.WithLimit(len(tagKeyIDs))),
		SpaceID:   spaceID,
		Status:    []entity2.TagStatus{entity2.TagStatusActive, entity2.TagStatusInactive},
		TagKeyIDs: tagKeyIDs,
	}
	tagKeys, _, err := s.tagRepo.MGetTagKeys(ctx, params, db.WithMaster())
	if err != nil {
		logs.CtxWarn(ctx, "[BatchGetTagsByTagKeyIDs] get tag keys failed, err: %v", err)
		return nil, err
	}
	for _, v := range tagKeys {
		item := v
		tagValues, err := s.GetAndBuildTagValues(ctx, spaceID, item.TagKeyID, *item.VersionNum, db.WithMaster())
		if err != nil {
			logs.CtxWarn(ctx, "[BatchGetTagsByTagKeyIDs] get & build tag values failed, spaceID: %d, tagKeyID: %d, versionNum: %d, err: %v",
				item.SpaceID, item.TagKeyID, item.VersionNum, err)
			return nil, err
		}
		item.TagValues = tagValues
	}
	return tagKeys, nil
}
