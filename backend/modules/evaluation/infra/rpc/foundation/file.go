// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package foundation

import (
	"context"

	"github.com/cloudwego/kitex/client/callopt"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file/fileservice"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type FileRPCAdapter struct {
	client fileservice.Client
}

func NewFileRPCProvider(client fileservice.Client) rpc.IFileProvider {
	return &FileRPCAdapter{
		client: client,
	}
}

func (f *FileRPCAdapter) MGetFileURL(ctx context.Context, keys []string) (urls map[string]string, err error) {
	var ttl int64 = 24 * 60 * 60 * 7
	req := &file.SignDownloadFileRequest{
		Keys: keys,
		Option: &file.SignFileOption{
			TTL: ptr.Of(ttl),
		},
		BusinessType: ptr.Of(file.BusinessTypeEvaluation),
	}
	resp, err := f.client.SignDownloadFile(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(resp.Uris) != len(keys) {
		return nil, errorx.New("url length mismatch with keys")
	}
	urls = make(map[string]string)
	for idx, key := range keys {
		urls[key] = resp.Uris[idx]
	}
	return urls, nil
}

func (f FileRPCAdapter) UploadLoopFileInner(ctx context.Context, req *file.UploadLoopFileInnerRequest, callOptions ...callopt.Option) (r *file.UploadLoopFileInnerResponse, err error) {
	return f.client.UploadLoopFileInner(ctx, req, callOptions...)
}

func (f *FileRPCAdapter) GetFileURL(ctx context.Context, key string) (url string, err error) {
	var ttl int64 = 100 * 24 * 60 * 60 // 100天
	req := &file.SignDownloadFileRequest{
		Keys: []string{key},
		Option: &file.SignFileOption{
			TTL: ptr.Of(ttl),
		},
		BusinessType: ptr.Of(file.BusinessTypeEvaluation),
	}
	resp, err := f.client.SignDownloadFile(ctx, req)
	if err != nil {
		return "", err
	}
	if len(resp.Uris) != 1 {
		return "", errorx.New("url length mismatch with keys")
	}

	url = resp.Uris[0]

	return url, nil
}
