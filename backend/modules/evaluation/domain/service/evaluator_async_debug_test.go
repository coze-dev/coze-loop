// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestEvaluatorServiceImpl_AsyncDebugEvaluator_SharedCoordinator(t *testing.T) {
	for _, evaluatorType := range []entity.EvaluatorType{entity.EvaluatorTypeAgent, entity.EvaluatorTypeCustomRPC} {
		for _, scenario := range []string{"nonempty metadata", "trace only", "ext only", "empty metadata", "provider failure with metadata", "provider failure metadata error", "cas failure", "reload failure", "reload nonterminal", "record failure", "context failure", "context repo missing", "provider failure", "success before ack failure", "failure before ack failure", "metadata failure", "rate limited", "id failure", "source missing"} {
			t.Run(fmt.Sprintf("%v/%s", evaluatorType, scenario), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				limiter := repomocks.NewMockRateLimiter(ctrl)
				idGenerator := idgenmocks.NewMockIIDGenerator(ctrl)
				recordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
				asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
				source := mocks.NewMockEvaluatorSourceService(ctrl)
				s := &EvaluatorServiceImpl{
					limiter: limiter, idgen: idGenerator, evaluatorRecordRepo: recordRepo, evalAsyncRepo: asyncRepo,
					evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{evaluatorType: source},
				}
				req := customRPCAsyncDebugRequest()
				if evaluatorType == entity.EvaluatorTypeAgent {
					req.EvaluatorDO.EvaluatorType = evaluatorType
					req.EvaluatorDO.AgentEvaluatorVersion = &entity.AgentEvaluatorVersion{ModelConfig: &entity.ModelConfig{ModelID: gptr.Of(int64(1))}}
					req.EvaluatorDO.CustomRPCEvaluatorVersion = nil
				}
				req.InputData.EvaluateDatasetFields = map[string]*entity.Content{"dataset": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("dataset input")}}
				req.InputData.EvaluateTargetOutputFields = map[string]*entity.Content{"target": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("target output")}}
				req.EvaluatorRunConf = &entity.EvaluatorRunConfig{}
				if scenario == "context repo missing" {
					s.evalAsyncRepo = nil
				}

				if scenario == "source missing" {
					delete(s.evaluatorSourceServices, evaluatorType)
				}
				ctx := session.WithCtxUser(context.Background(), &session.User{ID: "debug-user"})
				if scenario != "source missing" {
					source.EXPECT().Validate(gomock.Any(), req.EvaluatorDO).Return(nil)
					limiter.EXPECT().AllowInvoke(gomock.Any(), int64(2)).Return(scenario != "rate limited")
				}
				if scenario != "source missing" && scenario != "rate limited" {
					var idErr error
					if scenario == "id failure" {
						idErr = errors.New("id failure")
					}
					idGenerator.EXPECT().GenID(gomock.Any()).Return(int64(999), idErr)
				}
				if scenario != "source missing" && scenario != "rate limited" && scenario != "id failure" {
					var saved *entity.EvaluatorRecord
					var savedCtx *entity.EvalAsyncCtx
					recordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, record *entity.EvaluatorRecord) error {
						saved = record
						assert.Equal(t, int64(999), record.ID)
						assert.Equal(t, int64(2), record.SpaceID)
						assert.Zero(t, record.EvaluatorVersionID)
						assert.Zero(t, record.ExperimentID)
						assert.Zero(t, record.ExperimentRunID)
						assert.NotEqual(t, entity.EvaluatorRecordSourceTypeInline, record.SourceType)
						assert.Equal(t, entity.EvaluatorRunStatusAsyncInvoking, record.Status)
						assert.Equal(t, "debug-user", gptr.Indirect(record.BaseInfo.CreatedBy.UserID))
						require.NotSame(t, req.InputData, record.EvaluatorInputData)
						record.EvaluatorInputData.InputFields["answer"].SetText("truncated preview")
						record.EvaluatorInputData.EvaluateDatasetFields["dataset"].SetText("truncated dataset")
						record.EvaluatorInputData.EvaluateTargetOutputFields["target"].SetText("truncated target")
						if scenario == "record failure" {
							return errors.New("record failure")
						}
						return nil
					})
					if scenario != "record failure" && scenario != "context repo missing" {
						asyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), "evaluator:999", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, actx *entity.EvalAsyncCtx) error {
							require.NotNil(t, saved, "record must exist before context")
							savedCtx = actx
							assert.Equal(t, int64(999), actx.RecordID)
							assert.Zero(t, actx.EvaluatorVersionID)
							assert.Nil(t, actx.Event)
							assert.Empty(t, actx.CallbackURL)
							assert.True(t, actx.ResumeReady)
							assert.Positive(t, actx.AsyncUnixMS)
							assert.Equal(t, gptr.Indirect(saved.BaseInfo.CreatedAt), actx.AsyncUnixMS)
							assert.Equal(t, "debug-user", actx.Session.UserID)
							if scenario == "context failure" {
								return errors.New("context failure")
							}
							return nil
						})
					}
					callbackBeforeAck := scenario == "success before ack failure" || scenario == "failure before ack failure"
					if scenario != "record failure" && scenario != "context failure" && scenario != "context repo missing" {
						source.EXPECT().AsyncDebug(gomock.Any(), req.EvaluatorDO, req.InputData, req.EvaluatorRunConf, int64(2), int64(999)).DoAndReturn(
							func(ctx context.Context, _ *entity.Evaluator, input *entity.EvaluatorInputData, _ *entity.EvaluatorRunConfig, _, _ int64) (map[string]string, string, error) {
								require.NotNil(t, savedCtx, "callback context must exist before provider dispatch")
								assert.Equal(t, "complete input", input.InputFields["answer"].GetText())
								assert.Equal(t, "dataset input", input.EvaluateDatasetFields["dataset"].GetText())
								assert.Equal(t, "target output", input.EvaluateTargetOutputFields["target"].GetText())
								input.Ext["runtime_param"] = "unfolded"
								assert.Equal(t, "original", saved.EvaluatorInputData.Ext["runtime_param"])
								if callbackBeforeAck {
									status := entity.EvaluatorRunStatusSuccess
									output := &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(0.0)}}
									if scenario == "failure before ack failure" {
										status = entity.EvaluatorRunStatusFail
										output = &entity.EvaluatorOutputData{EvaluatorRunError: &entity.EvaluatorRunError{Code: errno.CommonInternalErrorCode, Message: "provider reported failure"}}
									}
									outcome, err := s.ReportEvaluatorInvokeResult(ctx, &entity.ReportEvaluatorRecordParam{RecordID: 999, SpaceID: 2, Status: status, OutputData: output})
									require.NoError(t, err)
									assert.Equal(t, entity.ReportEvaluatorResultApplied, outcome)
								}
								if scenario == "provider failure with metadata" || scenario == "provider failure metadata error" {
									return map[string]string{"dispatch": "failed"}, "trace-error", errors.New("provider failure")
								}
								if scenario == "empty metadata" {
									return nil, "", nil
								}
								if callbackBeforeAck {
									return map[string]string{"sandbox_session_id": "debug-session"}, "trace-callback", errors.New("provider failure")
								}
								if scenario == "provider failure" || scenario == "cas failure" || scenario == "reload failure" || scenario == "reload nonterminal" {
									return nil, "", errors.New("provider failure")
								}
								if scenario == "trace only" {
									return nil, "trace-debug", nil
								}
								if scenario == "ext only" {
									return map[string]string{"sandbox_session_id": "debug-session"}, "", nil
								}
								return map[string]string{"dispatch": "accepted", "sandbox_session_id": "debug-session"}, "trace-debug", nil
							})
					}
					if callbackBeforeAck {
						recordRepo.EXPECT().UpdateEvaluatorRecordAsyncDispatch(gomock.Any(), int64(999), int64(2), "trace-callback", gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, traceID string, output *entity.EvaluatorOutputData) error {
							assert.NotEqual(t, entity.EvaluatorRunStatusAsyncInvoking, saved.Status)
							assert.Equal(t, "debug-session", output.Ext["sandbox_session_id"])
							saved.TraceID = traceID
							return nil
						})
						recordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(999), false).DoAndReturn(func(ctx context.Context, _ int64, _ bool, _ ...entity.GetEvaluatorRecordOptionFn) (*entity.EvaluatorRecord, error) {
							assert.True(t, contexts.CtxWriteDB(ctx))
							return saved, nil
						})
						terminalStatus := entity.EvaluatorRunStatusSuccess
						if scenario == "failure before ack failure" {
							terminalStatus = entity.EvaluatorRunStatusFail
						}
						recordRepo.EXPECT().CompareAndSwapEvaluatorRecordResult(gomock.Any(), int64(999), int64(2), entity.EvaluatorRunStatusAsyncInvoking, terminalStatus, gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, _, status entity.EvaluatorRunStatus, output *entity.EvaluatorOutputData) (bool, error) {
							saved.Status, saved.EvaluatorOutputData = status, output
							return true, nil
						})
						recordRepo.EXPECT().CompareAndSwapEvaluatorRecordResult(gomock.Any(), int64(999), int64(2), entity.EvaluatorRunStatusAsyncInvoking, entity.EvaluatorRunStatusFail, gomock.Any()).Return(false, nil)
						recordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(999), false, gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, _ bool, _ ...entity.GetEvaluatorRecordOptionFn) (*entity.EvaluatorRecord, error) {
							assert.True(t, contexts.CtxWriteDB(ctx))
							assert.Equal(t, terminalStatus, saved.Status)
							return saved, nil
						})
					} else if scenario == "provider failure with metadata" || scenario == "provider failure metadata error" {
						var metadataErr error
						if scenario == "provider failure metadata error" {
							metadataErr = errors.New("metadata failure")
						}
						metadata := recordRepo.EXPECT().UpdateEvaluatorRecordAsyncDispatch(gomock.Any(), int64(999), int64(2), "trace-error", gomock.Any()).Return(metadataErr)
						recordRepo.EXPECT().CompareAndSwapEvaluatorRecordResult(gomock.Any(), int64(999), int64(2), entity.EvaluatorRunStatusAsyncInvoking, entity.EvaluatorRunStatusFail, gomock.Any()).After(metadata).Return(true, nil)
					} else if scenario == "cas failure" {
						recordRepo.EXPECT().CompareAndSwapEvaluatorRecordResult(gomock.Any(), int64(999), int64(2), entity.EvaluatorRunStatusAsyncInvoking, entity.EvaluatorRunStatusFail, gomock.Any()).Return(false, errors.New("cas failure"))
					} else if scenario == "reload failure" || scenario == "reload nonterminal" {
						recordRepo.EXPECT().CompareAndSwapEvaluatorRecordResult(gomock.Any(), int64(999), int64(2), entity.EvaluatorRunStatusAsyncInvoking, entity.EvaluatorRunStatusFail, gomock.Any()).Return(false, nil)
						recordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(999), false, gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, _ bool, _ ...entity.GetEvaluatorRecordOptionFn) (*entity.EvaluatorRecord, error) {
							assert.True(t, contexts.CtxWriteDB(ctx))
							if scenario == "reload failure" {
								return nil, errors.New("reload failure")
							}
							return saved, nil
						})
					} else if scenario == "context failure" || scenario == "context repo missing" || scenario == "provider failure" {
						recordRepo.EXPECT().CompareAndSwapEvaluatorRecordResult(gomock.Any(), int64(999), int64(2), entity.EvaluatorRunStatusAsyncInvoking, entity.EvaluatorRunStatusFail, gomock.Any()).Return(true, nil)
					} else if scenario != "record failure" && scenario != "empty metadata" {
						var metadataErr error
						if scenario == "metadata failure" {
							metadataErr = errors.New("metadata failure")
						}
						traceID := "trace-debug"
						if scenario == "ext only" {
							traceID = ""
						}
						recordRepo.EXPECT().UpdateEvaluatorRecordAsyncDispatch(gomock.Any(), int64(999), int64(2), traceID, gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, _ string, output *entity.EvaluatorOutputData) error {
							if scenario == "trace only" {
								assert.Empty(t, output.Ext)
							} else {
								assert.Equal(t, "debug-session", output.Ext["sandbox_session_id"])
							}
							return metadataErr
						})
						if scenario == "metadata failure" {
							recordRepo.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(999), false).DoAndReturn(func(ctx context.Context, _ int64, _ bool, _ ...entity.GetEvaluatorRecordOptionFn) (*entity.EvaluatorRecord, error) {
								assert.True(t, contexts.CtxWriteDB(ctx))
								assert.Equal(t, entity.EvaluatorRunStatusAsyncInvoking, saved.Status)
								return saved, nil
							})
							recordRepo.EXPECT().CompareAndSwapEvaluatorRecordResult(gomock.Any(), int64(999), int64(2), entity.EvaluatorRunStatusAsyncInvoking, entity.EvaluatorRunStatusSuccess, gomock.Any()).Return(true, nil)
						}
					}
				}
				resp, err := s.AsyncDebugEvaluator(ctx, req)
				switch scenario {
				case "nonempty metadata", "trace only", "ext only", "empty metadata", "metadata failure", "success before ack failure", "failure before ack failure":
					require.NoError(t, err)
					require.NotNil(t, resp)
					assert.Equal(t, int64(999), resp.InvokeID)
					if scenario == "success before ack failure" || scenario == "failure before ack failure" {
						assert.Equal(t, "trace-callback", resp.TraceID)
					}
					if scenario == "metadata failure" {
						outcome, callbackErr := s.ReportEvaluatorInvokeResult(ctx, &entity.ReportEvaluatorRecordParam{RecordID: 999, SpaceID: 2, Status: entity.EvaluatorRunStatusSuccess, OutputData: &entity.EvaluatorOutputData{EvaluatorResult: &entity.EvaluatorResult{Score: gptr.Of(1.0)}}})
						require.NoError(t, callbackErr)
						assert.Equal(t, entity.ReportEvaluatorResultApplied, outcome)
					}
				default:
					require.Error(t, err)
					assert.Nil(t, resp)
					if scenario == "provider failure metadata error" {
						assert.Contains(t, err.Error(), "provider failure")
						assert.NotContains(t, err.Error(), "metadata failure")
					}
					if scenario == "rate limited" {
						statusErr, ok := errorx.FromStatusError(err)
						require.True(t, ok)
						assert.Equal(t, int32(errno.EvaluatorQPSLimitCode), statusErr.Code())
					}
				}
			})
		}
	}
}

func TestEvaluatorServiceImpl_AsyncDebugEvaluator_InvalidRequestNeverDispatches(t *testing.T) {
	for _, evaluatorType := range []entity.EvaluatorType{entity.EvaluatorTypeAgent, entity.EvaluatorTypeCustomRPC} {
		for _, scenario := range []string{"nil request", "nil evaluator", "unsupported", "cross space", "nil input", "invalid schema", "nil schema", "missing version", "source validation"} {
			t.Run(fmt.Sprintf("%v/%s", evaluatorType, scenario), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				source := mocks.NewMockEvaluatorSourceService(ctrl)
				svc := &EvaluatorServiceImpl{
					limiter: repomocks.NewMockRateLimiter(ctrl), idgen: idgenmocks.NewMockIIDGenerator(ctrl),
					evaluatorRecordRepo: repomocks.NewMockIEvaluatorRecordRepo(ctrl), evalAsyncRepo: repomocks.NewMockIEvalAsyncRepo(ctrl),
					evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{evaluatorType: source},
				}
				req := customRPCAsyncDebugRequest()
				if evaluatorType == entity.EvaluatorTypeAgent {
					req.EvaluatorDO.EvaluatorType = evaluatorType
					req.EvaluatorDO.AgentEvaluatorVersion = &entity.AgentEvaluatorVersion{ModelConfig: &entity.ModelConfig{ModelID: gptr.Of(int64(1))}}
					req.EvaluatorDO.CustomRPCEvaluatorVersion = nil
				}
				code := int32(errno.InvalidEvaluatorConfigurationCode)
				switch scenario {
				case "nil request":
					req = nil
					code = errno.CommonInvalidParamCode
				case "nil evaluator":
					req.EvaluatorDO = nil
					code = errno.EvaluatorNotExistCode
				case "unsupported":
					req.EvaluatorDO.EvaluatorType = entity.EvaluatorTypeCode
					code = errno.InvalidEvaluatorTypeCode
				case "cross space":
					req.EvaluatorDO.SpaceID = 3
					code = errno.EvaluatorVersionNotFoundCode
				case "nil input":
					req.InputData = nil
					code = errno.InvalidInputDataCode
				case "invalid schema", "nil schema":
					schemas := []*entity.ArgsSchema{{Key: gptr.Of("answer"), SupportContentTypes: []entity.ContentType{entity.ContentTypeText}, JsonSchema: gptr.Of(`{"type":"integer"}`)}}
					code = errno.ContentSchemaInvalidCode
					if scenario == "nil schema" {
						schemas = []*entity.ArgsSchema{nil}
						code = errno.InvalidEvaluatorConfigurationCode
					}
					if evaluatorType == entity.EvaluatorTypeAgent {
						req.EvaluatorDO.AgentEvaluatorVersion.InputSchemas = schemas
					} else {
						req.EvaluatorDO.CustomRPCEvaluatorVersion.InputSchemas = schemas
					}
				case "missing version":
					req.EvaluatorDO.AgentEvaluatorVersion = nil
					req.EvaluatorDO.CustomRPCEvaluatorVersion = nil
					if evaluatorType == entity.EvaluatorTypeAgent {
						source.EXPECT().Validate(gomock.Any(), req.EvaluatorDO).Return(errorx.NewByCode(code))
					}
				case "source validation":
					source.EXPECT().Validate(gomock.Any(), req.EvaluatorDO).Return(errorx.NewByCode(code))
				}
				resp, err := svc.AsyncDebugEvaluator(context.Background(), req)
				require.Error(t, err)
				require.Nil(t, resp)
				statusErr, ok := errorx.FromStatusError(err)
				require.True(t, ok)
				assert.Equal(t, code, statusErr.Code())
			})
		}
	}
}
