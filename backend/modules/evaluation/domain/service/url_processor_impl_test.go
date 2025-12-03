// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDefaultURLProcessor(t *testing.T) {
	processor := NewDefaultURLProcessor()
	assert.NotNil(t, processor)
	assert.IsType(t, &DefaultURLProcessor{}, processor)
}

func TestDefaultURLProcessor_ProcessSignURL(t *testing.T) {
	ctx := context.Background()
	processor := NewDefaultURLProcessor()

	tests := []struct {
		name     string
		signURL  string
		setupEnv func()
		want     string
	}{
		{
			name:    "正常URL处理",
			signURL: "https://example.com/path?param=value",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "example.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "https://example.com/path?param=value",
		},
		{
			name:    "本地主机URL处理 - 匹配本地主机",
			signURL: "https://test.com/path/to/file?query=123",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "test.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "https://test.com/path/to/file?query=123",
		},
		{
			name:    "本地主机URL处理 - 带端口",
			signURL: "https://localhost:8080/api/download?token=abc",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "localhost")
				os.Setenv("COZE_LOOP_OSS_PORT", "8080")
			},
			want: "/api/download?token=abc",
		},
		{
			name:    "URL解码 - Unicode转义",
			signURL: "https://example.com/path\u0026file?param=value\u003d123",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "other.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "https://example.com/path&file?param=value=123",
		},
		{
			name:    "URL解码 - 普通URL编码",
			signURL: "https://example.com/path%20with%20spaces?param=%E4%B8%AD%E6%96%87",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "other.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "https://example.com/path with spaces?param=中文",
		},
		{
			name:    "URL解码 - 混合编码",
			signURL: "https://example.com/path\u0020with%20spaces?param=value\u003d123%26end",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "other.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "https://example.com/path with spaces?param=value=123&end",
		},
		{
			name:    "无效URL - 仍然返回原始值",
			signURL: "not-a-valid-url",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "example.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "not-a-valid-url",
		},
		{
			name:    "空URL",
			signURL: "",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "example.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "",
		},
		{
			name:    "URL解码失败 - 仍然继续处理",
			signURL: "https://example.com/path%ZZinvalid?param=value",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "other.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "https://example.com/path%ZZinvalid?param=value",
		},
		{
			name:    "本地主机URL处理 - 不匹配的主机",
			signURL: "https://external.com/path?param=value",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "internal.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "https://external.com/path?param=value",
		},
		{
			name:    "复杂路径和查询参数",
			signURL: "https://test.com/api/v1/users/123/files/document.pdf?token=abc123&expires=1234567890&signature=xyz%3D%3D",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "test.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "https://test.com/api/v1/users/123/files/document.pdf?token=abc123&expires=1234567890&signature=xyz==",
		},
		{
			name:    "URL解码 - 只有Unicode转义",
			signURL: "https://example.com/api\u002Ftest\u003Fparam\u003Dvalue",
			setupEnv: func() {
				os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
				os.Setenv("COZE_LOOP_OSS_DOMAIN", "other.com")
				os.Setenv("COZE_LOOP_OSS_PORT", "")
			},
			want: "https://example.com/api/test?param=value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清理环境变量
			os.Clearenv()
			
			// 设置测试环境
			if tt.setupEnv != nil {
				tt.setupEnv()
			}

			// 执行测试
			got := processor.ProcessSignURL(ctx, tt.signURL)
			
			// 验证结果
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultURLProcessor_ProcessSignURL_Parallel(t *testing.T) {
	ctx := context.Background()
	processor := NewDefaultURLProcessor()

	t.Run("并行处理测试", func(t *testing.T) {
		t.Parallel()
		
		// 设置环境变量
		os.Setenv("COZE_LOOP_OSS_PROTOCOL", "https")
		os.Setenv("COZE_LOOP_OSS_DOMAIN", "parallel.com")
		os.Setenv("COZE_LOOP_OSS_PORT", "")

		signURL := "https://parallel.com/test?param=value"
		want := "https://parallel.com/test?param=value"
		
		got := processor.ProcessSignURL(ctx, signURL)
		assert.Equal(t, want, got)
	})
}