// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tag

import (
	"context"
	"fmt"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/tag/tagservice"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/tag/convertor"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

type TagRPCAdapter struct {
	client tagservice.Client
}

func NewTagRPCProvider(client tagservice.Client) rpc.ITagRPCAdapter {
	return &TagRPCAdapter{
		client: client,
	}
}

func (t *TagRPCAdapter) GetTagInfo(ctx context.Context, workspaceID int64, tagIDStr string) (*rpc.TagInfo, error) {
	id, err := strconv.ParseInt(tagIDStr, 10, 64)
	if err != nil {
		return nil, err
	}
	res, err := t.client.BatchGetTags(ctx, &tag.BatchGetTagsRequest{
		WorkspaceID: workspaceID,
		TagKeyIds:   []int64{id},
	})
	if err != nil {
		return nil, err
	} else if len(res.TagInfoList) == 0 {
		return nil, fmt.Errorf("tag info not found")
	} else if len(res.TagInfoList) > 1 {
		logs.CtxWarn(ctx, "Multiple tag infos found for %d", id)
	}
	tagInfo := res.TagInfoList[0]
	return convertor.TagDTO2DO(tagInfo), nil
}

func (t *TagRPCAdapter) BatchGetTagInfo(ctx context.Context, workspaceID int64, tagIDs []string) (map[int64]*rpc.TagInfo, error) {
	if len(tagIDs) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		id, err := strconv.ParseInt(tagID, 10, 64)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	res, err := t.client.BatchGetTags(ctx, &tag.BatchGetTagsRequest{
		WorkspaceID: workspaceID,
		TagKeyIds:   ids,
	})
	if err != nil {
		return nil, err
	} else if len(res.TagInfoList) == 0 {
		return nil, fmt.Errorf("tag info not found")
	}
	tagList := convertor.TagListDTO2DO(res.TagInfoList)
	tagMap := lo.Associate(tagList, func(item *rpc.TagInfo) (int64, *rpc.TagInfo) {
		return item.TagKeyId, item
	})
	return tagMap, nil
}
