// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestItemBusyDefersOnlyAfterConfirmedPublish(t *testing.T) {
	for _, name := range []string{"confirmed", "ordinary", "evaluator_callback", "send_failure", "cancelled_after_accept", "publisher_missing", "factory_missing", "context_cancelled"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			factory, server := finishRealLockerFactory(t)
			event := &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, ExptRunMode: entity.EvaluationModeFailRetry, RequestID: "original-request", RetryTimes: 2, MaxRetryTimes: 7, CreateAt: 1700000000, AsyncReportTrigger: true, ResumeSource: &entity.ExptResumeSource{Kind: "target", TurnID: 0, RecordID: 11, InvokeID: "11"}, Session: &entity.Session{UserID: "original-user"}, Ext: map[string]string{"preserved": "original"}}
			if name == "ordinary" {
				event.RequestID = ""
				event.ResumeSource = nil
				event.AsyncReportTrigger = false
				event.ExptRunMode = entity.EvaluationModeSubmit
			}
			if name == "evaluator_callback" {
				event.AsyncReportTrigger = false
				event.AsyncEvaluatorReportTrigger = true
				event.ResumeSource.Kind = "evaluator"
			}
			original := event.Clone()
			key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", event.ExptID, event.EvalSetItemID)
			require.NoError(t, server.Set(key, "other-holder"))
			config := componentmocks.NewMockIConfiger(ctrl)
			config.EXPECT().GetExptExecConf(gomock.Any(), int64(1)).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{IntervalSecond: 3}}).AnyTimes()
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			svc := &ExptItemEventEvalServiceImpl{configer: config, publisher: publisher, itemLocker: factory}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sendError := errors.New("send not acknowledged")
			switch name {
			case "publisher_missing":
				svc.publisher = nil
			case "factory_missing":
				svc.itemLocker = func() lock.ILocker { return nil }
			case "context_cancelled":
				cancel()
			default:
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil()).DoAndReturn(func(_ context.Context, copy *entity.ExptItemEvalEvent, delay *time.Duration, modify func(*entity.ExptItemEvalEvent)) error {
					require.Equal(t, original, copy, "lock contention cannot alter source identity or spend business retry budget")
					require.NotNil(t, delay)
					require.Equal(t, 3*time.Second, *delay)
					require.Nil(t, modify)
					if name == "send_failure" {
						return sendError
					}
					if name == "cancelled_after_accept" {
						cancel()
					}
					return nil
				})
			}
			err := svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error {
				t.Fatal("a busy delivery must not execute or release another holder's resources")
				return nil
			}))(ctx, event)
			if name == "confirmed" || name == "ordinary" || name == "evaluator_callback" {
				require.NoError(t, err)
			} else {
				require.True(t, errno.NeedMQRetry(err), "unconfirmed follow-up requires broker retry: %v", err)
			}
			if name == "send_failure" {
				require.ErrorIs(t, err, sendError)
			}
			require.Equal(t, original, event)
			owner, ownerErr := server.Get(key)
			require.NoError(t, ownerErr)
			require.Equal(t, "other-holder", owner)
		})
	}
}
