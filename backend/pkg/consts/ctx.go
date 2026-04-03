// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

type ctxKey string

const (
	CtxKeyLogID         = "K_LOGID"
	CtxKeyFlowMethodKey = ctxKey("X_FLOW_METHOD")

	CookieLanguageKey = "i18next"
	LocaleZhCN        = "zh-CN"
	LocalEnUS         = "en-US"
	LocaleDefault     = LocalEnUS
)
