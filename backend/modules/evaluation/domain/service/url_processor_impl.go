// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"net/url"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/localos"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// DefaultURLProcessor 默认的 URL 处理器实现
type DefaultURLProcessor struct{}

func NewDefaultURLProcessor() component.IURLProcessor {
	return &DefaultURLProcessor{}
}

func (p *DefaultURLProcessor) ProcessSignURL(ctx context.Context, signURL string) string {
	logs.CtxInfo(ctx, "get export record sign url origin: %v", signURL)
	unescaped, err := url.QueryUnescape(conv.UnescapeUnicode(signURL))
	if err != nil {
		logs.CtxWarn(ctx, "QueryUnescape fail, raw: %v", signURL)
	} else {
		signURL = unescaped
	}
	logs.CtxInfo(ctx, "get export record sign url unescaped: %v", signURL)
	parsedURL, err := url.Parse(signURL)
	if err == nil {
		if parsedURL.Host == localos.GetLocalOSHost() {
			signURL = fmt.Sprintf("%s?%s", parsedURL.Path, parsedURL.RawQuery)
		}
	}
	return signURL
}

