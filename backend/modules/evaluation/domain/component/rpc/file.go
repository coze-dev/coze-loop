// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/cloudwego/kitex/client/callopt"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file"
)

type IFileRPCAdapter interface {
	UploadLoopFileInner(ctx context.Context, req *file.UploadLoopFileInnerRequest, callOptions ...callopt.Option) (r *file.UploadLoopFileInnerResponse, err error)
	// SignUploadFile(ctx context.Context, req *file.SignUploadFileRequest, callOptions ...callopt.Option) (r *file.SignUploadFileResponse, err error)
	// SignDownloadFile(ctx context.Context, req *file.SignDownloadFileRequest, callOptions ...callopt.Option) (r *file.SignDownloadFileResponse, err error)
	GetFileURL(ctx context.Context, key string) (url string, err error)
}

//go:generate mockgen -destination=mocks/file_provider.go -package=mocks . IFileProvider
type IFileProvider interface {
	MGetFileURL(ctx context.Context, keys []string) (urls map[string]string, err error)
}
