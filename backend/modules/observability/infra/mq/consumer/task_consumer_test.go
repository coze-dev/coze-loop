// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

// MockITaskQueueConsumer 是 ITaskQueueConsumer 的 mock 实现
type MockITaskQueueConsumer struct {
	ctrl     *gomock.Controller
	recorder *MockITaskQueueConsumerMockRecorder
}

// MockITaskQueueConsumerMockRecorder 是 mock 的记录器
type MockITaskQueueConsumerMockRecorder struct {
	mock *MockITaskQueueConsumer
}

// NewMockITaskQueueConsumer 创建一个新的 mock 实例
func NewMockITaskQueueConsumer(ctrl *gomock.Controller) *MockITaskQueueConsumer {
	mock := &MockITaskQueueConsumer{ctrl: ctrl}
	mock.recorder = &MockITaskQueueConsumerMockRecorder{mock}
	return mock
}

// EXPECT 返回一个对象，允许调用者指示预期的使用
func (m *MockITaskQueueConsumer) EXPECT() *MockITaskQueueConsumerMockRecorder {
	return m.recorder
}

// SpanTrigger mocks base method
func (m *MockITaskQueueConsumer) SpanTrigger(ctx context.Context, rawSpan *entity.RawSpan, span *loop_span.Span) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SpanTrigger", ctx, rawSpan, span)
	ret0, _ := ret[0].(error)
	return ret0
}

// SpanTrigger indicates an expected call of SpanTrigger
func (mr *MockITaskQueueConsumerMockRecorder) SpanTrigger(ctx, rawSpan, span interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SpanTrigger", reflect.TypeOf((*MockITaskQueueConsumer)(nil).SpanTrigger), ctx, rawSpan, span)
}

// AutoEvalCallback mocks base method
func (m *MockITaskQueueConsumer) AutoEvalCallback(ctx context.Context, event *entity.AutoEvalEvent) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AutoEvalCallback", ctx, event)
	ret0, _ := ret[0].(error)
	return ret0
}

// AutoEvalCallback indicates an expected call of AutoEvalCallback
func (mr *MockITaskQueueConsumerMockRecorder) AutoEvalCallback(ctx, event interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AutoEvalCallback", reflect.TypeOf((*MockITaskQueueConsumer)(nil).AutoEvalCallback), ctx, event)
}

// AutoEvalCorrection mocks base method
func (m *MockITaskQueueConsumer) AutoEvalCorrection(ctx context.Context, event *entity.CorrectionEvent) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AutoEvalCorrection", ctx, event)
	ret0, _ := ret[0].(error)
	return ret0
}

// AutoEvalCorrection indicates an expected call of AutoEvalCorrection
func (mr *MockITaskQueueConsumerMockRecorder) AutoEvalCorrection(ctx, event interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AutoEvalCorrection", reflect.TypeOf((*MockITaskQueueConsumer)(nil).AutoEvalCorrection), ctx, event)
}

// BackFill mocks base method
func (m *MockITaskQueueConsumer) BackFill(ctx context.Context, event *entity.BackFillEvent) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BackFill", ctx, event)
	ret0, _ := ret[0].(error)
	return ret0
}

// BackFill indicates an expected call of BackFill
func (mr *MockITaskQueueConsumerMockRecorder) BackFill(ctx, event interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BackFill", reflect.TypeOf((*MockITaskQueueConsumer)(nil).BackFill), ctx, event)
}

func TestTaskConsumer_ConsumerCfg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfigLoader := mocks.NewMockIConfigLoader(ctrl)
	mockHandler := NewMockITaskQueueConsumer(ctrl)
	consumer := NewTaskConsumer(mockHandler, mockConfigLoader)

	tests := []struct {
		name          string
		mockSetup     func()
		expectedCfg   *mq.ConsumerConfig
		expectedError error
	}{
		{
			name: "正常场景: 配置加载成功",
			mockSetup: func() {
				enablePPE := true
				isEnabled := true
				cfg := &config.MqConsumerCfg{
					Addr:          []string{"localhost:9876"},
					Topic:         "test_topic",
					ConsumerGroup: "test_group",
					Timeout:       5000,
					WorkerNum:     10,
					EnablePPE:     &enablePPE,
					IsEnabled:     &isEnabled,
					TagExpression: nil,
				}
				mockConfigLoader.EXPECT().UnmarshalKey(gomock.Any(), "task_mq_consumer_config", gomock.Any()).
					SetArg(2, *cfg).Return(nil)
			},
			expectedCfg: func() *mq.ConsumerConfig {
				enablePPE := true
				isEnabled := true
				return &mq.ConsumerConfig{
					Addr:                 []string{"localhost:9876"},
					Topic:                "test_topic",
					ConsumerGroup:        "test_group",
					ConsumeTimeout:       5000000000, // 5秒转换为纳秒
					ConsumeGoroutineNums: 10,
					EnablePPE:            &enablePPE,
					IsEnabled:            &isEnabled,
				}
			}(),
			expectedError: nil,
		},
		{
			name: "正常场景: 配置加载成功且包含TagExpression",
			mockSetup: func() {
				tagExpr := "tagA || tagB"
				enablePPE := false
				isEnabled := true
				cfg := &config.MqConsumerCfg{
					Addr:          []string{"localhost:9876"},
					Topic:         "test_topic",
					ConsumerGroup: "test_group",
					Timeout:       3000,
					WorkerNum:     5,
					EnablePPE:     &enablePPE,
					IsEnabled:     &isEnabled,
					TagExpression: &tagExpr,
				}
				mockConfigLoader.EXPECT().UnmarshalKey(gomock.Any(), "task_mq_consumer_config", gomock.Any()).
					SetArg(2, *cfg).Return(nil)
			},
			expectedCfg: func() *mq.ConsumerConfig {
				enablePPE := false
				isEnabled := true
				return &mq.ConsumerConfig{
					Addr:                 []string{"localhost:9876"},
					Topic:                "test_topic",
					ConsumerGroup:        "test_group",
					ConsumeTimeout:       3000000000, // 3秒转换为纳秒
					ConsumeGoroutineNums: 5,
					EnablePPE:            &enablePPE,
					IsEnabled:            &isEnabled,
					TagExpression:        "tagA || tagB",
				}
			}(),
			expectedError: nil,
		},
		{
			name: "异常场景: 配置加载失败",
			mockSetup: func() {
				mockConfigLoader.EXPECT().UnmarshalKey(gomock.Any(), "task_mq_consumer_config", gomock.Any()).
					Return(assert.AnError)
			},
			expectedCfg:   nil,
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			cfg, err := consumer.ConsumerCfg(context.Background())

			assert.Equal(t, tt.expectedError, err)
			if tt.expectedError == nil {
				assert.NotNil(t, cfg)
				assert.Equal(t, tt.expectedCfg.Addr, cfg.Addr)
				assert.Equal(t, tt.expectedCfg.Topic, cfg.Topic)
				assert.Equal(t, tt.expectedCfg.ConsumerGroup, cfg.ConsumerGroup)
				assert.Equal(t, tt.expectedCfg.ConsumeTimeout, cfg.ConsumeTimeout)
				assert.Equal(t, tt.expectedCfg.ConsumeGoroutineNums, cfg.ConsumeGoroutineNums)
				assert.Equal(t, tt.expectedCfg.EnablePPE, cfg.EnablePPE)
				assert.Equal(t, tt.expectedCfg.IsEnabled, cfg.IsEnabled)
				assert.Equal(t, tt.expectedCfg.TagExpression, cfg.TagExpression)
			}
		})
	}
}

func TestTaskConsumer_HandleMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfigLoader := mocks.NewMockIConfigLoader(ctrl)
	mockHandler := NewMockITaskQueueConsumer(ctrl)
	consumer := NewTaskConsumer(mockHandler, mockConfigLoader)

	tests := []struct {
		name          string
		message       *mq.MessageExt
		mockSetup     func()
		expectedError error
	}{
		{
			name: "正常场景: 消息处理成功",
			message: &mq.MessageExt{
				Message: mq.Message{
					Body: func() []byte {
						data, _ := json.Marshal(&entity.RawSpan{
							LogID:  "test_log_id",
							TraceID: "test_trace_id",
							SpanID:  "test_span_id",
						})
						return data
					}(),
				},
				MsgID: "test_msg_id",
			},
			mockSetup: func() {
				mockHandler.EXPECT().SpanTrigger(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "正常场景: JSON解析失败但不返回错误",
			message: &mq.MessageExt{
				Message: mq.Message{
					Body: []byte("invalid json"),
				},
				MsgID: "test_msg_id",
			},
			mockSetup:     func() {},
			expectedError: nil,
		},
		{
			name: "正常场景: 处理空消息",
			message: &mq.MessageExt{
				Message: mq.Message{
					Body: []byte(""),
				},
				MsgID: "test_msg_id",
			},
			mockSetup:     func() {},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := consumer.HandleMessage(context.Background(), tt.message)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}