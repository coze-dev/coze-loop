// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHookReportTargetSaveBeforeResume(t *testing.T) {
	for _, failure := range []string{"none", "save", "publish"} {
		t.Run(failure, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			records := repomocks.NewMockIEvalTargetRepo(ctrl)
			async := repomocks.NewMockIEvalAsyncRepo(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			config := configmocks.NewMockIConfiger(ctrl)
			stored := &entity.EvalTargetRecord{ID: 99, SpaceID: 9, ExperimentRunID: 3, ItemID: 4, TurnID: 0, ItemVersionID: 7, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking)}
			event := &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, EvalSetItemID: 4}
			async.EXPECT().GetEvalAsyncCtx(gomock.Any(), "99").Return(&entity.EvalAsyncCtx{Event: event, AsyncUnixMS: time.Now().UnixMilli(), EnableExtractTrajectory: gptr.Of(false)}, nil)
			records.EXPECT().GetEvalTargetRecordByIDAndSpaceID(gomock.Any(), int64(9), int64(99)).DoAndReturn(func(context.Context, int64, int64) (*entity.EvalTargetRecord, error) {
				copy := *stored
				return &copy, nil
			})
			var order []string
			injected := errors.New("injected " + failure + " failure")
			records.EXPECT().SaveEvalTargetRecord(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rec *entity.EvalTargetRecord, _ *bool) error {
				order = append(order, "save")
				if failure == "save" {
					return injected
				}
				copy := *rec
				stored = &copy
				return nil
			})
			if failure != "save" {
				config.EXPECT().GetTargetTrajectoryConf(gomock.Any()).Return(&entity.TargetTrajectoryConf{})
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), event, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.ExptItemEvalEvent, _ *time.Duration, modify func(*entity.ExptItemEvalEvent)) error {
					require.Equal(t, []string{"save"}, order)
					require.Equal(t, entity.EvalTargetRunStatusSuccess, gptr.Indirect(stored.Status))
					require.NotNil(t, stored.EvalTargetOutputData)
					order = append(order, "publish")
					modify(e)
					assert.True(t, e.AsyncReportTrigger)
					assert.Equal(t, int64(3), e.ExptRunID)
					raw, _ := json.Marshal(e)
					assert.NotContains(t, string(raw), "ActualOutput")
					if failure == "publish" {
						return injected
					}
					return nil
				})
			}
			app := &EvalOpenAPIApplication{targetSvc: service.NewEvalTargetServiceImpl(records, nil, nil, nil, nil, config, nil, nil), asyncRepo: async, publisher: publisher, configer: config}
			_, err := app.ReportEvalTargetInvokeResult_(context.Background(), newSuccessInvokeResultReq(9, 99))
			if failure == "none" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, injected)
			}
			if failure == "save" {
				assert.Equal(t, []string{"save"}, order)
				assert.Equal(t, entity.EvalTargetRunStatusAsyncInvoking, gptr.Indirect(stored.Status))
			} else {
				assert.Equal(t, []string{"save", "publish"}, order)
				assert.Equal(t, entity.EvalTargetRunStatusSuccess, gptr.Indirect(stored.Status))
			}
			assert.Equal(t, int64(7), stored.ItemVersionID)
		})
	}
}

// The service constructor is singleton-backed; this test repo has no test-lifetime mock state.
type hookReportEvaluatorStore struct {
	repo.IEvaluatorRecordRepo
	mu     sync.Mutex
	rows   map[int64]*entity.EvaluatorRecord
	fail   map[int64]bool
	writes map[int64]int
}

var hookReportRecords = &hookReportEvaluatorStore{rows: map[int64]*entity.EvaluatorRecord{}, fail: map[int64]bool{}, writes: map[int64]int{}}
var hookReportSequence atomic.Int64

func (r *hookReportEvaluatorStore) GetEvaluatorRecord(_ context.Context, id int64, _ bool, _ ...entity.GetEvaluatorRecordOptionFn) (*entity.EvaluatorRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row := r.rows[id]
	if row == nil {
		return nil, nil
	}
	copy := *row
	return &copy, nil
}
func (r *hookReportEvaluatorStore) CompareAndSwapEvaluatorRecordResult(_ context.Context, id, space int64, expected, next entity.EvaluatorRunStatus, out *entity.EvaluatorOutputData) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writes[id]++
	if r.fail[id] {
		return false, errors.New("injected save failure")
	}
	row := r.rows[id]
	if row == nil || row.SpaceID != space || row.Status != expected {
		return false, nil
	}
	row.Status = next
	row.EvaluatorOutputData = out
	return true, nil
}

func TestHookReportEvaluatorSaveBeforeResume(t *testing.T) {
	svc := service.NewEvaluatorServiceImpl(nil, nil, nil, nil, hookReportRecords, nil, nil, nil, nil, nil, nil, nil)
	for _, failure := range []string{"none", "save", "publish", "auth"} {
		t.Run(failure, func(t *testing.T) {
			id := hookReportSequence.Add(1)
			hookReportRecords.mu.Lock()
			hookReportRecords.rows[id] = &entity.EvaluatorRecord{ID: id, SpaceID: 9, ExperimentID: 2, ExperimentRunID: 3, ItemID: 4, TurnID: 0, ItemVersionID: 7, Status: entity.EvaluatorRunStatusAsyncInvoking}
			hookReportRecords.fail[id] = failure == "save"
			hookReportRecords.mu.Unlock()
			t.Cleanup(func() {
				hookReportRecords.mu.Lock()
				defer hookReportRecords.mu.Unlock()
				delete(hookReportRecords.rows, id)
				delete(hookReportRecords.fail, id)
				delete(hookReportRecords.writes, id)
			})
			ctrl := gomock.NewController(t)
			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			async := repomocks.NewMockIEvalAsyncRepo(ctrl)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			injected := errors.New("injected " + failure + " failure")
			auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *rpc.AuthorizationParam) error {
				assert.Equal(t, int64(9), p.SpaceID)
				if failure == "auth" {
					return injected
				}
				return nil
			})
			event := &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, EvalSetItemID: 4}
			if failure != "auth" {
				async.EXPECT().GetEvalAsyncCtxStrong(gomock.Any(), gomock.Any()).Return(&entity.EvalAsyncCtx{Event: event, EvaluatorVersionID: 11, ResumeReady: true}, nil)
			}
			if failure == "none" || failure == "publish" {
				publisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), event, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, e *entity.ExptItemEvalEvent, _ *time.Duration, modify func(*entity.ExptItemEvalEvent)) error {
					hookReportRecords.mu.Lock()
					writes := hookReportRecords.writes[id]
					copy := *hookReportRecords.rows[id]
					hookReportRecords.mu.Unlock()
					require.Equal(t, 1, writes)
					require.Equal(t, entity.EvaluatorRunStatusSuccess, copy.Status)
					require.Equal(t, 0.9, gptr.Indirect(copy.EvaluatorOutputData.EvaluatorResult.Score))
					modify(e)
					assert.True(t, e.AsyncEvaluatorReportTrigger)
					assert.Equal(t, int64(3), e.ExptRunID)
					raw, _ := json.Marshal(e)
					assert.NotContains(t, string(raw), "score")
					if failure == "publish" {
						return injected
					}
					return nil
				})
			}
			app := &EvalOpenAPIApplication{auth: auth, asyncRepo: async, evaluatorService: svc, publisher: publisher}
			_, err := app.ReportEvaluatorInvokeResult_(context.Background(), &openapi.ReportEvaluatorInvokeResultRequest{WorkspaceID: gptr.Of(int64(9)), InvokeID: gptr.Of(id), Status: gptr.Of(spi.InvokeEvaluatorRunStatus_SUCCESS), Output: &spi.InvokeEvaluatorOutputData{EvaluatorResult_: &spi.InvokeEvaluatorResult_{Score: gptr.Of(0.9)}}})
			if failure == "none" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			row, _ := hookReportRecords.GetEvaluatorRecord(context.Background(), id, false)
			if failure == "save" || failure == "auth" {
				assert.Equal(t, entity.EvaluatorRunStatusAsyncInvoking, row.Status)
			} else {
				assert.Equal(t, entity.EvaluatorRunStatusSuccess, row.Status)
			}
			assert.Equal(t, int64(7), row.ItemVersionID)
		})
	}
}
