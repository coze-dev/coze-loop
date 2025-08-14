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
)

func (t *TagRepoImpl) MCreateTagValues(ctx context.Context, val []*entity2.TagValue, opts ...db.Option) error {
	if len(val) == 0 {
		return nil
	}
	// gen id
	ids, err := t.idGen.GenMultiIDs(ctx, len(val)*2)
	if err != nil {
		return err
	}
	for i, v := range val {
		item := v
		item.ID = ids[i*2]
		if gvalue.IsZero(item.TagValueID) {
			item.TagValueID = ids[i*2+1]
		}
	}
	// gen po
	pps := gslice.Map(val, (*entity2.TagValue).ToPO)
	// insert
	if err = t.db.NewSession(ctx, opts...).Create(pps).Error; err != nil {
		return errno.MaybeDBErr(err, "MCreateTagValues")
	}
	return nil
}

func (t *TagRepoImpl) GetTagValue(ctx context.Context, spaceID, id int64, opts ...db.Option) (*entity2.TagValue, error) {
	if spaceID <= 0 || id <= 0 {
		return nil, errno.BadReqErrorf("space_id and id are required")
	}
	pp := &model.TagValue{}
	if err := t.db.NewSession(ctx, opts...).Where("space_id = ? and id = ?", spaceID, id).First(pp).Error; err != nil {
		return nil, errno.MaybeDBErr(err, "GetTagValue")
	}
	return convertor.TagValuePO2DO(pp), nil
}

func (t *TagRepoImpl) MGetTagValue(ctx context.Context, param *entity2.MGetTagValueParam, opts ...db.Option) ([]*entity2.TagValue, *pagination.PageResult, error) {
	where, err := param.ToWhere()
	if err != nil {
		return nil, nil, err
	}
	var pps []*model.TagValue
	tx := t.db.NewSession(ctx, opts...).Where(where)
	if err = param.Paginator.Find(ctx, tx, &pps).Error; err != nil {
		return nil, nil, errno.MaybeDBErr(err, "MGetTagValue")
	}
	bos := gslice.Map(pps, convertor.TagValuePO2DO)
	return bos, param.Paginator.Result(), nil
}

func (t *TagRepoImpl) PatchTagValue(ctx context.Context, spaceID, id int64, patch *entity2.TagValue, opts ...db.Option) error {
	if spaceID <= 0 || id <= 0 {
		return errno.InvalidParamErrorf("space_id and id are required")
	}
	po := patch.ToPO()
	if po == nil {
		return errno.InvalidParamErrorf("patch is nil")
	}
	result := t.db.NewSession(ctx, opts...).Where("space_id = ? and id = ?", spaceID, id).Updates(po)
	if err := result.Error; err != nil {
		return errno.MaybeDBErr(err, "PatchTagValue")
	}
	if result.RowsAffected == 0 {
		return errno.InvalidParamErrorf("tag value is not exist, space_id: %d, id: %d", spaceID, id)
	}
	return nil
}

func (t *TagRepoImpl) DeleteTagValue(ctx context.Context, spaceID, id int64, opts ...db.Option) error {
	if spaceID <= 0 || id <= 0 {
		return errno.InvalidParamErrorf("space_id and id are required")
	}
	if err := t.db.NewSession(ctx, opts...).Where("space_id = ? and id = ?", spaceID, id).Delete(&model.TagValue{}).Error; err != nil {
		return errno.MaybeDBErr(err, "DeleteTagValue")
	}
	return nil
}

func (t *TagRepoImpl) UpdateTagValuesStatus(ctx context.Context, spaceID, tagKeyID int64, versionNum int32, toStatus entity2.TagStatus, updateInfo bool, opts ...db.Option) error {
	if spaceID <= 0 || tagKeyID <= 0 {
		return errno.BadReqErrorf("space_id and tagKeyID are required")
	}
	updates := map[string]interface{}{
		"status": toStatus,
	}
	if updateInfo {
		updates["updated_at"] = time.Now()
		updates["updated_by"] = session.UserIDInCtxOrEmpty(ctx)
	}
	tx := t.db.NewSession(ctx, opts...).Model(&model.TagValue{})
	if !updateInfo {
		tx = tx.Omit("updated_at")
	}
	result := tx.Where("space_id = ? and tag_key_id = ? and version_num = ?", spaceID, tagKeyID, versionNum).Updates(updates)
	if err := result.Error; err != nil {
		return errno.MaybeDBErr(err)
	}
	return nil
}
