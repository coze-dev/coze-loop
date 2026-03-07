// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import "context"

type IWorkflowProvider interface {
	BatchGetWorkflows(ctx context.Context, spaceIDs []string) (map[string]string, error)
}
