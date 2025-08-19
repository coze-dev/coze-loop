// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
)

//go:generate mockgen -destination=mocks/config_provider.go -package=mocks . IConfigProvider
type IConfigProvider interface {
	GetPromptHubMaxQPSBySpace(ctx context.Context, spaceID int64) (maxQPS int, err error)

	GetPromptDefaultConfig(ctx context.Context) (config *prompt.PromptDetail, err error)
}
