// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package consumer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	mocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCurrentRequestBrokerSeesRetry(t *testing.T) {
	for _, request := range []string{"", "rq1"} {
		t.Run(request, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := mocks.NewMockExptItemEvalEvent(ctrl)
			want := errno.NewMQRetryErr("item lock busy")
			event := &entity.ExptItemEvalEvent{RequestID: request, SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 11, ExptRunMode: entity.EvaluationModeSubmit, Session: &entity.Session{UserID: "original"}}
			body, err := json.Marshal(event)
			require.NoError(t, err)
			svc.EXPECT().Eval(gomock.Any(), gomock.Eq(event)).Return(want)
			handler := NewExptRecordEvalConsumer(svc)
			err = handler.HandleMessage(context.Background(), &mq.MessageExt{Message: mq.Message{Body: body}})
			require.Same(t, want, err)
		})
	}
}
