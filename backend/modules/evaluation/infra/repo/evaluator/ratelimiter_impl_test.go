// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	limiterMocks "github.com/coze-dev/coze-loop/backend/infra/limiter/mocks"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestRateLimiterImpl_AllowInvoke(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLimiter := limiterMocks.NewMockIRateLimiter(ctrl)

	tests := []struct {
		name           string
		spaceID        int64
		mockSetup      func()
		expectedResult bool
	}{
		{
			name:    "允许调用",
			spaceID: 1,
			mockSetup: func() {
				mockLimiter.EXPECT().
					AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&limiter.Result{
						Allowed: true,
					}, nil)
			},
			expectedResult: true,
		},
		{
			name:    "不允许调用",
			spaceID: 1,
			mockSetup: func() {
				mockLimiter.EXPECT().
					AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&limiter.Result{
						Allowed: false,
					}, nil)
			},
			expectedResult: false,
		},
		{
			name:    "限流器错误时默认允许调用",
			spaceID: 1,
			mockSetup: func() {
				mockLimiter.EXPECT().
					AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			limiter := &RateLimiterImpl{
				limiter: mockLimiter,
			}

			result := limiter.AllowInvoke(context.Background(), tt.spaceID)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestPlainRateLimiterImpl_AllowInvokeWithKeyLimit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPlainLimiter := limiterMocks.NewMockIPlainRateLimiter(ctrl)

	tests := []struct {
		name           string
		key            string
		limit          *commonentity.RateLimit
		mockSetup      func()
		expectedResult bool
	}{
		{
			name: "key为空，返回false",
			key:  "",
			limit: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(10)),
				Burst:  gptr.Of(int32(10)),
				Period: gptr.Of(time.Duration(60)),
			},
			mockSetup:      func() {},
			expectedResult: false,
		},
		{
			name:  "limit为nil，返回true",
			key:   "test_key",
			limit: nil,
			mockSetup: func() {
				// 不需要调用mock，因为limit为nil直接返回true
			},
			expectedResult: true,
		},
		{
			name: "limit.Period为nil，返回true",
			key:  "test_key",
			limit: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(10)),
				Burst:  gptr.Of(int32(10)),
				Period: nil,
			},
			mockSetup:      func() {},
			expectedResult: true,
		},
		{
			name: "limit.Rate为nil，返回true",
			key:  "test_key",
			limit: &commonentity.RateLimit{
				Rate:   nil,
				Burst:  gptr.Of(int32(10)),
				Period: gptr.Of(time.Duration(60)),
			},
			mockSetup:      func() {},
			expectedResult: true,
		},
		{
			name: "允许调用",
			key:  "test_key",
			limit: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(10)),
				Burst:  gptr.Of(int32(10)),
				Period: gptr.Of(time.Duration(60)),
			},
			mockSetup: func() {
				mockPlainLimiter.EXPECT().
					AllowN(gomock.Any(), "test_key", 1, gomock.Any()).
					Return(&limiter.Result{
						Allowed: true,
					}, nil)
			},
			expectedResult: true,
		},
		{
			name: "不允许调用",
			key:  "test_key",
			limit: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(10)),
				Burst:  gptr.Of(int32(10)),
				Period: gptr.Of(time.Duration(60)),
			},
			mockSetup: func() {
				mockPlainLimiter.EXPECT().
					AllowN(gomock.Any(), "test_key", 1, gomock.Any()).
					Return(&limiter.Result{
						Allowed: false,
					}, nil)
			},
			expectedResult: false,
		},
		{
			name: "限流器错误时默认允许调用",
			key:  "test_key",
			limit: &commonentity.RateLimit{
				Rate:   gptr.Of(int32(10)),
				Burst:  gptr.Of(int32(10)),
				Period: gptr.Of(time.Duration(60)),
			},
			mockSetup: func() {
				mockPlainLimiter.EXPECT().
					AllowN(gomock.Any(), "test_key", 1, gomock.Any()).
					Return(nil, assert.AnError)
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			limiter := &PlainRateLimiterImpl{
				limiter: mockPlainLimiter,
			}

			result := limiter.AllowInvokeWithKeyLimit(context.Background(), tt.key, tt.limit)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
