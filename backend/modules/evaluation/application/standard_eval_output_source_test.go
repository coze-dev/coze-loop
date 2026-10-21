// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/bytedance/gg/gptr"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSourceStandardRequestJSONPresentEmptyAnd19Digits(t *testing.T) {
	raw := `{"workspace_id":1,"expt_id":2,"item_ids":[3],"expected_expt_run_id":9223372036854775806,"expected_request_id":"","expected_source_turn_records":[{"turn_id":"0","target_record_id":"0","registered":[{"version_id":"9223372036854775806","alias":"","record_id":"9223372036854775805"}]}],"source_only":true}`
	var req exptpb.MGetExperimentStandardEvalOutputsRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))
	encoded, err := json.Marshal(&req)
	require.NoError(t, err)
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &got))
	require.Equal(t, `""`, string(got["expected_request_id"]), "present empty raw request identity must survive")
	require.Equal(t, "9223372036854775806", string(got["expected_expt_run_id"]))
	require.JSONEq(t, `[{"turn_id":"0","target_record_id":"0","registered":[{"version_id":"9223372036854775806","alias":"","record_id":"9223372036854775805"}]}]`, string(got["expected_source_turn_records"]))
	require.NoError(t, req.IsValid())
	transport := thrift.NewTMemoryBuffer()
	protocol := thrift.NewTBinaryProtocolTransport(transport)
	require.NoError(t, req.Write(protocol))
	wire := append([]byte(nil), transport.Bytes()...)
	var decoded exptpb.MGetExperimentStandardEvalOutputsRequest
	require.NoError(t, decoded.Read(protocol))
	require.Equal(t, req, decoded)
	var fast exptpb.MGetExperimentStandardEvalOutputsRequest
	n, err := fast.FastRead(wire)
	require.NoError(t, err)
	require.Equal(t, len(wire), n)
	require.Equal(t, req, fast)
	buffer := make([]byte, req.BLength())
	require.Equal(t, len(buffer), req.FastWriteNocopy(buffer, nil))
	// Normal and fast generators may emit fields in different legal Thrift order.
	var fastWritten exptpb.MGetExperimentStandardEvalOutputsRequest
	n, err = fastWritten.FastRead(buffer)
	require.NoError(t, err)
	require.Equal(t, len(buffer), n)
	require.Equal(t, req, fastWritten)
}

func sourceStandardFixture() *entity.ExptStandardEvalOutputSource {
	c := entity.ExptItemCurrentRequest{SpaceID: 1, ExptID: 2, RunID: 3, ItemID: 4, Status: entity.ItemRunState_Success, ResultState: entity.ExptItemResultStateResulted}
	turn := &entity.ExptTurnResult{ID: 20, SpaceID: 1, ExptID: 2, ExptRunID: 3, ItemID: 4, TurnID: 5, TargetResultID: 6, Status: int32(entity.TurnRunState_Success)}
	log := turn.ToRunLogDO()
	log.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 101, RecordID: 1001}}}
	return &entity.ExptStandardEvalOutputSource{Snapshot: &entity.ExptItemResultPublishSource{
		Item:    &entity.ExptItemResult{ID: 10, SpaceID: 1, ExptID: 2, ExptRunID: 3, ItemID: 4, Status: entity.ItemRunState_Success},
		ItemLog: &entity.ExptItemResultRunLog{ID: 11, SpaceID: 1, ExptID: 2, ExptRunID: 3, ItemID: 4, Status: int32(entity.ItemRunState_Success), ResultState: int32(entity.ExptItemResultStateResulted)},
		Source:  &entity.ExptCurrentItemRunSource{Current: c, CanonicalTurns: []*entity.ExptTurnResult{turn}, Turns: map[int64]*entity.ExptTurnResultRunLog{5: log}, CurrentRefs: []*entity.ExptTurnEvaluatorResultRef{{ID: 30, SpaceID: 1, ExptID: 2, ExptTurnResultID: 20, EvaluatorVersionID: 101, EvaluatorResultID: 1001}}},
	}}
}

type sourceStandardResultService struct {
	*servicemocks.MockExptResultService
	read func(context.Context, int64, int64, int64, int64) (*entity.ExptStandardEvalOutputSource, error)
}

func (s *sourceStandardResultService) GetStandardEvalOutputSource(ctx context.Context, space, expt, run, item int64, expectedRequestID ...*string) (*entity.ExptStandardEvalOutputSource, error) {
	return s.read(ctx, space, expt, run, item)
}

func sourceStandardExpectedRequest(frozen *entity.ExptStandardEvalOutputSource) *exptpb.MGetExperimentStandardEvalOutputsRequest {
	c := frozen.Snapshot.Source.Current
	records, _ := service.StandardEvalOutputSourceRecords(frozen)
	return &exptpb.MGetExperimentStandardEvalOutputsRequest{WorkspaceID: c.SpaceID, ExptID: c.ExptID, ItemIds: []int64{c.ItemID}, ExpectedExptRunID: gptr.Of(c.RunID), ExpectedRequestID: gptr.Of(c.RequestID), ExpectedSourceTurnRecords: standardEvalOutputSourceRecordsDTO(records)}
}

func TestSourceStandardFullFreezeAndPostRead(t *testing.T) {
	for _, scenario := range []string{"same", "request_changed", "canonical_record_changed", "physical_ref_changed", "run_changed", "reader_error", "foreign_result", "shared_target_space"} {
		t.Run(scenario, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			result := servicemocks.NewMockExptResultService(ctrl)
			manager := servicemocks.NewMockIExptManager(ctrl)
			target := servicemocks.NewMockIEvalTargetService(ctrl)
			frozen := sourceStandardFixture()
			next := sourceStandardFixture()
			fault := errors.New("primary source unavailable")
			reads := 0
			wrapped := &sourceStandardResultService{MockExptResultService: result, read: func(_ context.Context, space, expt, run, item int64) (*entity.ExptStandardEvalOutputSource, error) {
				require.Equal(t, []int64{1, 2, 3, 4}, []int64{space, expt, run, item})
				reads++
				if reads == 1 {
					return frozen, nil
				}
				if scenario == "reader_error" {
					return nil, fault
				}
				return next, nil
			}}
			app := &experimentApplication{auth: auth, resultSvc: wrapped, manager: manager, evalTargetService: target}
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
			manager.EXPECT().GetDetail(gomock.Any(), int64(2), int64(1), gomock.Any()).Return(makeStandardEvalOutputExpt(2, 1), nil).AnyTimes()
			target.EXPECT().GetEvalTarget(gomock.Any(), int64(200)).Return(&entity.EvalTarget{ID: 200, SpaceID: 1, SourceTargetID: "src-200"}, nil).AnyTimes()
			result.EXPECT().MGetExperimentResult(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *entity.MGetExperimentResultParam) (*entity.MGetExperimentReportResult, error) {
				require.False(t, param.UseAccelerator)
				require.True(t, param.FullTrajectory)
				report := makeStandardEvalOutputReportResult(2, 3, 4, 5, 6)
				p := report.ItemResults[0].TurnResults[0].ExperimentResults[0].Payload
				p.TargetOutput.EvalTargetRecord.SpaceID = 1
				p.EvaluatorOutput.EvaluatorRecords[101].SpaceID = 1
				switch scenario {
				case "request_changed":
					next.Snapshot.Source.Current.RequestID = "rq2"
				case "canonical_record_changed":
					next.Snapshot.Source.CanonicalTurns[0].TargetResultID = 7
					next.Snapshot.Source.Turns[5].TargetResultID = 7
				case "physical_ref_changed":
					next.Snapshot.Source.CurrentRefs[0].ID++
				case "run_changed":
					next.Snapshot.Source.Current.RunID++
				case "foreign_result":
					p.TargetOutput.EvalTargetRecord.ID++
				case "shared_target_space":
					p.TargetOutput.EvalTargetRecord.SpaceID = 19
					p.EvaluatorOutput.EvaluatorRecords[101].SpaceID = 21
				}
				return report, nil
			})
			resp, err := app.MGetExperimentStandardEvalOutputs(context.Background(), sourceStandardExpectedRequest(frozen))
			if scenario == "same" || scenario == "shared_target_space" {
				require.NoError(t, err)
				require.NotNil(t, resp.Items[0].Output)
				require.Equal(t, "", *resp.Items[0].RequestID)
				require.Equal(t, int64(3), resp.Items[0].GetExptRunID())
				require.Equal(t, 2, reads)
			} else {
				require.Nil(t, resp, "mismatch must return zero contents")
				if scenario == "reader_error" {
					require.ErrorIs(t, err, fault)
				} else {
					require.Equal(t, int32(errno.StandardEvalOutputSourceMismatchCode), sourceStandardErrorCode(t, err))
				}
			}
		})
	}
}

func TestSourceStandardSourceOnlyAndFailClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	result := servicemocks.NewMockExptResultService(ctrl)
	auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil).Times(3)
	app := &experimentApplication{auth: auth, resultSvc: result}
	req := sourceStandardExpectedRequest(sourceStandardFixture())
	req.SourceOnly = gptr.Of(true)
	resp, err := app.MGetExperimentStandardEvalOutputs(context.Background(), req)
	require.Nil(t, resp)
	require.Equal(t, int32(errno.StandardEvalOutputSourceNotReadyCode), sourceStandardErrorCode(t, err))
	app.resultSvc = &sourceStandardResultService{MockExptResultService: result, read: func(context.Context, int64, int64, int64, int64) (*entity.ExptStandardEvalOutputSource, error) {
		return sourceStandardFixture(), nil
	}}
	resp, err = app.MGetExperimentStandardEvalOutputs(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	item := resp.Items[0]
	require.Nil(t, item.Output)
	require.Nil(t, item.Detail)
	require.Nil(t, item.Rounds)
	require.Nil(t, item.Agent)
	require.Nil(t, item.Eval)
	require.Nil(t, item.Extra)
	req.ExpectedRequestID = gptr.Of("manual-rq1")
	resp, err = app.MGetExperimentStandardEvalOutputs(context.Background(), req)
	require.Nil(t, resp)
	require.Equal(t, int32(errno.StandardEvalOutputSourceMismatchCode), sourceStandardErrorCode(t, err))
}

func TestSourceStandardStrictExpectedValidation(t *testing.T) {
	for _, scenario := range []string{"partial_run", "partial_request", "empty_records", "nil_turn", "nil_field", "zero_version", "zero_record", "negative", "overflow", "leading_zero", "duplicate_turn", "duplicate_registered", "duplicate_inline", "unsorted_turn", "unsorted_version", "empty_inline", "batch_expected", "zero_item"} {
		t.Run(scenario, func(t *testing.T) {
			req := sourceStandardExpectedRequest(sourceStandardFixture())
			turn := req.ExpectedSourceTurnRecords[0]
			ref := turn.Registered[0]
			switch scenario {
			case "partial_run":
				req.ExpectedRequestID = nil
			case "partial_request":
				req.ExpectedExptRunID = nil
			case "empty_records":
				req.ExpectedSourceTurnRecords = []*exptpb.ItemStandardEvalOutputTurnSource{}
			case "nil_turn":
				req.ExpectedSourceTurnRecords[0] = nil
			case "nil_field":
				turn.TargetRecordID = nil
			case "zero_version":
				ref.VersionID = gptr.Of("0")
			case "zero_record":
				ref.RecordID = gptr.Of("0")
			case "negative":
				turn.TurnID = gptr.Of("-1")
			case "overflow":
				ref.RecordID = gptr.Of("9223372036854775808")
			case "leading_zero":
				ref.RecordID = gptr.Of("010")
			case "duplicate_turn":
				req.ExpectedSourceTurnRecords = append(req.ExpectedSourceTurnRecords, turn)
			case "duplicate_registered":
				turn.Registered = append(turn.Registered, ref)
			case "duplicate_inline":
				turn.Inline = []*exptpb.ItemStandardEvalOutputInlineSource{{InlineKey: gptr.Of("x"), RecordID: gptr.Of("1")}, {InlineKey: gptr.Of("x"), RecordID: gptr.Of("2")}}
			case "unsorted_turn":
				req.ExpectedSourceTurnRecords = append(req.ExpectedSourceTurnRecords, &exptpb.ItemStandardEvalOutputTurnSource{TurnID: gptr.Of("0"), TargetRecordID: gptr.Of("0")})
			case "unsorted_version":
				turn.Registered = append(turn.Registered, &exptpb.ItemStandardEvalOutputRegisteredSource{VersionID: gptr.Of("10"), Alias: gptr.Of(""), RecordID: gptr.Of("2")})
			case "empty_inline":
				turn.Inline = []*exptpb.ItemStandardEvalOutputInlineSource{{InlineKey: gptr.Of(""), RecordID: gptr.Of("1")}}
			case "batch_expected":
				req.ItemIds = []int64{4, 5}
			case "zero_item":
				req.ItemIds = []int64{0}
			}
			_, err := validateStandardEvalOutputSourceRequest(req)
			require.Equal(t, int32(errno.CommonInvalidParamCode), sourceStandardErrorCode(t, err))
		})
	}
	legacy := &exptpb.MGetExperimentStandardEvalOutputsRequest{WorkspaceID: 1, ExptID: 2, ItemIds: []int64{3, 4}}
	expected, err := validateStandardEvalOutputSourceRequest(legacy)
	require.NoError(t, err)
	require.False(t, expected)
	legacy.SourceOnly = gptr.Of(true)
	expected, err = validateStandardEvalOutputSourceRequest(legacy)
	require.NoError(t, err)
	require.False(t, expected, "identity query must never invent legacy expected source")
}

func TestSourceStandardLegacyExactJSON(t *testing.T) {
	req := &exptpb.MGetExperimentStandardEvalOutputsRequest{WorkspaceID: 1, ExptID: 2, ItemIds: []int64{3}}
	raw, err := json.Marshal(req)
	require.NoError(t, err)
	require.Equal(t, `{"workspace_id":1,"expt_id":2,"item_ids":[3]}`, string(raw))
	raw, err = json.Marshal(&exptpb.ItemStandardEvalOutput{})
	require.NoError(t, err)
	require.Equal(t, `{"expt_id":null,"item_id":null,"dataset_key":null,"item_key":null,"status":null,"eval_target_workspace_id":null,"eval_target_id":null,"source_target_id":null,"expt_workspace_id":null,"expt_run_id":null,"dataset_workspace_id":null,"dataset_id":null,"dataset_version_id":null,"dataset_version_name":null,"experiment_group_key":null,"experiment_create_time":null,"item_end_time":null,"created_by":null,"fornax_sandbox_log_url":null,"detail":null,"rounds":null,"agent":null,"output":null,"eval":null,"extra":null}`, string(raw))
}

func sourceStandardErrorCode(t *testing.T, err error) int32 {
	t.Helper()
	status, ok := errorx.FromStatusError(err)
	require.True(t, ok, "expected platform typed business error: %v", err)
	return status.Code()
}
