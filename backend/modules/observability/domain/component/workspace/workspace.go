// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
)

//go:generate mockgen -destination=mocks/workspace_provider.go -package=mocks . IWorkSpaceProvider
type IWorkSpaceProvider interface {
	GetIngestWorkSpaceID(ctx context.Context, spans []*span.InputSpan) string
}
