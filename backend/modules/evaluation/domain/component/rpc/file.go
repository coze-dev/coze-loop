// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"time"

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

type EvidenceArchiveDownloadRequest struct {
	CallerSpaceID      int64
	RecordID           int64
	ResourceSpaceID    int64
	EvaluatorVersionID int64
	ObjectKey          string
	Status             string
}

// IEvidenceArchiveURLProvider authorizes each record before signing. A missing
// record ID in the response denies access to both archive metadata and its URL.
type IEvidenceArchiveURLProvider interface {
	MGetEvidenceArchiveDownloadURL(ctx context.Context, requests []*EvidenceArchiveDownloadRequest, ttl time.Duration) (urls map[int64]string, err error)
}
