// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tag

import (
	"context"
	"time"

	"github.com/bytedance/gg/gslice"
	"github.com/bytedance/gg/gvalue"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	entity2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/tag/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/tag/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func (t *TagRepoImpl) MCreateTagKeys(ctx context.Context, val []*entity2.TagKey, opt ...db.Option) error {
	if len(val) == 0 {
		return nil
	}
	// gen id
	ids, err := t.idGen.GenMultiIDs(ctx, len(val)*2)
	if err != nil {
		return err
	}
	for i, v := range val {
		tagKey := v
		tagKey.ID = ids[i*2]
		if gvalue.IsZero(tagKey.TagKeyID) {
			tagKey.TagKeyID = ids[i*2+1]
		}
	}
	// gen po
	pps, err := gslice.TryMap(val, (*entity2.TagKey).ToPO).Get()
	if err != nil {
		logs.CtxError(ctx, "[MCreateTagKeys] convert to po failed, bo: %v, err: %v", json.MarshalStringIgnoreErr(val), err)
		return err
	}
	// insert
	if err = t.db.NewSession(ctx, opt...).Create(pps).Error; err != nil {
		return errno.MaybeDBErr(err, "MCreateTagKeys")
	}
	return nil
}

func (t *TagRepoImpl) GetTagKey(ctx context.Context, spaceID, id int64, opts ...db.Option) (*entity2.TagKey, error) {
	pp := &model.TagKey{}
	if err := t.db.NewSession(ctx, opts...).Where("space_id = ? and id = ?", spaceID, id).First(pp).Error; err != nil {
		return nil, errno.MaybeDBErr(err, "GetTagKey")
	}
	return convertor.TagKeyPO2DO(pp)
}

func (t *TagRepoImpl) MGetTagKeys(ctx context.Context, param *entity2.MGetTagKeyParam, opts ...db.Option) ([]*entity2.TagKey, *pagination.PageResult, error) {
	where, err := param.ToWhere()
	if err != nil {
		return nil, nil, err
	}
	var pp []*model.TagKey
	tx := t.db.NewSession(ctx, opts...).Where(where)
	if err = param.Paginator.Find(ctx, tx, &pp).Error; err != nil {
		return nil, nil, errno.MaybeDBErr(err, "MGetTagKeys")
	}
	bos, err := gslice.TryMap(pp, convertor.TagKeyPO2DO).Get()
	if err != nil {
		logs.CtxError(ctx, "[MGetTagKeys] convert po to bo failed, po: %v, err: %+v", json.MarshalStringIgnoreErr(pp), err)
		return nil, nil, err
	}
	return bos, param.Paginator.Result(), nil
}

func (t *TagRepoImpl) PatchTagKey(ctx context.Context, spaceID, id int64, patch *entity2.TagKey, opts ...db.Option) error {
	if spaceID <= 0 || id <= 0 {
		return errno.InvalidParamErrorf("space_id and id are required")
	}

	// convert
	pp, err := patch.ToPO()
	if err != nil {
		logs.CtxError(ctx, "[PatchTagKey] convert to po failed, bo: %v, err: %v", json.MarshalStringIgnoreErr(patch), err)
		return err
	}
	if pp == nil {
		return errno.InvalidParamErrorf("patch is nil")
	}

	result := t.db.NewSession(ctx, opts...).Where("space_id = ? and id = ?", spaceID, id).Updates(pp)
	if err := result.Error; err != nil {
		return errno.MaybeDBErr(err, "PatchTagKey")
	}

	if result.RowsAffected == 0 {
		return errno.InvalidParamErrorf("tagKey not found, space_id: %d, id: %d", spaceID, id)
	}
	return nil
}

func (t *TagRepoImpl) DeleteTagKey(ctx context.Context, spaceID, id int64, opts ...db.Option) error {
	if spaceID <= 0 || id <= 0 {
		return errno.InvalidParamErrorf("space_id and id are required")
	}

	result := t.db.NewSession(ctx, opts...).Where("space_id = ? and id = ?", spaceID, id).Delete(&model.TagKey{})
	if err := result.Error; err != nil {
		return errno.MaybeDBErr(err, "DeleteTagKey")
	}
	if result.RowsAffected == 0 {
		return errno.InvalidParamErrorf("tagKey not found, space_id: %d, id: %d", spaceID, id)
	}
	return nil
}

func (t *TagRepoImpl) UpdateTagKeysStatus(ctx context.Context, spaceID, tagKeyID int64, versionNum int32, toStatus entity2.TagStatus, updateInfo bool, opts ...db.Option) error {
	if spaceID <= 0 || tagKeyID <= 0 {
		return errno.InvalidParamErrorf("space_id and tagKeyID are required")
	}
	updates := map[string]interface{}{
		"status": toStatus,
	}
	if updateInfo {
		updates["updated_at"] = time.Now()
		updates["updated_by"] = session.UserIDInCtxOrEmpty(ctx)

	}
	tx := t.db.NewSession(ctx, opts...).Model(&model.TagKey{})
	if !updateInfo {
		tx = tx.Omit("updated_at")
	}
	result := tx.Where("space_id = ? and tag_key_id = ? and version_num = ?", spaceID, tagKeyID, versionNum).Updates(updates)
	if err := result.Error; err != nil {
		return errno.MaybeDBErr(err, "UpdateTagKeysStatus")
	}
	return nil
}

func (t *TagRepoImpl) CountTagKeys(ctx context.Context, param *entity2.MGetTagKeyParam, opts ...db.Option) (int64, error) {
	where, err := param.ToWhere()
	if err != nil {
		logs.CtxError(ctx, "[CountTagKeys] param is illegal, err: %+v", err)
		return 0, errno.InvalidParamErr(err)
	}
	var res int64
	result := t.db.NewSession(ctx, opts...).Model(&model.TagKey{}).Where(where).Count(&res)
	if result.Error != nil {
		logs.CtxError(ctx, "[CountTagKeys] count tag keys failed, err: %+v", err)
		return 0, errno.MaybeDBErr(result.Error, "CountTagKeys")
	}
	return res, nil
}
