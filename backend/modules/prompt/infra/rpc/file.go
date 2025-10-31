// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file/fileservice"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

//go:generate mockgen -source=../../../../kitex_gen/coze/loop/foundation/file/fileservice/client.go -destination=mocks/fileservice_mock.go -package=mocks -mock_names Client=FileServiceClient
type FileRPCAdapter struct {
	client fileservice.Client
}

func NewFileRPCProvider(client fileservice.Client) rpc.IFileProvider {
	return &FileRPCAdapter{
		client: client,
	}
}

func (f *FileRPCAdapter) MGetFileURL(ctx context.Context, keys []string) (urls map[string]string, err error) {
	var ttl int64 = 24 * 60 * 60
	req := &file.SignDownloadFileRequest{
		Keys: keys,
		Option: &file.SignFileOption{
			TTL: ptr.Of(ttl),
		},
		BusinessType: ptr.Of(file.BusinessTypePrompt),
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

func (f *FileRPCAdapter) UploadFileForServer(ctx context.Context, mimeType string, body []byte, workspaceID int64) (key string, err error) {
	req := &file.UploadFileForServerRequest{
		MimeType:    mimeType,
		Body:        body,
		WorkspaceID: workspaceID,
	}
	resp, err := f.client.UploadFileForServer(ctx, req)
	if err != nil {
		return "", err
	}
	if resp.Data == nil || resp.Data.FileName == nil {
		return "", errorx.New("upload file response invalid: missing file name")
	}
	return *resp.Data.FileName, nil
}
