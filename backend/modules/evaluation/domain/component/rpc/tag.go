// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/tag.go -package=mocks . ITagRPCAdapter
type ITagRPCAdapter interface {
	GetTagInfo(context.Context, int64, int64) (*entity.TagInfo, error)
	BatchGetTagInfo(context.Context, int64, []int64) (map[int64]*entity.TagInfo, error)
}
