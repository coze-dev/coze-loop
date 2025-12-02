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

func TestTraceProducerImpl_IngestSpans(t *testing.T) {
	type fields struct {
		producerProxy map[string]*producerProxy
	}
	type args struct {
		ctx context.Context
		td  *entity.TraceData
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		mockSetup     func(mqProducer *mocks.MockIProducer)
		expectedError error
	}{
		{
			name: "正常场景: 成功发送trace数据",
			fields: fields{
				producerProxy: map[string]*producerProxy{
					"test_tenant": {
						traceTopic: "test_topic",
						mqProducer: nil, // 将在mockSetup中设置
					},
				},
			},
			args: args{
				ctx: context.Background(),
				td: &entity.TraceData{
					Tenant: "test_tenant",
					SpanList: []*loop_span.Span{
						{
							TraceID: "test_trace_id",
							SpanID:  "test_span_id",
						},
					},
				},
			},
			mockSetup: func(mqProducer *mocks.MockIProducer) {
				mqProducer.EXPECT().SendAsync(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "异常场景: tenant producer不存在",
			fields: fields{
				producerProxy: map[string]*producerProxy{
					"other_tenant": {
						traceTopic: "test_topic",
						mqProducer: nil,
					},
				},
			},
			args: args{
				ctx: context.Background(),
				td: &entity.TraceData{
					Tenant: "test_tenant",
					SpanList: []*loop_span.Span{
						{
							TraceID: "test_trace_id",
							SpanID:  "test_span_id",
						},
					},
				},
			},
			mockSetup:     func(mqProducer *mocks.MockIProducer) {},
			expectedError: errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode, errorx.WithExtraMsg("tenant producer not exist")),
		},
		{
			name: "异常场景: JSON序列化失败",
			fields: fields{
				producerProxy: map[string]*producerProxy{
					"test_tenant": {
						traceTopic: "test_topic",
						mqProducer: nil,
					},
				},
			},
			args: args{
				ctx: context.Background(),
				td: &entity.TraceData{
					Tenant: "test_tenant",
					SpanList: []*loop_span.Span{
						nil, // 这将导致JSON序列化失败
					},
				},
			},
			mockSetup:     func(mqProducer *mocks.MockIProducer) {},
			expectedError: errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode, errorx.WithExtraMsg("trace data marshal failed")),
		},
		{
			name: "正常场景: 大数据分批发送",
			fields: fields{
				producerProxy: map[string]*producerProxy{
					"test_tenant": {
						traceTopic: "test_topic",
						mqProducer: nil, // 将在mockSetup中设置
					},
				},
			},
			args: args{
				ctx: context.Background(),
				td: &entity.TraceData{
					Tenant: "test_tenant",
					SpanList: []*loop_span.Span{
						{
							TraceID: "test_trace_id_1",
							SpanID:  "test_span_id_1",
							Input:   string(make([]byte, maxBatchSize)), // 大数据
						},
						{
							TraceID: "test_trace_id_2",
							SpanID:  "test_span_id_2",
						},
					},
				},
			},
			mockSetup: func(mqProducer *mocks.MockIProducer) {
				// 期望发送两次异步消息
				mqProducer.EXPECT().SendAsync(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(2)
			},
			expectedError: nil,
		},
		{
			name: "异常场景: 单个大span数据过大",
			fields: fields{
				producerProxy: map[string]*producerProxy{
					"test_tenant": {
						traceTopic: "test_topic",
						mqProducer: nil,
					},
				},
			},
			args: args{
				ctx: context.Background(),
				td: &entity.TraceData{
					Tenant: "test_tenant",
					SpanList: []*loop_span.Span{
						{
							TraceID: "test_trace_id",
							SpanID:  "test_span_id",
							Input:   string(make([]byte, maxBatchSize+1)), // 超过限制的数据
						},
					},
				},
			},
			mockSetup:     func(mqProducer *mocks.MockIProducer) {},
			expectedError: errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("span size too large")),
		},
		{
			name: "异常场景: MQ异步发送失败",
			fields: fields{
				producerProxy: map[string]*producerProxy{
					"test_tenant": {
						traceTopic: "test_topic",
						mqProducer: nil, // 将在mockSetup中设置
					},
				},
			},
			args: args{
				ctx: context.Background(),
				td: &entity.TraceData{
					Tenant: "test_tenant",
					SpanList: []*loop_span.Span{
						{
							TraceID: "test_trace_id",
							SpanID:  "test_span_id",
						},
					},
				},
			},
			mockSetup: func(mqProducer *mocks.MockIProducer) {
				mqProducer.EXPECT().SendAsync(gomock.Any(), gomock.Any(), gomock.Any()).Return(assert.AnError)
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

			// 设置producerProxy中的mqProducer
			if tt.fields.producerProxy != nil {
				for _, proxy := range tt.fields.producerProxy {
					proxy.mqProducer = mqProducerMock
				}
			}

			producer := &TraceProducerImpl{
				producerProxy: tt.fields.producerProxy,
			}

			err := producer.IngestSpans(tt.args.ctx, tt.args.td)

			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewTraceProducerImpl(t *testing.T) {
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
			name: "正常场景: 创建trace producer成功",
			args: args{
				traceConfig: nil, // 将在mockSetup中设置
				mqFactory:   nil, // 将在mockSetup中设置
			},
			mockSetup: func(traceConfig *confmocks.MockITraceConfig, mqFactory *mocks.MockIFactory) {
				ingestTenantCfg := map[string]*config.IngestConfig{
					"tenant1": {
						MaxSpanLength: 1024,
						MqProducer: config.MqProducerCfg{
							Topic:         "topic1",
							Addr:          []string{"localhost:9876"},
							Timeout:       5000,
							RetryTimes:    3,
							ProducerGroup: "group1",
						},
					},
					"tenant2": {
						MaxSpanLength: 1024,
						MqProducer: config.MqProducerCfg{
							Topic:         "topic2",
							Addr:          []string{"localhost:9877"},
							Timeout:       3000,
							RetryTimes:    2,
							ProducerGroup: "group2",
						},
					},
				}
				traceConfig.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(ingestTenantCfg, nil)

				producerMock1 := mocks.NewMockIProducer(gomock.NewController(t))
				producerMock1.EXPECT().Start().Return(nil)
				producerMock2 := mocks.NewMockIProducer(gomock.NewController(t))
				producerMock2.EXPECT().Start().Return(nil)

				mqFactory.EXPECT().NewProducer(gomock.Any()).Return(producerMock1, nil)
				mqFactory.EXPECT().NewProducer(gomock.Any()).Return(producerMock2, nil)
			},
			expectedError: false,
		},
		{
			name: "正常场景: 跳过空配置",
			args: args{
				traceConfig: nil,
				mqFactory:   nil,
			},
			mockSetup: func(traceConfig *confmocks.MockITraceConfig, mqFactory *mocks.MockIFactory) {
				ingestTenantCfg := map[string]*config.IngestConfig{
					"tenant1": nil, // 空配置
					"tenant2": {
						MaxSpanLength: 1024,
						MqProducer: config.MqProducerCfg{
							Topic:         "topic2",
							Addr:          []string{"localhost:9877"},
							Timeout:       3000,
							RetryTimes:    2,
							ProducerGroup: "group2",
						},
					},
				}
				traceConfig.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(ingestTenantCfg, nil)

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
				traceConfig.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(nil, assert.AnError)
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
				ingestTenantCfg := map[string]*config.IngestConfig{
					"tenant1": {
						MaxSpanLength: 1024,
						MqProducer: config.MqProducerCfg{
							Topic:         "", // 空topic
							Addr:          []string{"localhost:9876"},
							Timeout:       5000,
							RetryTimes:    3,
							ProducerGroup: "group1",
						},
					},
				}
				traceConfig.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(ingestTenantCfg, nil)
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
				ingestTenantCfg := map[string]*config.IngestConfig{
					"tenant1": {
						MqProducer: config.MqProducerCfg{
							Topic:         "topic1",
							Addr:          []string{"localhost:9876"},
							Timeout:       5000,
							RetryTimes:    3,
							ProducerGroup: "group1",
						},
					},
				}
				traceConfig.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(ingestTenantCfg, nil)
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
				ingestTenantCfg := map[string]*config.IngestConfig{
					"tenant1": {
						MqProducer: config.MqProducerCfg{
							Topic:         "topic1",
							Addr:          []string{"localhost:9876"},
							Timeout:       5000,
							RetryTimes:    3,
							ProducerGroup: "group1",
						},
					},
				}
				traceConfig.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(ingestTenantCfg, nil)

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
			singletonTraceProducer = nil
			traceProducerOnce = sync.Once{}

			producer, err := NewTraceProducerImpl(traceConfigMock, mqFactoryMock)

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

func TestTraceProducerImpl_Singleton(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	traceConfigMock := confmocks.NewMockITraceConfig(ctrl)
	mqFactoryMock := mocks.NewMockIFactory(ctrl)

	ingestTenantCfg := map[string]*config.IngestConfig{
		"tenant1": {
			MaxSpanLength: 1024,
			MqProducer: config.MqProducerCfg{
				Topic:         "topic1",
				Addr:          []string{"localhost:9876"},
				Timeout:       5000,
				RetryTimes:    3,
				ProducerGroup: "group1",
			},
		},
	}
	traceConfigMock.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(ingestTenantCfg, nil).Times(2)

	producerMock := mocks.NewMockIProducer(ctrl)
	producerMock.EXPECT().Start().Return(nil).Times(2)

	mqFactoryMock.EXPECT().NewProducer(gomock.Any()).Return(producerMock, nil).Times(2)

	// 重置单例
	singletonTraceProducer = nil
	traceProducerOnce = sync.Once{}

	// 第一次调用
	producer1, err1 := NewTraceProducerImpl(traceConfigMock, mqFactoryMock)
	assert.NoError(t, err1)
	assert.NotNil(t, producer1)

	// 第二次调用，应该返回同一个实例
	producer2, err2 := NewTraceProducerImpl(traceConfigMock, mqFactoryMock)
	assert.NoError(t, err2)
	assert.NotNil(t, producer2)
	assert.Equal(t, producer1, producer2)
}
