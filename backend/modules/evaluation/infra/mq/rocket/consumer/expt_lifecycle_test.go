// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	mock_service "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestExptLifecycleConsumer_HandleMessage(t *testing.T) {
	type fields struct {
		handler *mock_service.MockExptLifecycleEventHandler
	}

	type args struct {
		ctx context.Context
		msg *mq.MessageExt
	}

	tests := []struct {
		name        string
		fields      fields
		args        args
		prepareMock func(f *fields)
		wantErr     error
	}{
		{
			name:   "json unmarshal fail",
			fields: fields{},
			args: args{
				ctx: context.Background(),
				msg: &mq.MessageExt{
					Message: mq.Message{
						Body: []byte("invalid json"),
					},
					MsgID: "msg1",
				},
			},
			prepareMock: func(f *fields) {},
			wantErr:     nil,
		},
		{
			name:   "valid event, handler success",
			fields: fields{},
			args: args{
				ctx: context.Background(),
				msg: func() *mq.MessageExt {
					runID := int64(10)
					event := &entity.ExptLifecycleEvent{
						ExptID:     1,
						ExptRunID:  &runID,
						SpaceID:    2,
						FromStatus: entity.ExptStatus_Pending,
						ToStatus:   entity.ExptStatus_Processing,
						ExptType:   entity.ExptType_Offline,
						SourceType: entity.SourceType_Evaluation,
					}
					b, _ := json.Marshal(event)
					return &mq.MessageExt{
						Message: mq.Message{Body: b},
						MsgID:   "msg2",
					}
				}(),
			},
			prepareMock: func(f *fields) {
				f.handler.EXPECT().HandleLifecycleEvent(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: nil,
		},
		{
			name:   "valid event, handler returns error",
			fields: fields{},
			args: args{
				ctx: context.Background(),
				msg: func() *mq.MessageExt {
					runID := int64(10)
					event := &entity.ExptLifecycleEvent{
						ExptID:     1,
						ExptRunID:  &runID,
						SpaceID:    2,
						FromStatus: entity.ExptStatus_Pending,
						ToStatus:   entity.ExptStatus_Processing,
						ExptType:   entity.ExptType_Offline,
						SourceType: entity.SourceType_Evaluation,
					}
					b, _ := json.Marshal(event)
					return &mq.MessageExt{
						Message: mq.Message{Body: b},
						MsgID:   "msg3",
					}
				}(),
			},
			prepareMock: func(f *fields) {
				f.handler.EXPECT().HandleLifecycleEvent(gomock.Any(), gomock.Any()).Return(errors.New("handler error"))
			},
			wantErr: errors.New("handler error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := tt.fields
			f.handler = mock_service.NewMockExptLifecycleEventHandler(ctrl)
			if tt.prepareMock != nil {
				tt.prepareMock(&f)
			}

			c := &ExptLifecycleConsumer{
				handler: f.handler,
			}
			err := c.HandleMessage(tt.args.ctx, tt.args.msg)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("HandleMessage() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
