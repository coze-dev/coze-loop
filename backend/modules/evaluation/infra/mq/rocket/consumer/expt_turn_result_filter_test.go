// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/stretchr/testify/require"
)

type filterConsumerService struct {
	evaluation.ExperimentService
	call func(*expt.UpsertExptTurnResultFilterRequest) (*expt.UpsertExptTurnResultFilterResponse, error)
}

func (s *filterConsumerService) UpsertExptTurnResultFilter(_ context.Context, r *expt.UpsertExptTurnResultFilterRequest) (*expt.UpsertExptTurnResultFilterResponse, error) {
	return s.call(r)
}

func TestFilterConsumerRejectsInvalidMessages(t *testing.T) {
	for _, body := range []string{`{`, `null`, `{}`, `{"SpaceID":1,"ExperimentID":2,"ItemID":[0]}`, `{"SpaceID":1,"ExperimentID":2,"FilterType":"unknown"}`, `{"SpaceID":1,"ExperimentID":2,"RetryTimes":-1}`} {
		t.Run(body, func(t *testing.T) {
			h := NewExptTurnResultFilterConsumer(&filterConsumerService{call: func(*expt.UpsertExptTurnResultFilterRequest) (*expt.UpsertExptTurnResultFilterResponse, error) {
				t.Fatal("invalid event reached application")
				return nil, nil
			}})
			require.Error(t, h.HandleMessage(context.Background(), &mq.MessageExt{Message: mq.Message{Body: []byte(body)}}))
		})
	}
}

func TestFilterConsumerPreservesApplicationFailures(t *testing.T) {
	want := errors.New("index unavailable")
	for _, tc := range []struct {
		name string
		resp *expt.UpsertExptTurnResultFilterResponse
		err  error
	}{
		{"handler", nil, want},
		{"nil response", nil, nil},
		{"base response", &expt.UpsertExptTurnResultFilterResponse{BaseResp: &base.BaseResp{StatusCode: 500, StatusMessage: "index unavailable"}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewExptTurnResultFilterConsumer(&filterConsumerService{call: func(r *expt.UpsertExptTurnResultFilterRequest) (*expt.UpsertExptTurnResultFilterResponse, error) {
				require.Equal(t, int64(1), r.GetWorkspaceID())
				require.Equal(t, int64(2), r.GetExperimentID())
				return tc.resp, tc.err
			}})
			err := h.HandleMessage(context.Background(), &mq.MessageExt{Message: mq.Message{Body: []byte(`{"SpaceID":1,"ExperimentID":2,"ItemID":[3]}`)}})
			require.Error(t, err)
			if tc.err != nil {
				require.ErrorIs(t, err, want)
			}
		})
	}
}
