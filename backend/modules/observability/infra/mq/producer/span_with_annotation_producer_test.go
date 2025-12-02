// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/infra/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestSpanWithAnnotationProducerImpl_SendSpanWithAnnotation(t *testing.T) {
	type fields struct {
		topic      string
		mqProducer mq.IProducer
	}
	type args struct {
		ctx     context.Context
		message *entity.SpanEvent
		tag     string
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		mockSetup     func(mqProducer *mocks.MockIProducer)
		expectedError error
	}{
		{
			name: "正常场景: 发送span annotation消息成功",
			fields: fields{
				topic: "test_topic",
			},
			args: args{
				ctx: context.Background(),
				message: &entity.SpanEvent{
					Span: &loop_span.Span{
						TraceID: "test_trace_id",
						SpanID:  "test_span_id",
					},
				},
				tag: "test_tag",
			},
			mockSetup: func(mqProducer *mocks.MockIProducer) {
				mqProducer.EXPECT().Send(gomock.Any(), gomock.Any()).Return("msg_id", nil)
			},
			expectedError: nil,
		},
		{
			name: "异常场景: JSON序列化失败",
			fields: fields{
				topic: "test_topic",
			},
			args: args{
				ctx:     context.Background(),
				message: nil, // 这将导致JSON序列化失败
				tag:     "test_tag",
			},
			mockSetup:     func(mqProducer *mocks.MockIProducer) {},
			expectedError: errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode),
		},
		{
			name: "异常场景: MQ发送失败",
			fields: fields{
				topic: "test_topic",
			},
			args: args{
				ctx: context.Background(),
				message: &entity.SpanEvent{
					Span: &loop_span.Span{
						TraceID: "test_trace_id",
						SpanID:  "test_span_id",
					},
				},
				tag: "test_tag",
			},
			mockSetup: func(mqProducer *mocks.MockIProducer) {
				mqProducer.EXPECT().Send(gomock.Any(), gomock.Any()).Return("", assert.AnError)
			},
			expectedError: errorx.NewByCode(obErrorx.CommercialCommonRPCErrorCodeCode),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mqProducerMock := mocks.NewMockIProducer(ctrl)
			tt.mockSetup(mqProducerMock)

			producer := &SpanWithAnnotationProducerImpl{
				topic:      tt.fields.topic,
				mqProducer: mqProducerMock,
			}

			err := producer.SendSpanWithAnnotation(tt.args.ctx, tt.args.message, tt.args.tag)
			
			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewSpanWithAnnotationProducerImpl(t *testing.T) {
	type args struct {
		traceConfig config.ITraceConfig
		mqFactory   mq.IFactory
	}
	tests := []struct {
		name          string
		args          args
		mockSetup     func(traceConfig *confmocks.MockITraceConfig, mqFactory *mocks.MockIFactory)
		expectedError bool
	}{
		{
			name: "正常场景: 创建producer成功",
			args: args{
				traceConfig: nil, // 将在mockSetup中设置
				mqFactory:   nil, // 将在mockSetup中设置
			},
			mockSetup: func(traceConfig *confmocks.MockITraceConfig, mqFactory *mocks.MockIFactory) {
				mqCfg := &config.MqProducerCfg{
					Topic:         "test_topic",
					Addr:          []string{"localhost:9876"},
					Timeout:       5000,
					RetryTimes:    3,
					ProducerGroup: "test_group",
				}
				traceConfig.EXPECT().GetSpanWithAnnotationMqProducerCfg(gomock.Any()).Return(mqCfg, nil)
				
				producerMock := mocks.NewMockIProducer(gomock.NewController(t))
				producerMock.EXPECT().Start().Return(nil)
				
				mqFactory.EXPECT().NewProducer(gomock.Any()).Return(producerMock, nil)
			},
			expectedError: false,
		},
		{
			name: "异常场景: 获取配置失败",
			args: args{
				traceConfig: nil,
				mqFactory:   nil,
			},
			mockSetup: func(traceConfig *confmocks.MockITraceConfig, mqFactory *mocks.MockIFactory) {
				traceConfig.EXPECT().GetSpanWithAnnotationMqProducerCfg(gomock.Any()).Return(nil, assert.AnError)
			},
			expectedError: true,
		},
		{
			name: "异常场景: topic为空",
			args: args{
				traceConfig: nil,
				mqFactory:   nil,
			},
			mockSetup: func(traceConfig *confmocks.MockITraceConfig, mqFactory *mocks.MockIFactory) {
				mqCfg := &config.MqProducerCfg{
					Topic:         "", // 空topic
					Addr:          []string{"localhost:9876"},
					Timeout:       5000,
					RetryTimes:    3,
					ProducerGroup: "test_group",
				}
				traceConfig.EXPECT().GetSpanWithAnnotationMqProducerCfg(gomock.Any()).Return(mqCfg, nil)
			},
			expectedError: true,
		},
		{
			name: "异常场景: 创建MQ producer失败",
			args: args{
				traceConfig: nil,
				mqFactory:   nil,
			},
			mockSetup: func(traceConfig *confmocks.MockITraceConfig, mqFactory *mocks.MockIFactory) {
				mqCfg := &config.MqProducerCfg{
					Topic:         "test_topic",
					Addr:          []string{"localhost:9876"},
					Timeout:       5000,
					RetryTimes:    3,
					ProducerGroup: "test_group",
				}
				traceConfig.EXPECT().GetSpanWithAnnotationMqProducerCfg(gomock.Any()).Return(mqCfg, nil)
				mqFactory.EXPECT().NewProducer(gomock.Any()).Return(nil, assert.AnError)
			},
			expectedError: true,
		},
		{
			name: "异常场景: 启动producer失败",
			args: args{
				traceConfig: nil,
				mqFactory:   nil,
			},
			mockSetup: func(traceConfig *confmocks.MockITraceConfig, mqFactory *mocks.MockIFactory) {
				mqCfg := &config.MqProducerCfg{
					Topic:         "test_topic",
					Addr:          []string{"localhost:9876"},
					Timeout:       5000,
					RetryTimes:    3,
					ProducerGroup: "test_group",
				}
				traceConfig.EXPECT().GetSpanWithAnnotationMqProducerCfg(gomock.Any()).Return(mqCfg, nil)
				
				producerMock := mocks.NewMockIProducer(gomock.NewController(t))
				producerMock.EXPECT().Start().Return(assert.AnError)
				
				mqFactory.EXPECT().NewProducer(gomock.Any()).Return(producerMock, nil)
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			
			traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
			mqFactoryMock := mocks.NewMockIFactory(ctrl)
			tt.mockSetup(traceConfigMock, mqFactoryMock)

			// 重置单例
			singletonSpanWithAnnotationProducer = nil
			spanWithAnnotationProducerOnce = sync.Once{}

			producer, err := NewSpanWithAnnotationProducerImpl(traceConfigMock, mqFactoryMock)
			
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, producer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, producer)
			}
		})
	}
}

func TestSpanWithAnnotationProducerImpl_Singleton(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
	mqFactoryMock := mocks.NewMockIFactory(ctrl)

	mqCfg := &config.MqProducerCfg{
		Topic:         "test_topic",
		Addr:          []string{"localhost:9876"},
		Timeout:       5000,
		RetryTimes:    3,
		ProducerGroup: "test_group",
	}
	traceConfigMock.EXPECT().GetSpanWithAnnotationMqProducerCfg(gomock.Any()).Return(mqCfg, nil).Times(2)
	
	producerMock := mocks.NewMockIProducer(ctrl)
	producerMock.EXPECT().Start().Return(nil).Times(2)
	
	mqFactoryMock.EXPECT().NewProducer(gomock.Any()).Return(producerMock, nil).Times(2)

	// 重置单例
	singletonSpanWithAnnotationProducer = nil
	spanWithAnnotationProducerOnce = sync.Once{}

	// 第一次调用
	producer1, err1 := NewSpanWithAnnotationProducerImpl(traceConfigMock, mqFactoryMock)
	assert.NoError(t, err1)
	assert.NotNil(t, producer1)

	// 第二次调用，应该返回同一个实例
	producer2, err2 := NewSpanWithAnnotationProducerImpl(traceConfigMock, mqFactoryMock)
	assert.NoError(t, err2)
	assert.NotNil(t, producer2)
	assert.Equal(t, producer1, producer2)
}