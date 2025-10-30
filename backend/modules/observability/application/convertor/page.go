// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	kitcommon "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	tracecommon "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func OrderByDTO2DO(orderBy *kitcommon.OrderBy) *tracecommon.OrderBy {
	if orderBy == nil {
		return nil
	}
	return &tracecommon.OrderBy{
		Field: orderBy.GetField(),
		IsAsc: orderBy.GetIsAsc(),
	}
}

func OrderByDO2DTO(orderBy *tracecommon.OrderBy) *kitcommon.OrderBy {
	if orderBy == nil {
		return nil
	}
	return &kitcommon.OrderBy{
		Field: ptr.Of(orderBy.Field),
		IsAsc: ptr.Of(orderBy.IsAsc),
	}
}
