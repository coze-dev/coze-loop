// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	configermocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestBatchGetEvaluatorRecordsOApi_UnapprovedArchiveIsHiddenWhenLegacyEnforceOff(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	recordService := servicemocks.NewMockEvaluatorRecordService(ctrl)
	evaluatorService := servicemocks.NewMockEvaluatorService(ctrl)
	configer := configermocks.NewMockIConfiger(ctrl)
	provider := &evidenceArchiveURLProviderStub{deny: true}
	record := newEvidenceArchiveRecordForTest(10, 200, "evaluator-evidence/v1/200/10/evidence.tar.gz", "complete")
	record.EvaluatorVersionID = 30
	app := &EvalOpenAPIApplication{auth: auth, evaluatorRecordService: recordService, evaluatorService: evaluatorService, configer: configer, fileProvider: provider, metric: &fakeOpenAPIMetric{}}
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *rpc.AuthorizationParam) error {
		assert.Equal(t, int64(100), param.SpaceID)
		return nil
	})
	recordService.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{10}, false, false).Return([]*entity.EvaluatorRecord{record}, nil)
	configer.EXPECT().GetCrossSpaceRecordReadEnforce(gomock.Any()).Return(false)
	evaluatorService.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), nil, []int64{30}, true).Return(nil, nil)
	resp, err := app.BatchGetEvaluatorRecordsOApi(context.Background(), &openapi.BatchGetEvaluatorRecordsOApiRequest{WorkspaceID: gptr.Of(int64(100)), EvaluatorRecordIds: []int64{10}})
	require.NoError(t, err)
	require.Len(t, resp.Data.Records, 1)
	assert.Nil(t, resp.Data.Records[0].GetEvaluatorOutputData().GetEvidenceArchive())
	require.Len(t, provider.requests, 1)
	assert.Equal(t, int64(100), provider.requests[0].CallerSpaceID, "request space must not be replaced")
	assert.Equal(t, int64(200), provider.requests[0].ResourceSpaceID)
	assert.Empty(t, provider.gotKeys)
}

func TestEvidenceArchiveDraftVersionDelegatesToAuthorizedSigner(t *testing.T) {
	record := newEvidenceArchiveRecordForTest(10, 200, "evidence/draft.tar.gz", "complete")
	record.EvaluatorVersionID = 0
	provider := &evidenceArchiveURLProviderStub{urls: map[string]string{"evidence/draft.tar.gz": "https://signed.example/draft"}}
	require.NoError(t, fillEvaluatorEvidenceArchiveURLs(context.Background(), provider, []*entity.EvaluatorRecord{record}, nil))
	require.NotNil(t, record.EvaluatorOutputData.EvidenceArchive)
	assert.Equal(t, "https://signed.example/draft", record.EvaluatorOutputData.EvidenceArchive.FornaxEvaluatorLogURL)
	require.Len(t, provider.requests, 1)
	assert.Zero(t, provider.requests[0].EvaluatorVersionID)
	assert.Equal(t, int64(200), provider.requests[0].CallerSpaceID)
}

func TestExperimentArchiveReadsConsumeStrictAuthorization(t *testing.T) {
	for _, entry := range []string{"experiment", "openapi"} {
		for _, deny := range []bool{true, false} {
			name := entry + "/allowed"
			if deny {
				name = entry + "/denied"
			}
			t.Run(name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				auth := rpcmocks.NewMockIAuthProvider(ctrl)
				resultSvc := servicemocks.NewMockExptResultService(ctrl)
				record := newEvidenceArchiveRecordForTest(10, 200, "evidence/private.tar.gz", "complete")
				items := []*entity.ItemResult{{TurnResults: []*entity.TurnResult{{ExperimentResults: []*entity.ExperimentResult{{
					ExperimentID: 7, Payload: &entity.ExperimentTurnPayload{EvaluatorOutput: &entity.TurnEvaluatorOutput{EvaluatorRecords: map[int64]*entity.EvaluatorRecord{30: record}}},
				}}}}}}
				provider := &evidenceArchiveURLProviderStub{deny: deny, urls: map[string]string{"evidence/private.tar.gz": "https://signed.example/private"}}
				auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				resultSvc.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).Return(&entity.MGetExperimentReportResult{ItemResults: items, Total: 1}, nil)
				var response any
				var err error
				if entry == "experiment" {
					app := &experimentApplication{auth: auth, resultSvc: resultSvc, fileProvider: provider}
					response, err = app.BatchGetExperimentResult_(context.Background(), &exptpb.BatchGetExperimentResultRequest{WorkspaceID: 100, ExperimentIds: []int64{7}})
				} else {
					app := &EvalOpenAPIApplication{auth: auth, resultSvc: resultSvc, fileProvider: provider, metric: &fakeOpenAPIMetric{}}
					response, err = app.ListExperimentResultOApi(context.Background(), &openapi.ListExperimentResultOApiRequest{WorkspaceID: gptr.Of(int64(100)), ExperimentID: gptr.Of(int64(7))})
				}
				require.NoError(t, err)
				require.Len(t, provider.requests, 1)
				assert.Equal(t, int64(100), provider.requests[0].CallerSpaceID)
				assert.Equal(t, int64(200), provider.requests[0].ResourceSpaceID)
				assert.Equal(t, int64(10), provider.requests[0].RecordID)
				assert.Equal(t, int64(30), provider.requests[0].EvaluatorVersionID)
				body, err := json.Marshal(response)
				require.NoError(t, err)
				if deny {
					assert.NotContains(t, string(body), "evidence_archive")
					assert.Empty(t, provider.gotKeys)
				} else {
					assert.Contains(t, string(body), "https://signed.example/private")
				}
			})
		}
	}
}

func TestEvidenceArchiveUnknownIdentityNeverReachesSigner(t *testing.T) {
	for _, field := range []string{"record", "space", "caller"} {
		t.Run(field, func(t *testing.T) {
			record := newEvidenceArchiveRecordForTest(10, 200, "evidence/private.tar.gz", "complete")
			caller := int64(100)
			switch field {
			case "record":
				record.ID = 0
			case "space":
				record.SpaceID = 0
			case "caller":
				caller = 0
			}
			provider := &evidenceArchiveURLProviderStub{}
			require.NoError(t, fillEvaluatorEvidenceArchiveURLs(context.Background(), provider, []*entity.EvaluatorRecord{record}, &caller))
			assert.Empty(t, provider.requests)
			assert.Nil(t, record.EvaluatorOutputData.EvidenceArchive)
		})
	}
}
