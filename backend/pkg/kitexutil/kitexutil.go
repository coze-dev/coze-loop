// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package kitexutil

import (
	"context"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func GetTOMethod(ctx context.Context) string {
	if rpcinfo.GetRPCInfo(ctx) != nil && rpcinfo.GetRPCInfo(ctx).To() != nil {
		return rpcinfo.GetRPCInfo(ctx).To().Method()
	}
	return ""
}