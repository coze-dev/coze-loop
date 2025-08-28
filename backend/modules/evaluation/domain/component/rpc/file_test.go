// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
)

func TestIFileProvider_MGetFileURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockIFileProvider(ctrl)

	tests := []struct {
		name      string
		setupMock func()
		keys      []string
		wantUrls  map[string]string
		wantErr   bool
	}{
		{
			name: "success - single key",
			setupMock: func() {
				mockProvider.EXPECT().
					MGetFileURL(gomock.Any(), []string{"key1"}).
					Return(map[string]string{"key1": "https://example.com/file1"}, nil).
					Times(1)
			},
			keys:     []string{"key1"},
			wantUrls: map[string]string{"key1": "https://example.com/file1"},
			wantErr:  false,
		},
		{
			name: "success - multiple keys",
			setupMock: func() {
				mockProvider.EXPECT().
					MGetFileURL(gomock.Any(), []string{"key1", "key2"}).
					Return(map[string]string{
						"key1": "https://example.com/file1",
						"key2": "https://example.com/file2",
					}, nil).
					Times(1)
			},
			keys: []string{"key1", "key2"},
			wantUrls: map[string]string{
				"key1": "https://example.com/file1",
				"key2": "https://example.com/file2",
			},
			wantErr: false,
		},
		{
			name: "success - empty keys",
			setupMock: func() {
				mockProvider.EXPECT().
					MGetFileURL(gomock.Any(), []string{}).
					Return(map[string]string{}, nil).
					Times(1)
			},
			keys:     []string{},
			wantUrls: map[string]string{},
			wantErr:  false,
		},
		{
			name: "error - provider returns error",
			setupMock: func() {
				mockProvider.EXPECT().
					MGetFileURL(gomock.Any(), []string{"key1"}).
					Return(nil, errors.New("provider error")).
					Times(1)
			},
			keys:     []string{"key1"},
			wantUrls: nil,
			wantErr:  true,
		},
		{
			name: "success - nil input handled",
			setupMock: func() {
				mockProvider.EXPECT().
					MGetFileURL(gomock.Any(), nil).
					Return(nil, nil).
					Times(1)
			},
			keys:     nil,
			wantUrls: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.setupMock()

			urls, err := mockProvider.MGetFileURL(context.Background(), tt.keys)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, urls)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantUrls, urls)
		})
	}
}

func TestIFileProvider_Interface(t *testing.T) {
	t.Parallel()

	// 测试接口方法签名
	var provider rpc.IFileProvider
	assert.Nil(t, provider) // 接口变量默认为nil

	// 验证接口方法存在
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProvider := mocks.NewMockIFileProvider(ctrl)

	// 验证方法签名正确
	mockProvider.EXPECT().
		MGetFileURL(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		Times(1)

	_, _ = mockProvider.MGetFileURL(context.Background(), []string{})
}
