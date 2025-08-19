// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
)

func TestRetryOnNotFound(t *testing.T) {
	tests := []struct {
		name         string
		fn           func(opt ...Option) error
		originalOpts []Option
		wantErr      error
		wantRetries  int
	}{
		{
			name: "success on first try",
			fn: func(opt ...Option) error {
				return nil
			},
			originalOpts: []Option{},
			wantErr:      nil,
			wantRetries:  0,
		},
		{
			name: "success after retry with master",
			fn: func(opt ...Option) error {
				// 检查是否使用了 WithMaster 选项
				o := &option{}
				for _, fn := range opt {
					fn(o)
				}
				if o.withMaster {
					return nil // 使用主库后成功
				}
				return gorm.ErrRecordNotFound // 第一次失败
			},
			originalOpts: []Option{},
			wantErr:      nil,
			wantRetries:  1,
		},
		{
			name: "already using master, no retry",
			fn: func(opt ...Option) error {
				return gorm.ErrRecordNotFound
			},
			originalOpts: []Option{WithMaster()},
			wantErr:      gorm.ErrRecordNotFound,
			wantRetries:  0,
		},
		{
			name: "other error, no retry",
			fn: func(opt ...Option) error {
				return assert.AnError
			},
			originalOpts: []Option{},
			wantErr:      assert.AnError,
			wantRetries:  0,
		},
		{
			name: "fail twice with master",
			fn: func(opt ...Option) error {
				o := &option{}
				for _, fn := range opt {
					fn(o)
				}
				if o.withMaster {
					return gorm.ErrRecordNotFound // 即使使用主库也失败
				}
				return gorm.ErrRecordNotFound // 第一次失败
			},
			originalOpts: []Option{},
			wantErr:      gorm.ErrRecordNotFound,
			wantRetries:  1,
		},
		{
			name: "success after retry with mixed options",
			fn: func(opt ...Option) error {
				o := &option{}
				for _, fn := range opt {
					fn(o)
				}
				if o.withMaster {
					return nil // 使用主库后成功
				}
				return gorm.ErrRecordNotFound // 第一次失败
			},
			originalOpts: []Option{Debug()}, // 包含其他选项
			wantErr:      nil,
			wantRetries:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RetryOnNotFound(tt.fn, tt.originalOpts)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
