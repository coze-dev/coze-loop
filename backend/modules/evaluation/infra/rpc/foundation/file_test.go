// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package foundation

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file/fileservice"
)

//go:generate mockgen -destination=mocks/fileservice_client.go -package=mocks github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file/fileservice Client

func TestNewFileRPCProvider(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 这里我们需要创建一个mock client，但由于kitex生成的Client是接口，
	// 我们可以直接使用nil来测试构造函数
	var mockClient fileservice.Client

	provider := NewFileRPCProvider(mockClient)

	assert.NotNil(t, provider)
	assert.IsType(t, &FileRPCAdapter{}, provider)
}

func TestFileRPCAdapter_MGetFileURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name        string
		keys        []string
		setupMock   func(*mockFileServiceClient)
		wantUrls    map[string]string
		wantErr     bool
		expectedErr string
	}{
		{
			name: "success - single key",
			keys: []string{"key1"},
			setupMock: func(mockClient *mockFileServiceClient) {
				mockClient.mockResponse = &file.SignDownloadFileResponse{
					Uris: []string{"https://example.com/file1"},
				}
				mockClient.mockError = nil
			},
			wantUrls: map[string]string{"key1": "https://example.com/file1"},
			wantErr:  false,
		},
		{
			name: "success - multiple keys",
			keys: []string{"key1", "key2"},
			setupMock: func(mockClient *mockFileServiceClient) {
				mockClient.mockResponse = &file.SignDownloadFileResponse{
					Uris: []string{"https://example.com/file1", "https://example.com/file2"},
				}
				mockClient.mockError = nil
			},
			wantUrls: map[string]string{
				"key1": "https://example.com/file1",
				"key2": "https://example.com/file2",
			},
			wantErr: false,
		},
		{
			name: "success - empty keys",
			keys: []string{},
			setupMock: func(mockClient *mockFileServiceClient) {
				mockClient.mockResponse = &file.SignDownloadFileResponse{
					Uris: []string{},
				}
				mockClient.mockError = nil
			},
			wantUrls: map[string]string{},
			wantErr:  false,
		},
		{
			name: "error - client returns error",
			keys: []string{"key1"},
			setupMock: func(mockClient *mockFileServiceClient) {
				mockClient.mockResponse = nil
				mockClient.mockError = errors.New("rpc error")
			},
			wantUrls: nil,
			wantErr:  true,
		},
		{
			name: "error - url length mismatch",
			keys: []string{"key1", "key2"},
			setupMock: func(mockClient *mockFileServiceClient) {
				mockClient.mockResponse = &file.SignDownloadFileResponse{
					Uris: []string{"https://example.com/file1"}, // 只有一个URL，但有两个key
				}
				mockClient.mockError = nil
			},
			wantUrls:    nil,
			wantErr:     true,
			expectedErr: "url length mismatch with keys",
		},
		{
			name: "success - nil keys handled",
			keys: nil,
			setupMock: func(mockClient *mockFileServiceClient) {
				mockClient.mockResponse = &file.SignDownloadFileResponse{
					Uris: []string{},
				}
				mockClient.mockError = nil
			},
			wantUrls: map[string]string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 为每个测试创建独立的mock客户端
			mockClient := &mockFileServiceClient{}
			adapter := &FileRPCAdapter{client: mockClient}
			tt.setupMock(mockClient)

			urls, err := adapter.MGetFileURL(context.Background(), tt.keys)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedErr != "" {
					assert.Contains(t, err.Error(), tt.expectedErr)
				}
				assert.Nil(t, urls)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantUrls, urls)
		})
	}
}

func TestFileRPCAdapter_MGetFileURL_RequestValidation(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := &mockFileServiceClient{}
	adapter := &FileRPCAdapter{client: mockClient}

	keys := []string{"test-key"}

	// 设置mock返回
	mockClient.mockResponse = &file.SignDownloadFileResponse{
		Uris: []string{"https://example.com/test-file"},
	}
	mockClient.mockError = nil

	// 执行调用
	_, err := adapter.MGetFileURL(context.Background(), keys)
	assert.NoError(t, err)

	// 验证请求参数
	assert.NotNil(t, mockClient.lastRequest)
	assert.Equal(t, keys, mockClient.lastRequest.Keys)
	assert.NotNil(t, mockClient.lastRequest.Option)
	assert.Equal(t, int64(24*60*60*7), *mockClient.lastRequest.Option.TTL) // 7天TTL
	assert.NotNil(t, mockClient.lastRequest.BusinessType)
	assert.Equal(t, file.BusinessTypeEvaluation, *mockClient.lastRequest.BusinessType)
}

// mockFileServiceClient 是一个简单的mock实现，用于测试
// 使用互斥锁来避免数据竞争
type mockFileServiceClient struct {
	mu           sync.RWMutex
	mockResponse *file.SignDownloadFileResponse
	mockError    error
	lastRequest  *file.SignDownloadFileRequest
}

func (m *mockFileServiceClient) UploadLoopFileInner(ctx context.Context, req *file.UploadLoopFileInnerRequest, callOptions ...callopt.Option) (r *file.UploadLoopFileInnerResponse, err error) {
	return nil, errors.New("not implemented")
}

func (m *mockFileServiceClient) SignUploadFile(ctx context.Context, req *file.SignUploadFileRequest, callOptions ...callopt.Option) (r *file.SignUploadFileResponse, err error) {
	return nil, errors.New("not implemented")
}

func (m *mockFileServiceClient) SignDownloadFile(ctx context.Context, req *file.SignDownloadFileRequest, callOptions ...callopt.Option) (r *file.SignDownloadFileResponse, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastRequest = req
	return m.mockResponse, m.mockError
}
