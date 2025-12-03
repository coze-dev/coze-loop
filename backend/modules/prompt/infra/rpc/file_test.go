// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewFileRPCProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewFileServiceClient(ctrl)
	provider := NewFileRPCProvider(mockClient)

	assert.NotNil(t, provider)
	adapter, ok := provider.(*FileRPCAdapter)
	assert.True(t, ok)
	assert.Equal(t, mockClient, adapter.client)
}

func TestFileRPCAdapter_MGetFileURL(t *testing.T) {
	tests := []struct {
		name       string
		keys       []string
		setupMock  func(*mocks.FileServiceClient)
		expectErr  string
		expectURLs map[string]string
	}{
		{
			name: "success - returns url map",
			keys: []string{"file-1", "file-2"},
			setupMock: func(mc *mocks.FileServiceClient) {
				mc.EXPECT().SignDownloadFile(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *file.SignDownloadFileRequest, _ ...callopt.Option) (*file.SignDownloadFileResponse, error) {
						assert.Equal(t, []string{"file-1", "file-2"}, req.Keys)
						if assert.NotNil(t, req.Option) {
							assert.Equal(t, int64(24*60*60), req.Option.GetTTL())
						}
						if assert.NotNil(t, req.BusinessType) {
							assert.Equal(t, file.BusinessTypePrompt, *req.BusinessType)
						}
						return &file.SignDownloadFileResponse{
							Uris: []string{"https://file-1", "https://file-2"},
						}, nil
					},
				)
			},
			expectURLs: map[string]string{
				"file-1": "https://file-1",
				"file-2": "https://file-2",
			},
		},
		{
			name: "failure - mismatched uri count",
			keys: []string{"file-1", "file-2"},
			setupMock: func(mc *mocks.FileServiceClient) {
				mc.EXPECT().SignDownloadFile(gomock.Any(), gomock.Any()).Return(
					&file.SignDownloadFileResponse{
						Uris: []string{"https://file-1"},
					},
					nil,
				)
			},
			expectErr: "url length mismatch with keys",
		},
		{
			name: "failure - rpc error",
			keys: []string{"file-1"},
			setupMock: func(mc *mocks.FileServiceClient) {
				mc.EXPECT().SignDownloadFile(gomock.Any(), gomock.Any()).Return(nil, errors.New("sign download failed"))
			},
			expectErr: "sign download failed",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewFileServiceClient(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			adapter := &FileRPCAdapter{
				client: mockClient,
			}

			urls, err := adapter.MGetFileURL(context.Background(), tt.keys)

			if tt.expectErr != "" {
				assert.ErrorContains(t, err, tt.expectErr)
				assert.Nil(t, urls)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectURLs, urls)
		})
	}
}

func TestFileRPCAdapter_UploadFileForServer(t *testing.T) {
	tests := []struct {
		name         string
		mimeType     string
		body         []byte
		workspaceID  int64
		setupMock    func(*mocks.FileServiceClient)
		expectErr    string
		expectResult string
	}{
		{
			name:        "success - returns uploaded key",
			mimeType:    "image/png",
			body:        []byte("content"),
			workspaceID: 101,
			setupMock: func(mc *mocks.FileServiceClient) {
				mc.EXPECT().UploadFileForServer(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *file.UploadFileForServerRequest, _ ...callopt.Option) (*file.UploadFileForServerResponse, error) {
						assert.Equal(t, "image/png", req.MimeType)
						assert.Equal(t, []byte("content"), req.Body)
						assert.Equal(t, int64(101), req.WorkspaceID)
						return &file.UploadFileForServerResponse{
							Data: &file.FileData{
								FileName: ptr.Of("uploaded-key"),
							},
						}, nil
					},
				)
			},
			expectResult: "uploaded-key",
		},
		{
			name:        "failure - missing file name in response",
			mimeType:    "image/png",
			body:        []byte("content"),
			workspaceID: 101,
			setupMock: func(mc *mocks.FileServiceClient) {
				mc.EXPECT().UploadFileForServer(gomock.Any(), gomock.Any()).Return(
					&file.UploadFileForServerResponse{
						Data: nil,
					},
					nil,
				)
			},
			expectErr: "upload file response invalid: missing file name",
		},
		{
			name:        "failure - rpc error",
			mimeType:    "image/png",
			body:        []byte("content"),
			workspaceID: 101,
			setupMock: func(mc *mocks.FileServiceClient) {
				mc.EXPECT().UploadFileForServer(gomock.Any(), gomock.Any()).Return(nil, errors.New("upload failed"))
			},
			expectErr: "upload failed",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewFileServiceClient(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			adapter := &FileRPCAdapter{
				client: mockClient,
			}

			result, err := adapter.UploadFileForServer(context.Background(), tt.mimeType, tt.body, tt.workspaceID)

			if tt.expectErr != "" {
				assert.ErrorContains(t, err, tt.expectErr)
				assert.Empty(t, result)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectResult, result)
		})
	}
}
