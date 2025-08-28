// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpcerror

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// UnwrapRPCError 包装RPC错误
func UnwrapRPCError(err error) error {
	if statusErr, ok := errorx.FromStatusError(err); ok {
		return statusErr
	}
	return errorx.WrapByCode(err, errno.CommonRPCErrorCode)
}
