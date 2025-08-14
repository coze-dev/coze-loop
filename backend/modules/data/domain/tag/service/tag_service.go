// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	entity2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

//go:generate mockgen -destination=mocks/tag_service_mock.go -package=mocks . ITagService
type ITagService interface {
	// CreateTag 新建 tag key & tag value.
	CreateTag(ctx context.Context, spaceID int64, val *entity2.TagKey, opts ...db.Option) (int64, error)

	// GetAllTagKeyVersionsByKeyID 获取指定tagKeyID的所有版本.
	GetAllTagKeyVersionsByKeyID(ctx context.Context, spaceID, tagKeyID int64, opts ...db.Option) ([]*entity2.TagKey, error)

	// GetAndBuildTagValues 获取指定tagKeyID和版本的选项，并建立树
	GetAndBuildTagValues(ctx context.Context, spaceID, tagKeyID int64, versionNum int32, opts ...db.Option) ([]*entity2.TagValue, error)

	// GetLatestTag 获取最新的tagKey和tagValue，并建树
	GetLatestTag(ctx context.Context, spaceID, tagKeyID int64, opts ...db.Option) (*entity2.TagKey, error)

	// UpdateTag 同时更新 tag key 和 tag value，生成新版本.
	UpdateTag(ctx context.Context, spaceID, tagKeyID int64, val *entity2.TagKey, opts ...db.Option) error

	// UpdateTagStatus 改 tag key 和 tag value 的状态, 不生成新版本.
	UpdateTagStatus(ctx context.Context, spaceID, tagKeyID int64, versionNum int32, status entity2.TagStatus, needLock, updatedInfo bool, opts ...db.Option) error

	// UpdateTagStatusWithNewVersion 改 tag key 和 tag value 的状态, 生成新版本.
	UpdateTagStatusWithNewVersion(ctx context.Context, spaceID, tagKeyID int64, status entity2.TagStatus) error

	// GetTagSpec 获取标签规格
	GetTagSpec(ctx context.Context, spaceID int64) (maxHeight, maxWidth, macTotal int64, err error)

	// BatchUpdateTagStatus 批量更新tag key状态
	BatchUpdateTagStatus(ctx context.Context, spaceID int64, tagKeyIDs []int64, toStatus entity2.TagStatus) (map[int64]string, error)

	// SearchTags 搜索Tag
	SearchTags(ctx context.Context, spaceID int64, param *entity2.MGetTagKeyParam) ([]*entity2.TagKey, *pagination.PageResult, error)

	// GetTagDetail 获取Tag详情
	GetTagDetail(ctx context.Context, spaceID int64, param *entity2.GetTagDetailReq) (*entity2.GetTagDetailResp, error)

	// BatchGetTagsByTagKeyIDs 通过tagKeyID获取tag key信息
	BatchGetTagsByTagKeyIDs(ctx context.Context, spaceID int64, tagKeyIDs []int64) ([]*entity2.TagKey, error)
}
