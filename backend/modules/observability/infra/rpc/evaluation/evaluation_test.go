// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluation

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/rpc/evaluation/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
)

//go:generate mockgen -destination=mock_expt_client_test.go -package=evaluation github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt/experimentservice Client

func TestSubmitExperiment_WorkspaceIDZero(t *testing.T) {
	t.Parallel()
	p := &EvaluationProvider{}
	_, _, err := p.SubmitExperiment(context.Background(), &rpc.SubmitExperimentReq{WorkspaceID: 0})
	assert.Error(t, err)
}

func TestSubmitExperiment_Success(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	tplID := int64(999)
	itemConcur := int32(5)
	itemRetry := int32(3)

	mockClient.EXPECT().SubmitExperiment(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *expt.SubmitExperimentRequest, opts ...interface{}) (*expt.SubmitExperimentResponse, error) {
			assert.Equal(t, int64(123), req.WorkspaceID)
			assert.Equal(t, &tplID, req.ExptTemplateID)
			assert.Equal(t, &itemConcur, req.ItemConcurNum)
			assert.Equal(t, &itemRetry, req.ItemRetryNum)
			assert.Equal(t, gptr.Of(true), req.EnableWeightedScore)
			return &expt.SubmitExperimentResponse{
				Experiment: &domainExpt.Experiment{ID: gptr.Of(int64(100))},
				RunID:      gptr.Of(int64(200)),
			}, nil
		},
	)

	p := &EvaluationProvider{client: mockClient}
	exptID, runID, err := p.SubmitExperiment(context.Background(), &rpc.SubmitExperimentReq{
		WorkspaceID:    123,
		ExptTemplateID: &tplID,
		ItemConcurNum:  &itemConcur,
		ItemRetryNum:   &itemRetry,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(100), exptID)
	assert.Equal(t, int64(200), runID)
}

func TestSubmitExperiment_WithIsWorkflowScheduled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mockClient.EXPECT().SubmitExperiment(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *expt.SubmitExperimentRequest, opts ...interface{}) (*expt.SubmitExperimentResponse, error) {
			require.NotNil(t, req.TriggerType)
			return &expt.SubmitExperimentResponse{
				Experiment: &domainExpt.Experiment{ID: gptr.Of(int64(1))},
				RunID:      gptr.Of(int64(2)),
			}, nil
		},
	)

	p := &EvaluationProvider{client: mockClient}
	_, _, err := p.SubmitExperiment(context.Background(), &rpc.SubmitExperimentReq{
		WorkspaceID:         123,
		IsWorkflowScheduled: gptr.Of(true),
	})
	assert.NoError(t, err)
}

func TestSubmitExperiment_ClientError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mocks.NewMockClient(ctrl)
	mockClient.EXPECT().SubmitExperiment(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

	p := &EvaluationProvider{client: mockClient}
	_, _, err := p.SubmitExperiment(context.Background(), &rpc.SubmitExperimentReq{
		WorkspaceID: 123,
	})
	assert.Error(t, err)
}
