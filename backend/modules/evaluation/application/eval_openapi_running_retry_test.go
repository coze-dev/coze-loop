// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domainexpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/js_conv"
)

func TestOpenAPIRunningRetryIdentityAndReceipt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		accepted int32
		rejected []*exptpb.RetryRejectedItem
		wantJSON string
	}{
		{
			name:     "partial acceptance",
			accepted: 1,
			rejected: []*exptpb.RetryRejectedItem{{ItemID: gptr.Of(int64(9223372036854775806)), Reason: gptr.Of("busy")}},
			wantJSON: `{"run_id":"9223372036854775807","request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05","accepted_count":1,"rejected_items":[{"item_id":"9223372036854775806","reason":"busy"}],"snapshot_at_ms":"1793275200123"}`,
		},
		{
			name:     "zero acceptance with rejection",
			rejected: []*exptpb.RetryRejectedItem{{ItemID: gptr.Of(int64(9223372036854775806)), Reason: gptr.Of("not_found")}},
			wantJSON: `{"run_id":"9223372036854775807","request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05","accepted_count":0,"rejected_items":[{"item_id":"9223372036854775806","reason":"not_found"}],"snapshot_at_ms":"1793275200123"}`,
		},
		{
			name:     "zero acceptance with empty rejection list",
			rejected: []*exptpb.RetryRejectedItem{},
			wantJSON: `{"run_id":"9223372036854775807","request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05","accepted_count":0,"rejected_items":[],"snapshot_at_ms":"1793275200123"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			fakeApp := &fakeExperimentApp{retryResp: &exptpb.RetryExperimentResponse{
				RunID: gptr.Of(int64(9223372036854775807)), RequestID: gptr.Of("72cc8921-40f4-4a43-b772-aaf2b5dcdf05"),
				AcceptedCount: gptr.Of(tc.accepted), RejectedItems: tc.rejected, SnapshotAtMs: gptr.Of(int64(1793275200123)),
			}}
			app := &EvalOpenAPIApplication{auth: auth, experimentApp: fakeApp, metric: &fakeOpenAPIMetric{}}
			var req openapi.RetryExperimentOApiRequest
			require.NoError(t, js_conv.GetUnmarshaler()([]byte(`{"workspace_id":"9","experiment_id":"7","retry_mode":"retry_target_items","item_ids":["9223372036854775806"],"expected_run_id":"9223372036854775807","request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05","ext":{"caller":"cli"}}`), &req))
			resp, err := app.RetryExperimentOApi(context.Background(), &req)
			require.NoError(t, err)
			require.NotNil(t, fakeApp.lastRetryReq)
			assert.Equal(t, int64(9223372036854775807), fakeApp.lastRetryReq.GetExpectedRunID())
			assert.Equal(t, "72cc8921-40f4-4a43-b772-aaf2b5dcdf05", fakeApp.lastRetryReq.GetRequestID())
			assert.Equal(t, domainexpt.ExptRetryMode_RetryTargetItems, fakeApp.lastRetryReq.GetRetryMode())
			assert.Equal(t, []int64{9223372036854775806}, fakeApp.lastRetryReq.ItemIds)
			assert.Equal(t, map[string]string{"caller": "cli"}, fakeApp.lastRetryReq.Ext)
			body, err := js_conv.GetMarshaler()(resp.Data)
			require.NoError(t, err)
			assert.JSONEq(t, tc.wantJSON, string(body))
		})
	}
}

func TestOpenAPIRunningRetryLegacyAndError(t *testing.T) {
	for _, current := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy request", true: "current run error"}[current], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			fakeApp := &fakeExperimentApp{retryResp: &exptpb.RetryExperimentResponse{RunID: gptr.Of(int64(18))}}
			input := `{"workspace_id":"9","experiment_id":"7"}`
			if current {
				input = `{"workspace_id":"9","experiment_id":"7","expected_run_id":"8","request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05"}`
				fakeApp.retryResp = nil
				fakeApp.retryErr = errors.New("request fingerprint conflict")
			}
			var req openapi.RetryExperimentOApiRequest
			require.NoError(t, js_conv.GetUnmarshaler()([]byte(input), &req))
			app := &EvalOpenAPIApplication{auth: auth, experimentApp: fakeApp, metric: &fakeOpenAPIMetric{}}
			resp, err := app.RetryExperimentOApi(context.Background(), &req)
			if current {
				require.ErrorIs(t, err, fakeApp.retryErr)
				require.Nil(t, resp)
				assert.Equal(t, int64(8), fakeApp.lastRetryReq.GetExpectedRunID())
				assert.Equal(t, "72cc8921-40f4-4a43-b772-aaf2b5dcdf05", fakeApp.lastRetryReq.GetRequestID())
				return
			}
			require.NoError(t, err)
			assert.Nil(t, fakeApp.lastRetryReq.ExpectedRunID)
			assert.Nil(t, fakeApp.lastRetryReq.RequestID)
			assert.Equal(t, domainexpt.ExptRetryMode_RetryFailure, fakeApp.lastRetryReq.GetRetryMode())
			assert.Equal(t, int64(18), resp.Data.GetRunID())
		})
	}
}

func TestOpenAPIRunningRetryRealExperimentApplication(t *testing.T) {
	for _, kind := range []string{"receipt", "manager error", "missing request_id", "zero expected_run_id"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := svcmocks.NewMockIExptManager(ctrl)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationWithoutSPIParam) error {
				assert.Equal(t, "7", param.ObjectID)
				assert.Equal(t, int64(9), param.ResourceSpaceID)
				assert.Equal(t, "owner", *param.OwnerID)
				return nil
			})
			manager.EXPECT().Get(gomock.Any(), int64(7), int64(9), gomock.Any()).Return(&entity.Experiment{
				ID: 7, SpaceID: 9, LatestRunID: 8, CreatedBy: "owner", Status: entity.ExptStatus_Processing,
			}, nil)
			input := `{"workspace_id":"9","experiment_id":"7","expected_run_id":"8","request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05","retry_mode":"retry_target_items","item_ids":["11","12"]}`
			var req openapi.RetryExperimentOApiRequest
			require.NoError(t, js_conv.GetUnmarshaler()([]byte(input), &req))
			failure := errors.New("current run mismatch")
			switch kind {
			case "missing request_id":
				req.RequestID = nil
			case "zero expected_run_id":
				req.ExpectedRunID = gptr.Of(int64(0))
			default:
				manager.EXPECT().RetryInCurrentRun(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RetryInCurrentRunRequest) (*entity.ManualRetryReceipt, error) {
					assert.Equal(t, int64(8), r.ExpectedRunID)
					assert.Equal(t, "72cc8921-40f4-4a43-b772-aaf2b5dcdf05", r.RequestID)
					assert.Equal(t, entity.EvaluationModeRetryItems, r.Mode)
					assert.Equal(t, []int64{11, 12}, r.ItemIDs)
					if kind == "manager error" {
						return nil, failure
					}
					return &entity.ManualRetryReceipt{
						RequestID: r.RequestID, AcceptedCount: 1,
						RejectedItems: []*entity.ManualRetryRejectedItem{{ItemID: 12, Reason: "busy"}}, SnapshotAtMS: 1793275200123,
					}, nil
				})
			}
			app := &EvalOpenAPIApplication{auth: auth, metric: &fakeOpenAPIMetric{}, experimentApp: &experimentApplication{manager: manager, auth: auth}}
			resp, err := app.RetryExperimentOApi(context.Background(), &req)
			if kind != "receipt" {
				require.Error(t, err)
				require.Nil(t, resp)
				if kind == "manager error" {
					assert.ErrorIs(t, err, failure)
				}
				return
			}
			require.NoError(t, err)
			body, err := js_conv.GetMarshaler()(resp.Data)
			require.NoError(t, err)
			assert.JSONEq(t, `{"run_id":"8","request_id":"72cc8921-40f4-4a43-b772-aaf2b5dcdf05","accepted_count":1,"rejected_items":[{"item_id":"12","reason":"busy"}],"snapshot_at_ms":"1793275200123"}`, string(body))
		})
	}
}
