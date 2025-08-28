// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/workspace"
)

func NewWorkspaceProvider() workspace.IWorkSpaceProvider {
	return &WorkspaceProviderImpl{}
}

type WorkspaceProviderImpl struct{}

func (t *WorkspaceProviderImpl) GetIngestWorkSpaceID(ctx context.Context, spans []*span.InputSpan) string {
	if len(spans) == 0 {
		return ""
	}
	return spans[0].WorkspaceID
}
