// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"context"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/api/handler/coze/loop/apis"
	"github.com/coze-dev/coze-loop/backend/infra/i18n"
)

func TestRootMw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupHandler  func() *apis.APIHandler
		expectedCount int
		wantPanic     bool
	}{
		{
			name: "成功配置根中间件",
			setupHandler: func() *apis.APIHandler {
				return &apis.APIHandler{
					Translater: &mockTranslater{},
				}
			},
			expectedCount: 4, // CtxCacheMW, AccessLogMW, LocaleMW, PacketAdapterMW
			wantPanic:     false,
		},
		{
			name: "Handler为nil时应该panic",
			setupHandler: func() *apis.APIHandler {
				return nil
			},
			expectedCount: 0,
			wantPanic:     true,
		},
		{
			name: "Translater为nil时仍能正常工作",
			setupHandler: func() *apis.APIHandler {
				return &apis.APIHandler{
					Translater: nil,
				}
			},
			expectedCount: 4,
			wantPanic:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantPanic {
				assert.Panics(t, func() {
					rootMw(tt.setupHandler())
				})
				return
			}

			handler := tt.setupHandler()
			middlewares := rootMw(handler)

			assert.NotNil(t, middlewares)
			assert.Equal(t, tt.expectedCount, len(middlewares))

			// 验证每个中间件都不为nil
			for i, mw := range middlewares {
				assert.NotNil(t, mw, "middleware at index %d should not be nil", i)
			}
		})
	}
}

func TestApiMw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupHandler  func() *apis.APIHandler
		expectedCount int
		wantPanic     bool
	}{
		{
			name: "成功配置API中间件",
			setupHandler: func() *apis.APIHandler {
				return &apis.APIHandler{}
			},
			expectedCount: 1, // SessionMW
			wantPanic:     false,
		},
		{
			name: "Handler为nil时返回中间件但使用时会出错",
			setupHandler: func() *apis.APIHandler {
				return nil
			},
			expectedCount: 1,
			wantPanic:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantPanic {
				assert.Panics(t, func() {
					_apiMw(tt.setupHandler())
				})
				return
			}

			handler := tt.setupHandler()
			middlewares := _apiMw(handler)

			assert.NotNil(t, middlewares)
			assert.Equal(t, tt.expectedCount, len(middlewares))

			// 验证每个中间件都不为nil
			for i, mw := range middlewares {
				assert.NotNil(t, mw, "middleware at index %d should not be nil", i)
			}
		})
	}
}

func TestLoopMw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupHandler  func() *apis.APIHandler
		expectedCount int
		wantPanic     bool
	}{
		{
			name: "成功配置Loop中间件",
			setupHandler: func() *apis.APIHandler {
				return &apis.APIHandler{}
			},
			expectedCount: 1, // PatTokenVerifyMW
			wantPanic:     false,
		},
		{
			name: "Handler为nil时返回中间件但使用时会出错",
			setupHandler: func() *apis.APIHandler {
				return nil
			},
			expectedCount: 1,
			wantPanic:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.wantPanic {
				assert.Panics(t, func() {
					_loopMw(tt.setupHandler())
				})
				return
			}

			handler := tt.setupHandler()
			middlewares := _loopMw(handler)

			assert.NotNil(t, middlewares)
			assert.Equal(t, tt.expectedCount, len(middlewares))

			// 验证每个中间件都不为nil
			for i, mw := range middlewares {
				assert.NotNil(t, mw, "middleware at index %d should not be nil", i)
			}
		})
	}
}

func TestAuthMw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupHandler   func() *apis.APIHandler
		expectedResult []app.HandlerFunc
	}{
		{
			name: "Auth中间件返回nil",
			setupHandler: func() *apis.APIHandler {
				return &apis.APIHandler{}
			},
			expectedResult: nil,
		},
		{
			name: "Handler为nil时Auth中间件仍返回nil",
			setupHandler: func() *apis.APIHandler {
				return nil
			},
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := tt.setupHandler()
			middlewares := _authMw(handler)

			assert.Equal(t, tt.expectedResult, middlewares)
		})
	}
}

func TestV1Mw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupHandler   func() *apis.APIHandler
		expectedResult []app.HandlerFunc
	}{
		{
			name: "V1中间件返回nil",
			setupHandler: func() *apis.APIHandler {
				return &apis.APIHandler{}
			},
			expectedResult: nil,
		},
		{
			name: "Handler为nil时V1中间件仍返回nil",
			setupHandler: func() *apis.APIHandler {
				return nil
			},
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := tt.setupHandler()
			middlewares := _v1Mw(handler)

			assert.Equal(t, tt.expectedResult, middlewares)
		})
	}
}

func TestMiddlewareIntegration(t *testing.T) {
	t.Parallel()

	t.Run("中间件链组合测试", func(t *testing.T) {
		t.Parallel()

		handler := &apis.APIHandler{
			Translater: &mockTranslater{},
		}

		// 测试所有中间件函数都能正常工作
		rootMiddlewares := rootMw(handler)
		apiMiddlewares := _apiMw(handler)
		loopMiddlewares := _loopMw(handler)
		authMiddlewares := _authMw(handler)
		v1Middlewares := _v1Mw(handler)

		// 验证根中间件
		require.NotNil(t, rootMiddlewares)
		assert.Equal(t, 4, len(rootMiddlewares))

		// 验证API中间件
		require.NotNil(t, apiMiddlewares)
		assert.Equal(t, 1, len(apiMiddlewares))

		// 验证Loop中间件
		require.NotNil(t, loopMiddlewares)
		assert.Equal(t, 1, len(loopMiddlewares))

		// 验证Auth中间件（应该为nil）
		assert.Nil(t, authMiddlewares)

		// 验证V1中间件（应该为nil）
		assert.Nil(t, v1Middlewares)
	})

	t.Run("中间件函数返回值类型验证", func(t *testing.T) {
		t.Parallel()

		handler := &apis.APIHandler{
			Translater: &mockTranslater{},
		}

		middlewares := rootMw(handler)
		for i, mw := range middlewares {
			assert.IsType(t, app.HandlerFunc(nil), mw, "middleware at index %d should be app.HandlerFunc type", i)
		}
	})
}

func TestMiddlewareParameterValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		middlewareFunc func(*apis.APIHandler) []app.HandlerFunc
		funcName       string
		shouldPanic    bool
	}{
		{
			name:           "rootMw with nil handler",
			middlewareFunc: rootMw,
			funcName:       "rootMw",
			shouldPanic:    true,
		},
		{
			name:           "_apiMw with nil handler",
			middlewareFunc: _apiMw,
			funcName:       "_apiMw",
			shouldPanic:    false,
		},
		{
			name:           "_loopMw with nil handler",
			middlewareFunc: _loopMw,
			funcName:       "_loopMw",
			shouldPanic:    false,
		},
		{
			name:           "_authMw with nil handler",
			middlewareFunc: _authMw,
			funcName:       "_authMw",
			shouldPanic:    false, // 这个函数不依赖handler，所以不会panic
		},
		{
			name:           "_v1Mw with nil handler",
			middlewareFunc: _v1Mw,
			funcName:       "_v1Mw",
			shouldPanic:    false, // 这个函数不依赖handler，所以不会panic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.shouldPanic {
				assert.Panics(t, func() {
					tt.middlewareFunc(nil)
				}, "%s should panic with nil handler", tt.funcName)
			} else {
				assert.NotPanics(t, func() {
					result := tt.middlewareFunc(nil)
					// 对于_authMw和_v1Mw，期望返回nil
					// 对于其他中间件，期望返回非nil的中间件数组
					switch tt.funcName {
					case "_authMw", "_v1Mw":
						assert.Nil(t, result, "%s should return nil", tt.funcName)
					case "rootMw":
						// rootMw需要特殊处理，因为它会panic
						// 这里不做额外检查
					default:
						assert.NotNil(t, result, "%s should return non-nil middleware array", tt.funcName)
					}
				}, "%s should not panic with nil handler", tt.funcName)
			}
		})
	}
}

// mockTranslater 是 i18n.ITranslater 的模拟实现
type mockTranslater struct{}

func (m *mockTranslater) Translate(ctx context.Context, key string, lang string) (string, error) {
	return key, nil // 简单返回key作为翻译结果
}

func (m *mockTranslater) MustTranslate(ctx context.Context, key string, lang string) string {
	return key // 简单返回key作为翻译结果
}

// 验证mockTranslater实现了i18n.ITranslater接口
var _ i18n.ITranslater = (*mockTranslater)(nil)

// TestMiddlewareWithMockDependencies 测试中间件的依赖注入
func TestMiddlewareWithMockDependencies(t *testing.T) {
	t.Parallel()

	t.Run("测试rootMw中的PacketAdapterMW依赖", func(t *testing.T) {
		t.Parallel()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// 创建带有mock translater的handler
		handler := &apis.APIHandler{
			Translater: &mockTranslater{},
		}

		middlewares := rootMw(handler)

		// 验证中间件数量和类型
		assert.Equal(t, 4, len(middlewares))

		// 验证所有中间件都是有效的函数
		for i, mw := range middlewares {
			assert.NotNil(t, mw, "middleware %d should not be nil", i)
			assert.IsType(t, app.HandlerFunc(nil), mw)
		}
	})

	t.Run("测试_apiMw中的SessionMW依赖", func(t *testing.T) {
		t.Parallel()

		handler := &apis.APIHandler{}
		middlewares := _apiMw(handler)

		// 验证SessionMW被正确创建
		assert.Equal(t, 1, len(middlewares))
		assert.NotNil(t, middlewares[0])
		assert.IsType(t, app.HandlerFunc(nil), middlewares[0])
	})

	t.Run("测试_loopMw中的PatTokenVerifyMW依赖", func(t *testing.T) {
		t.Parallel()

		handler := &apis.APIHandler{}
		middlewares := _loopMw(handler)

		// 验证PatTokenVerifyMW被正确创建
		assert.Equal(t, 1, len(middlewares))
		assert.NotNil(t, middlewares[0])
		assert.IsType(t, app.HandlerFunc(nil), middlewares[0])
	})
}

// TestMiddlewareErrorHandling 测试中间件的错误处理
func TestMiddlewareErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("处理Translater为nil的情况", func(t *testing.T) {
		t.Parallel()

		handler := &apis.APIHandler{
			Translater: nil, // 故意设置为nil
		}

		// rootMw应该仍然能够工作，即使Translater为nil
		assert.NotPanics(t, func() {
			middlewares := rootMw(handler)
			assert.Equal(t, 4, len(middlewares))
		})
	})

	t.Run("处理空Handler结构体", func(t *testing.T) {
		t.Parallel()

		handler := &apis.APIHandler{} // 空的handler

		// 所有中间件函数都应该能处理空的handler
		assert.NotPanics(t, func() {
			rootMw(handler)
			_apiMw(handler)
			_loopMw(handler)
			_authMw(handler)
			_v1Mw(handler)
		})
	})
}

// TestMiddlewareConsistency 测试中间件的一致性
func TestMiddlewareConsistency(t *testing.T) {
	t.Parallel()

	t.Run("多次调用同一中间件函数应该返回相同结构", func(t *testing.T) {
		t.Parallel()

		handler := &apis.APIHandler{
			Translater: &mockTranslater{},
		}

		// 多次调用rootMw
		mw1 := rootMw(handler)
		mw2 := rootMw(handler)

		assert.Equal(t, len(mw1), len(mw2))

		// 验证两次调用返回的中间件数量相同
		assert.Equal(t, 4, len(mw1))
		assert.Equal(t, 4, len(mw2))
	})

	t.Run("不同handler实例应该返回相同结构的中间件", func(t *testing.T) {
		t.Parallel()

		handler1 := &apis.APIHandler{Translater: &mockTranslater{}}
		handler2 := &apis.APIHandler{Translater: &mockTranslater{}}

		mw1 := rootMw(handler1)
		mw2 := rootMw(handler2)

		assert.Equal(t, len(mw1), len(mw2))
		assert.Equal(t, 4, len(mw1))
		assert.Equal(t, 4, len(mw2))
	})
}
