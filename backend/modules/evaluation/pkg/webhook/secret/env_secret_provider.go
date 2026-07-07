// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package secret 提供 webhook 签名密钥来源实现。
//
// EnvSecretProvider 走 OSS 静态 secret 路径(env FORNAX_WEBHOOK_STATIC_SECRET,
// 对齐 test_case 21);env 未配置时返回空 → signer 走降级路径 "sha256="
// (对齐 test_case 22)。商业侧 workspace_sk 由 configer.GetWebhookSecurityConf
// 提供 → 独立 impl,后一轮 wire。
package secret

import (
	"context"
	"os"
)

// EnvSecretKey OSS 静态 secret 的 env 变量名(tech_design 已锁)。
const EnvSecretKey = "FORNAX_WEBHOOK_STATIC_SECRET"

// EnvSecretProvider 从 env 读固定 secret,忽略 workspaceID。
// 实现 dispatcher.SecretProvider 接口(通过 duck typing,不引入 dispatcher 包避免环)。
type EnvSecretProvider struct{}

// NewEnvSecretProvider 供 wire DI 使用。
func NewEnvSecretProvider() *EnvSecretProvider { return &EnvSecretProvider{} }

// GetSecret 返回 env FORNAX_WEBHOOK_STATIC_SECRET;env 未配置返 ""(不报错,由 signer 走降级)。
func (*EnvSecretProvider) GetSecret(_ context.Context, _ int64) (string, error) {
	return os.Getenv(EnvSecretKey), nil
}
