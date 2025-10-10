// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	CtxKeyEnv = "K_ENV"
	XttEnv    = "x_tt_env"
)

func ToJSONString(ctx context.Context, obj interface{}) string {
	if obj == nil {
		return ""
	}
	jsonData, err := sonic.Marshal(obj)
	if err != nil {
		logs.CtxError(ctx, "JSON marshal error: %v", err)
		return ""
	}
	jsonStr := string(jsonData)
	return jsonStr
}

func fillCtxWithEnv(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, CtxKeyEnv, "boe_auto_task")
	if env, ok := metainfo.GetPersistentValue(ctx, XttEnv); ok {
		ctx = context.WithValue(ctx, CtxKeyEnv, env) //nolint:staticcheck,SA1029
	}
	//if env := os.Getenv(XttEnv); env != "" {
	//	ctx = context.WithValue(ctx, CtxKeyEnv, env) //nolint:staticcheck,SA1029
	//}

	return ctx
}
