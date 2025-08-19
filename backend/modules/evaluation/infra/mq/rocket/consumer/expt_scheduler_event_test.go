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

func TestExptSchedulerConsumer_HandleMessage(t *testing.T) {
	type fields struct {
		scheduler *mock_service.MockExptSchedulerEvent
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
			wantErr:     nil, // 反序列化失败返回 nil
		},
		{
			name:   "event.Session is nil, schedule success",
			fields: fields{},
			args: args{
				ctx: context.Background(),
				msg: func() *mq.MessageExt {
					event := &entity.ExptScheduleEvent{
						ExptID:    1,
						SpaceID:   2,
						ExptRunID: 3,
					}
					b, _ := json.Marshal(event)
					return &mq.MessageExt{
						Message: mq.Message{Body: b},
						MsgID:   "msg2",
					}
				}(),
			},
			prepareMock: func(f *fields) {
				f.scheduler.EXPECT().Schedule(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: nil,
		},
		{
			name:   "event.Session not nil, schedule returns error",
			fields: fields{},
			args: args{
				ctx: context.Background(),
				msg: func() *mq.MessageExt {
					event := &entity.ExptScheduleEvent{
						ExptID:    1,
						SpaceID:   2,
						ExptRunID: 3,
						Session:   &entity.Session{UserID: "u1"},
					}
					b, _ := json.Marshal(event)
					return &mq.MessageExt{
						Message: mq.Message{Body: b},
						MsgID:   "msg3",
					}
				}(),
			},
			prepareMock: func(f *fields) {
				f.scheduler.EXPECT().Schedule(gomock.Any(), gomock.Any()).Return(errors.New("schedule error"))
			},
			wantErr: errors.New("schedule error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := tt.fields
			f.scheduler = mock_service.NewMockExptSchedulerEvent(ctrl)
			if tt.prepareMock != nil {
				tt.prepareMock(&f)
			}

			c := &ExptSchedulerConsumer{
				scheduler: f.scheduler,
			}
			err := c.HandleMessage(tt.args.ctx, tt.args.msg)
			if !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("HandleMessage() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
