// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package notifications

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ctxKey 是内部 context key 类型，避免与其它包的 key 冲突。
type ctxKey struct{}

// notificationsCtxKey 用于在 request-scoped context 中透传已解析的 notification rules，
// 从 SubmitExperimentOApi handler 一路带到 experimentApp.CreateExperiment，
// 让 domain 层 Experiment 实例携带 rules 而无需改动 kitex_gen CreateExperimentRequest IDL。
var notificationsCtxKey = ctxKey{}

// WithRules 将 rules 挂到 ctx 上。rules 为 nil 时返回原 ctx。
// 语义上等价于「本请求已解析出的 notification 规则」，落到下游 CreateExperiment / CreateExpt 时读回。
func WithRules(ctx context.Context, rules []entity.NotificationRule) context.Context {
	if rules == nil {
		return ctx
	}
	return context.WithValue(ctx, notificationsCtxKey, rules)
}

// RulesFromContext 读回 ctx 上的 rules，未挂或类型不符返回 nil。
func RulesFromContext(ctx context.Context) []entity.NotificationRule {
	if ctx == nil {
		return nil
	}
	v, ok := ctx.Value(notificationsCtxKey).([]entity.NotificationRule)
	if !ok {
		return nil
	}
	return v
}
