// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	limitermocks "github.com/coze-dev/coze-loop/backend/infra/limiter/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestOpenAPIApplication_AllowByKeyWithScene(t *testing.T) {
	const key = "123"

	tests := []struct {
		name        string
		reqScene    loop_span.TraceScene
		setup       func(cfg *configmocks.MockITraceConfig, rl *limitermocks.MockIRateLimiter)
		wantAllowed bool
		wantScene   loop_span.TraceScene
	}{
		{
			name:     "non-cached scene -> default bucket, default scene",
			reqScene: loop_span.TraceSceneDefault,
			setup: func(cfg *configmocks.MockITraceConfig, rl *limitermocks.MockIRateLimiter) {
				cfg.EXPECT().GetQueryMaxQPS(gomock.Any(), key).Return(10, nil)
				rl.EXPECT().AllowN(gomock.Any(), key, 1, gomock.Any()).
					Return(&limiter.Result{Allowed: true}, nil)
			},
			wantAllowed: true,
			wantScene:   loop_span.TraceSceneDefault,
		},
		{
			name:     "cached configured (>0) -> cached bucket, cached scene",
			reqScene: loop_span.TraceSceneCached,
			setup: func(cfg *configmocks.MockITraceConfig, rl *limitermocks.MockIRateLimiter) {
				cfg.EXPECT().GetCachedQueryMaxQPS(gomock.Any(), key).Return(50, nil)
				rl.EXPECT().AllowN(gomock.Any(), "cached:"+key, 1, gomock.Any()).
					Return(&limiter.Result{Allowed: true}, nil)
			},
			wantAllowed: true,
			wantScene:   loop_span.TraceSceneCached,
		},
		{
			name:     "cached bucket full -> not allowed",
			reqScene: loop_span.TraceSceneCached,
			setup: func(cfg *configmocks.MockITraceConfig, rl *limitermocks.MockIRateLimiter) {
				cfg.EXPECT().GetCachedQueryMaxQPS(gomock.Any(), key).Return(50, nil)
				rl.EXPECT().AllowN(gomock.Any(), "cached:"+key, 1, gomock.Any()).
					Return(&limiter.Result{Allowed: false}, nil)
			},
			wantAllowed: false,
			wantScene:   loop_span.TraceSceneCached,
		},
		{
			name:     "cached but qps<=0 -> degrade to default bucket, default scene",
			reqScene: loop_span.TraceSceneCached,
			setup: func(cfg *configmocks.MockITraceConfig, rl *limitermocks.MockIRateLimiter) {
				cfg.EXPECT().GetCachedQueryMaxQPS(gomock.Any(), key).Return(0, nil)
				cfg.EXPECT().GetQueryMaxQPS(gomock.Any(), key).Return(10, nil)
				rl.EXPECT().AllowN(gomock.Any(), key, 1, gomock.Any()).
					Return(&limiter.Result{Allowed: true}, nil)
			},
			wantAllowed: true,
			wantScene:   loop_span.TraceSceneDefault,
		},
		{
			name:     "cached but cfg read errors -> degrade to default bucket, default scene",
			reqScene: loop_span.TraceSceneCached,
			setup: func(cfg *configmocks.MockITraceConfig, rl *limitermocks.MockIRateLimiter) {
				cfg.EXPECT().GetCachedQueryMaxQPS(gomock.Any(), key).Return(0, errors.New("cfg boom"))
				cfg.EXPECT().GetQueryMaxQPS(gomock.Any(), key).Return(10, nil)
				rl.EXPECT().AllowN(gomock.Any(), key, 1, gomock.Any()).
					Return(&limiter.Result{Allowed: true}, nil)
			},
			wantAllowed: true,
			wantScene:   loop_span.TraceSceneDefault,
		},
		{
			name:     "default qps read errors -> fail-open, default scene",
			reqScene: loop_span.TraceSceneDefault,
			setup: func(cfg *configmocks.MockITraceConfig, rl *limitermocks.MockIRateLimiter) {
				cfg.EXPECT().GetQueryMaxQPS(gomock.Any(), key).Return(0, errors.New("cfg boom"))
			},
			wantAllowed: true,
			wantScene:   loop_span.TraceSceneDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cfgMock := configmocks.NewMockITraceConfig(ctrl)
			rlMock := limitermocks.NewMockIRateLimiter(ctrl)
			tt.setup(cfgMock, rlMock)

			o := &OpenAPIApplication{traceConfig: cfgMock, rateLimiter: rlMock}
			allowed, scene := o.AllowByKeyWithScene(context.Background(), key, tt.reqScene)
			assert.Equal(t, tt.wantAllowed, allowed)
			assert.Equal(t, tt.wantScene, scene)
		})
	}
}
