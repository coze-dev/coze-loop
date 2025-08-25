// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"context"
)

//go:generate mockgen -destination=mocks/workspace_provider.go -package=mocks . IWorkSpaceProvider
type IWorkSpaceProvider interface {
	GetIngestWorkSpaceID(ctx context.Context, spans []*span.InputSpan) string
}
