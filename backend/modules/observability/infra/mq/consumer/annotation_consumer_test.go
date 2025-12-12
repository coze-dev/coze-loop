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
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

// MockIAnnotationQueueConsumer 是 IAnnotationQueueConsumer 的 mock 实现
type MockIAnnotationQueueConsumer struct {
	ctrl     *gomock.Controller
	recorder *MockIAnnotationQueueConsumerMockRecorder
}

// MockIAnnotationQueueConsumerMockRecorder 是 mock 的记录器
type MockIAnnotationQueueConsumerMockRecorder struct {
	mock *MockIAnnotationQueueConsumer
}

// NewMockIAnnotationQueueConsumer 创建一个新的 mock 实例
func NewMockIAnnotationQueueConsumer(ctrl *gomock.Controller) *MockIAnnotationQueueConsumer {
	mock := &MockIAnnotationQueueConsumer{ctrl: ctrl}
	mock.recorder = &MockIAnnotationQueueConsumerMockRecorder{mock}
	return mock
}

// EXPECT 返回一个对象，允许调用者指示预期的使用
func (m *MockIAnnotationQueueConsumer) EXPECT() *MockIAnnotationQueueConsumerMockRecorder {
	return m.recorder
}

// Send mocks base method
func (m *MockIAnnotationQueueConsumer) Send(ctx context.Context, event *entity.AnnotationEvent) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Send", ctx, event)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send
func (mr *MockIAnnotationQueueConsumerMockRecorder) Send(ctx, event interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockIAnnotationQueueConsumer)(nil).Send), ctx, event)
}

func TestAnnotationConsumer_HandleMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfigLoader := mocks.NewMockIConfigLoader(ctrl)
	mockHandler := NewMockIAnnotationQueueConsumer(ctrl)
	consumer := NewAnnotationConsumer(mockHandler, mockConfigLoader)

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
						data, _ := json.Marshal(&entity.AnnotationEvent{})
						return data
					}(),
				},
				MsgID: "test_msg_id",
			},
			mockSetup: func() {
				mockHandler.EXPECT().Send(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "异常场景: 业务处理返回错误",
			message: &mq.MessageExt{
				Message: mq.Message{
					Body: func() []byte {
						data, _ := json.Marshal(&entity.AnnotationEvent{})
						return data
					}(),
				},
				MsgID: "test_msg_id",
			},
			mockSetup: func() {
				mockHandler.EXPECT().Send(gomock.Any(), gomock.Any()).
					Return(assert.AnError)
			},
			expectedError: assert.AnError,
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
