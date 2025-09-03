// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tag

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag/tagservice"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type TagRPCAdapter struct {
	client tagservice.Client
}

func NewTagRPCProvider(client tagservice.Client) rpc.ITagRPCAdapter {
	return &TagRPCAdapter{
		client: client,
	}
}

func (t *TagRPCAdapter) GetTagInfo(ctx context.Context, workspaceID int64, tagID int64) (*entity.TagInfo, error) {
	res, err := t.client.BatchGetTags(ctx, &tag.BatchGetTagsRequest{
		WorkspaceID: workspaceID,
		TagKeyIds:   []int64{tagID},
	})
	if err != nil {
		return nil, err
	} else if len(res.TagInfoList) == 0 {
		return nil, fmt.Errorf("tag info not found")
	} else if len(res.TagInfoList) > 1 {
		logs.CtxWarn(ctx, "Multiple tag infos found for %d", tagID)
	}
	tagInfo := res.TagInfoList[0]
	return TagDTO2DO(tagInfo), nil
}

func (t *TagRPCAdapter) BatchGetTagInfo(ctx context.Context, workspaceID int64, tagIDs []int64) (map[int64]*entity.TagInfo, error) {
	if len(tagIDs) == 0 {
		return nil, nil
	}
	res, err := t.client.BatchGetTags(ctx, &tag.BatchGetTagsRequest{
		WorkspaceID: workspaceID,
		TagKeyIds:   tagIDs,
	})
	if err != nil {
		return nil, err
	} else if len(res.TagInfoList) == 0 {
		return nil, fmt.Errorf("tag info not found")
	}
	tagList := TagListDTO2DO(res.TagInfoList)
	tagMap := lo.Associate(tagList, func(item *entity.TagInfo) (int64, *entity.TagInfo) {
		return item.TagKeyId, item
	})
	return tagMap, nil
}
