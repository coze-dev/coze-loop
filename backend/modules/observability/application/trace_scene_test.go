// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// cachedCfg 构造一个命中/未命中 workspace 的 TraceSceneCfg。
func cachedCfg(defaultEnabled bool, overrides map[int64]bool) *config.TraceSceneCfg {
	return &config.TraceSceneCfg{
		CachedEnabled: config.SpaceAwareParam[bool]{
			Default:   defaultEnabled,
			Overrides: overrides,
		},
	}
}

func TestResolveTraceScene(t *testing.T) {
	const wsEnabled = int64(111)
	const wsDisabled = int64(222)

	tests := []struct {
		name      string
		scene     string
		workspace int64
		setup     func(m *configmocks.MockITraceConfig)
		want      loop_span.TraceScene
	}{
		{
			name:      "empty scene degrades to default without reading cfg",
			scene:     "",
			workspace: wsEnabled,
			setup:     func(m *configmocks.MockITraceConfig) {}, // GetTraceSceneCfg 不应被调用
			want:      loop_span.TraceSceneDefault,
		},
		{
			name:      "explicit default scene degrades to default without reading cfg",
			scene:     string(loop_span.TraceSceneDefault),
			workspace: wsEnabled,
			setup:     func(m *configmocks.MockITraceConfig) {},
			want:      loop_span.TraceSceneDefault,
		},
		{
			name:      "cached requested and workspace enabled -> cached",
			scene:     string(loop_span.TraceSceneCached),
			workspace: wsEnabled,
			setup: func(m *configmocks.MockITraceConfig) {
				m.EXPECT().GetTraceSceneCfg(gomock.Any()).
					Return(cachedCfg(false, map[int64]bool{wsEnabled: true}), nil)
			},
			want: loop_span.TraceSceneCached,
		},
		{
			name:      "cached requested but workspace not enabled -> degrade to default",
			scene:     string(loop_span.TraceSceneCached),
			workspace: wsDisabled,
			setup: func(m *configmocks.MockITraceConfig) {
				m.EXPECT().GetTraceSceneCfg(gomock.Any()).
					Return(cachedCfg(false, map[int64]bool{wsEnabled: true}), nil)
			},
			want: loop_span.TraceSceneDefault,
		},
		{
			name:      "cached requested but cfg read errors -> degrade to default",
			scene:     string(loop_span.TraceSceneCached),
			workspace: wsEnabled,
			setup: func(m *configmocks.MockITraceConfig) {
				m.EXPECT().GetTraceSceneCfg(gomock.Any()).
					Return(nil, errors.New("cfg boom"))
			},
			want: loop_span.TraceSceneDefault,
		},
		{
			name:      "cached requested but cfg nil -> degrade to default",
			scene:     string(loop_span.TraceSceneCached),
			workspace: wsEnabled,
			setup: func(m *configmocks.MockITraceConfig) {
				m.EXPECT().GetTraceSceneCfg(gomock.Any()).Return(nil, nil)
			},
			want: loop_span.TraceSceneDefault,
		},
		{
			name:      "cached requested and enabled via default -> cached",
			scene:     string(loop_span.TraceSceneCached),
			workspace: wsDisabled,
			setup: func(m *configmocks.MockITraceConfig) {
				m.EXPECT().GetTraceSceneCfg(gomock.Any()).
					Return(cachedCfg(true, nil), nil)
			},
			want: loop_span.TraceSceneCached,
		},
	}

	for _, tt := range tests {
		t.Run("openapi/"+tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cfgMock := configmocks.NewMockITraceConfig(ctrl)
			tt.setup(cfgMock)
			o := &OpenAPIApplication{traceConfig: cfgMock}
			got := o.resolveTraceScene(context.Background(), tt.workspace, tt.scene)
			assert.Equal(t, tt.want, got)
		})
		t.Run("trace/"+tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cfgMock := configmocks.NewMockITraceConfig(ctrl)
			tt.setup(cfgMock)
			tr := &TraceApplication{traceConfig: cfgMock}
			got := tr.resolveTraceScene(context.Background(), tt.workspace, tt.scene)
			assert.Equal(t, tt.want, got)
		})
	}
}
