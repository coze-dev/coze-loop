// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rmqreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	mqmocks "github.com/coze-dev/coze-loop/backend/infra/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/consumer"
	consumermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/consumer/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func TestRmqReceiver_Start(t *testing.T) {
	type fields struct {
		mqFactory    mq.IFactory
		nextConsumer consumer.Consumer
		config       *Config
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		wantErr      bool
	}{
		{
			name: "start successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mqConsumer := mqmocks.NewMockIConsumer(ctrl)
				mqConsumer.EXPECT().Start().Return(nil)
				mqConsumer.EXPECT().Close().Return(nil)
				mqConsumer.EXPECT().RegisterHandler(gomock.Any()).Return()
				mqFactory := mqmocks.NewMockIFactory(ctrl)
				mqFactory.EXPECT().NewConsumer(gomock.Any()).Return(mqConsumer, nil)
				return fields{
					mqFactory:    mqFactory,
					nextConsumer: nil,
					config:       &Config{},
				}
			},
			wantErr: false,
		},
		{
			name: "start failed when factory is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mqConsumer := mqmocks.NewMockIConsumer(ctrl)
				mqConsumer.EXPECT().Start().Return(assert.AnError)
				mqConsumer.EXPECT().Close().Return(nil)
				mqConsumer.EXPECT().RegisterHandler(gomock.Any()).Return()
				mqFactory := mqmocks.NewMockIFactory(ctrl)
				mqFactory.EXPECT().NewConsumer(gomock.Any()).Return(mqConsumer, nil)
				return fields{
					mqFactory:    mqFactory,
					nextConsumer: nil,
					config:       &Config{},
				}
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &rmqReceiver{
				mqFactory:    fields.mqFactory,
				nextConsumer: fields.nextConsumer,
				config:       fields.config,
			}
			err := r.Start(context.Background())
			assert.Equal(t, tt.wantErr, err != nil)
			_ = r.Shutdown(context.Background())
		})
	}
}

func TestRmqReceiver_HandleMessage(t *testing.T) {
	type fields struct {
		nextConsumer consumer.Consumer
	}
	tests := []struct {
		name    string
		fields  fields
		msg     *mq.MessageExt
		wantErr bool
	}{
		{
			name: "handle message successfully",
			fields: fields{
				nextConsumer: func() consumer.Consumer {
					ctrl := gomock.NewController(t)
					mock := consumermocks.NewMockConsumer(ctrl)
					mock.EXPECT().ConsumeTraces(gomock.Any(), gomock.Any()).Return(nil)
					return mock
				}(),
			},
			msg: &mq.MessageExt{
				Message: mq.Message{
					Body: validTraceData(),
				},
			},
			wantErr: false,
		},
		{
			name: "handle message failed when unmarshal error",
			fields: fields{
				nextConsumer: nil,
			},
			msg: &mq.MessageExt{
				Message: mq.Message{
					Body: []byte(`invalid json`),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &rmqReceiver{
				nextConsumer: tt.fields.nextConsumer,
			}
			err := r.HandleMessage(context.Background(), tt.msg)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func validTraceData() []byte {
	td := entity.TraceData{
		Tenant: "a",
		TenantInfo: entity.TenantInfo{
			TTL: loop_span.TTL30d,
		},
		SpanList: loop_span.SpanList{
			{
				StartTime:       time.Now().Add(-time.Hour * 12).UnixMicro(),
				SpanID:          "0000000000000001",
				TraceID:         "00000000000000000000000000000001",
				DurationMicros:  0,
				LogicDeleteTime: 0,
				TagsLong: map[string]int64{
					"a": 1,
				},
				SystemTagsLong: map[string]int64{},
				SystemTagsString: map[string]string{
					"dc": "aa",
					"x":  "11",
				},
			},
		},
	}
	out, _ := json.Marshal(td)
	return out
}
