// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"
)

//go:generate mockgen -destination=mocks/url_processor.go -package=mocks . IURLProcessor
type IURLProcessor interface {
	// ProcessSignURL 处理签名 URL，包括 URL 解码和本地主机处理
	ProcessSignURL(ctx context.Context, signURL string) string
}
