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

// NewDefaultURLProcessor 创建默认的URL处理器实例
func NewDefaultURLProcessor() component.IURLProcessor {
	return &DefaultURLProcessor{}
}

// ProcessSignURL 处理签名URL，包括本地主机处理
func (p *DefaultURLProcessor) ProcessSignURL(ctx context.Context, signURL string) string {
	logs.CtxInfo(ctx, "get export record sign url origin: %v", signURL)
	parsedURL, err := url.Parse(conv.UnescapeUnicode(signURL))
	if err != nil {
		logs.CtxWarn(ctx, "Parse URL fail, raw: %v", signURL)
		return signURL
	}

	// 关键：签名 URL 的 path / query 必须与签名服务（对象存储/CDN）签发时的 percent-encoding 逐字节一致，
	// 否则 ① x-signature 等签名是针对编码后的 path 计算的，path 一旦被改写签名即失效；
	// ② 文件名里的 '/' 若被反转义会变成真正的路径分隔符，破坏 object key 结构导致 404。
	// 历史实现为了让中文文件名“可读”对整个 URL 做了 QueryUnescape，对中文恰好不破坏 path 合法性，
	// 但对实验名含 '[' ']' '/' 空格 等字符的场景会破坏 path 与签名，导致导出下载失败。
	// 因此这里不再改动 path / query 的编码，原样透传签发结果；文件名展示交由下载响应头处理。
	escapedPath := parsedURL.EscapedPath()
	rawQuery := parsedURL.RawQuery

	// localos（本地对象存储）场景下仅去掉 scheme+host，保留编码后的 path 与 query。
	if parsedURL.Host == localos.GetLocalOSHost() {
		if rawQuery != "" {
			signURL = fmt.Sprintf("%s?%s", escapedPath, rawQuery)
		} else {
			signURL = escapedPath
		}
		logs.CtxInfo(ctx, "get export record sign url final(localos): %v", signURL)
		return signURL
	}

	if parsedURL.Scheme != "" && parsedURL.Host != "" {
		signURL = fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, escapedPath)
	} else {
		signURL = escapedPath
	}
	if rawQuery != "" {
		signURL = fmt.Sprintf("%s?%s", signURL, rawQuery)
	}

	logs.CtxInfo(ctx, "get export record sign url final: %v", signURL)
	return signURL
}
