// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func OrderByDTO2DO(orderBy *common.OrderBy) *entity.OrderBy {
	if orderBy == nil {
		return nil
	}
	return &entity.OrderBy{
		Field: orderBy.GetField(),
		IsAsc: orderBy.GetIsAsc(),
	}
}

func OrderByDO2DTO(orderBy *entity.OrderBy) *common.OrderBy {
	if orderBy == nil {
		return nil
	}
	return &common.OrderBy{
		Field: ptr.Of(orderBy.Field),
		IsAsc: ptr.Of(orderBy.IsAsc),
	}
}
