// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import "context"

const (
	AuthActionTraceRead          = "readLoopTrace"
	AuthActionTraceIngest        = "ingestLoopTrace"
	AuthActionTraceList          = "listLoopTrace"
	AuthActionTraceViewCreate    = "createLoopTraceView"
	AuthActionTraceViewList      = "listLoopTraceView"
	AuthActionTraceViewEdit      = "edit"
	AuthActionAnnotationCreate   = "createLoopTraceAnnotation"
	AuthActionAnnotationDelete   = "deleteLoopTraceAnnotation"
	AuthActionTraceExport        = "exportLoopTrace"
	AuthActionTracePreviewExport = "previewExportLoopTrace"
	AuthActionTraceTaskCreate    = "createLoopTask"
	AuthActionTraceTaskList      = "listLoopTask"
	AuthActionTraceTaskEdit      = "edit"
	AuthActionTraceMetricRead    = "readLoopIndictor"
)

//go:generate mockgen -destination=mocks/auth_provider.go -package=mocks . IAuthProvider
type IAuthProvider interface {
	CheckWorkspacePermission(ctx context.Context, action, workspaceId string, isOpi bool) error
	CheckViewPermission(ctx context.Context, action, workspaceId, viewId string) error
	CheckIngestPermission(ctx context.Context, workspaceId string) error
	CheckQueryPermission(ctx context.Context, workspaceId, platformType string) error
	CheckTaskPermission(ctx context.Context, action, workspaceId, taskId string) error
	GetClaim(ctx context.Context) *Claim
}

type Claim struct {
	AuthType         string            `json:"auth_type"`
	ThirdPartyClient *ThirdPartyClient `json:"third_party_client,omitempty"`
}

type ThirdPartyClient struct {
	BizScene string `json:"biz_scene"`
	BizID    string `json:"biz_id"`
}
