// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
)

const (
	AuthActionFileUpload        = "uploadFile"
	AuthActionOpenAPIFileUpload = "uploadLoopFile"
)

type IAuthProvider interface {
	CheckWorkspacePermission(ctx context.Context, action, workspaceId string) error
}
