// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/bytedance/gg/gptr"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
)

type lifecycleStore struct {
	target           *entity.EvalTarget
	beforeTransition func()

	repo.IEvalTargetRepo
	records        map[int64]*entity.EvalTargetRecord
	active         bool
	failCreate     bool
	failTransition bool
	checkedSpace   int64
}

func copyLifecycleRecord(r *entity.EvalTargetRecord) *entity.EvalTargetRecord {
	if r == nil {
		return nil
	}
	b, _ := json.Marshal(r)
	var out entity.EvalTargetRecord
	_ = json.Unmarshal(b, &out)
	return &out
}
func (s *lifecycleStore) LatestSandboxInvocation(_ context.Context, space, run, item, turn int64) (*entity.EvalTargetRecord, error) {
	var result *entity.EvalTargetRecord
	for _, r := range s.records {
		if r.SpaceID == space && r.ExperimentRunID == run && r.ItemID == item && r.TurnID == turn && (result == nil || r.ID > result.ID) {
			result = r
		}
	}
	return copyLifecycleRecord(result), nil
}
func (s *lifecycleStore) SandboxInvocationOwnerActive(_ context.Context, space, expt, run, item int64) (bool, error) {
	s.checkedSpace = space
	return s.active, nil
}
func (s *lifecycleStore) CreateEvalTargetRecord(_ context.Context, r *entity.EvalTargetRecord, _ *bool) (int64, error) {
	if s.failCreate {
		return 0, errors.New("database unavailable")
	}
	s.records[r.ID] = copyLifecycleRecord(r)
	return r.ID, nil
}
func (s *lifecycleStore) GetEvalTargetRecordByIDAndSpaceID(_ context.Context, space, id int64) (*entity.EvalTargetRecord, error) {
	return copyLifecycleRecord(s.records[id]), nil
}
func (s *lifecycleStore) TransitionSandboxInvocation(_ context.Context, r *entity.EvalTargetRecord, phase string) (bool, error) {
	if s.beforeTransition != nil {
		fn := s.beforeTransition
		s.beforeTransition = nil
		fn()
	}
	old := s.records[r.ID]
	if s.failTransition || old == nil || gptr.Indirect(old.Status) != entity.EvalTargetRunStatusAsyncInvoking || old.Ext["sandbox_lifecycle_phase"] != phase {
		return false, nil
	}
	s.records[r.ID] = copyLifecycleRecord(r)
	return true, nil
}
func newLifecycleFixture() (*sandboxInvocationLifecycle, *lifecycleStore) {
	store := &lifecycleStore{records: map[int64]*entity.EvalTargetRecord{}, active: true}
	svc := &EvalTargetServiceImpl{evalTargetRepo: store, sandboxLeasePendingRegistrar: func(context.Context, *entity.EvalTargetRecord, int64, int64) error { return nil }}
	return &sandboxInvocationLifecycle{service: svc, spaceID: 10, target: &entity.EvalTarget{ID: 20, EvalTargetVersion: &entity.EvalTargetVersion{ID: 21}}, param: &entity.ExecuteTargetCtx{ExperimentID: gptr.Of(int64(30)), ExperimentRunID: gptr.Of(int64(31)), ItemID: 40, TurnID: 41, ExptSpaceID: 99}, input: &entity.EvalTargetInputData{}}, store
}
func TestSandboxLifecycleRegistersBeforeDispatch(t *testing.T) {
	l, s := newLifecycleFixture()
	registered := false
	l.service.sandboxLeasePendingRegistrar = func(_ context.Context, r *entity.EvalTargetRecord, expt, space int64) error {
		require.NotNil(t, s.records[r.ID])
		require.Equal(t, int64(10), space)
		require.Equal(t, int64(10), r.SpaceID)
		registered = true
		return nil
	}
	id, err := l.prepare(context.Background(), 50, []string{"50-windows", "50-orch"})
	require.NoError(t, err)
	require.Zero(t, id)
	require.True(t, registered)
	require.Equal(t, int64(99), s.checkedSpace)
	require.Equal(t, "initializing", s.records[50].Ext["sandbox_lifecycle_phase"])
	require.NoError(t, l.finish(context.Background()))
	require.Equal(t, "ready", s.records[50].Ext["sandbox_lifecycle_phase"])
}
func TestSandboxLifecycleFailClosedBeforeDispatch(t *testing.T) {
	for _, mode := range []string{"database", "queue", "cancelled", "lock-lost"} {
		t.Run(mode, func(t *testing.T) {
			l, s := newLifecycleFixture()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "database":
				s.failCreate = true
			case "queue":
				l.service.sandboxLeasePendingRegistrar = func(context.Context, *entity.EvalTargetRecord, int64, int64) error { return errors.New("queue down") }
			case "cancelled":
				s.active = false
			case "lock-lost":
				cancel()
			}
			_, err := l.prepare(ctx, 50, []string{"50-windows", "50-orch"})
			require.Error(t, err)
		})
	}
}
func TestSandboxLifecycleFencesInterruptedAttempt(t *testing.T) {
	old, s := newLifecycleFixture()
	_, err := old.prepare(context.Background(), 50, []string{"50-windows", "50-orch"})
	require.NoError(t, err)
	next := *old
	next.record = nil
	_, err = next.prepare(context.Background(), 51, []string{"51-windows", "51-orch"})
	require.NoError(t, err)
	require.Equal(t, entity.EvalTargetRunStatusFail, *s.records[50].Status)
	require.Equal(t, entity.EvalTargetRunStatusAsyncInvoking, *s.records[51].Status)
	require.Error(t, old.check(context.Background()))
	require.Error(t, old.finish(context.Background()))
	require.Equal(t, "[\"50-windows\",\"50-orch\"]", s.records[50].Ext["sandbox_lifecycle_execute_ids"])
}
func TestSandboxLifecycleResumesReadyWithoutNewAllocation(t *testing.T) {
	l, s := newLifecycleFixture()
	_, err := l.prepare(context.Background(), 50, []string{"50-windows", "50-orch"})
	require.NoError(t, err)
	require.NoError(t, l.finish(context.Background()))
	next := *l
	next.record = nil
	id, err := next.prepare(context.Background(), 51, []string{"51-windows", "51-orch"})
	require.NoError(t, err)
	require.Equal(t, int64(50), id)
	require.Len(t, s.records, 1)
}
func TestSandboxLifecycleCancelCannotBeResurrected(t *testing.T) {
	l, s := newLifecycleFixture()
	_, err := l.prepare(context.Background(), 50, []string{"50-windows", "50-orch"})
	require.NoError(t, err)
	s.records[50].Status = gptr.Of(entity.EvalTargetRunStatusFail)
	s.records[50].Ext["sandbox_lifecycle_phase"] = "aborted"
	require.Error(t, l.finish(context.Background()))
	require.Equal(t, entity.EvalTargetRunStatusFail, *s.records[50].Status)
}
func TestSandboxLifecycleRejectsMismatchedOwner(t *testing.T) {
	l, s := newLifecycleFixture()
	_, err := l.prepare(context.Background(), 50, []string{"50-windows", "50-orch"})
	require.NoError(t, err)
	s.records[50].TargetVersionID = 999
	_, err = l.prepare(context.Background(), 51, []string{"51-windows", "51-orch"})
	require.ErrorContains(t, err, "owner mismatch")
	require.Len(t, s.records, 1)
}

func TestSandboxLifecycleKeepsReportedResult(t *testing.T) {
	for _, status := range []entity.EvalTargetRunStatus{entity.EvalTargetRunStatusSuccess, entity.EvalTargetRunStatusFail} {
		t.Run(string(rune(status)), func(t *testing.T) {
			l, s := newLifecycleFixture()
			_, err := l.prepare(context.Background(), 50, []string{"50-windows", "50-orch"})
			require.NoError(t, err)
			s.records[50].Status = gptr.Of(status)
			s.records[50].EvalTargetOutputData.OutputFields = map[string]*entity.Content{"answer": {Text: gptr.Of("reported")}}
			require.NoError(t, l.finish(context.Background()))
			require.Equal(t, status, *l.record.Status)
			require.Equal(t, "reported", *l.record.EvalTargetOutputData.OutputFields["answer"].Text)
			next := *l
			next.record = nil
			id, err := next.prepare(context.Background(), 51, []string{"51-windows", "51-orch"})
			require.NoError(t, err)
			require.Equal(t, int64(50), id)
			require.Len(t, s.records, 1)
		})
	}
}

func (s *lifecycleStore) GetEvalTargetVersion(context.Context, int64, int64) (*entity.EvalTarget, error) {
	return s.target, nil
}
func (s *lifecycleStore) SaveEvalTargetRecord(_ context.Context, r *entity.EvalTargetRecord, _ *bool) error {
	s.records[r.ID] = copyLifecycleRecord(r)
	return nil
}

type lifecycleOperator struct {
	ISourceEvalTargetOperateService
	run func(context.Context, int64, *entity.ExecuteEvalTargetParam) (int64, string, map[string]string, error)
}

func (o lifecycleOperator) ValidateInput(context.Context, int64, []*entity.ArgsSchema, *entity.EvalTargetInputData) error {
	return nil
}
func (o lifecycleOperator) AsyncExecute(ctx context.Context, space int64, p *entity.ExecuteEvalTargetParam) (int64, string, map[string]string, error) {
	return o.run(ctx, space, p)
}

func TestSandboxLifecycleAsyncExecuteAcceptsConcurrentReport(t *testing.T) {
	for _, at := range []string{"before_finish", "during_ready_cas"} {
		t.Run(at, func(t *testing.T) {
			l, store := newLifecycleFixture()
			svc := l.service
			l.target.EvalTargetType = entity.EvalTargetTypeSandboxAgent
			store.target = l.target
			metric := metricsmocks.NewMockEvalTargetMetrics(gomock.NewController(t))
			metric.EXPECT().EmitRun(gomock.Any(), gomock.Any(), gomock.Any())
			svc.metric = metric
			report := func() {
				require.NoError(t, svc.ReportInvokeRecords(context.Background(), &entity.ReportTargetRecordParam{SpaceID: 10, RecordID: 50, Status: entity.EvalTargetRunStatusSuccess, OutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{"answer": {Text: gptr.Of("complete")}}}, SkipErrMsgConvert: true, EnableExtractTrajectory: gptr.Of(false)}))
			}
			svc.typedOperators = map[entity.EvalTargetType]ISourceEvalTargetOperateService{entity.EvalTargetTypeSandboxAgent: lifecycleOperator{run: func(ctx context.Context, space int64, p *entity.ExecuteEvalTargetParam) (int64, string, map[string]string, error) {
				_, err := p.PrepareSandboxExecution(ctx, 50, []string{"50-windows", "50-orch"})
				require.NoError(t, err)
				if at == "before_finish" {
					report()
				} else {
					store.beforeTransition = report
				}
				return 50, "sandbox_agent", map[string]string{"sandbox_execute_ids": "[\"50-windows\",\"50-orch\"]"}, nil
			}}}
			record, _, err := svc.AsyncExecuteTarget(context.Background(), 10, l.target.ID, l.target.EvalTargetVersion.ID, l.param, l.input)
			require.NoError(t, err)
			require.NotNil(t, record)
			require.Equal(t, entity.EvalTargetRunStatusSuccess, *record.Status)
			require.Equal(t, "complete", *record.EvalTargetOutputData.OutputFields["answer"].Text)
			require.Equal(t, entity.EvalTargetRunStatusSuccess, *store.records[50].Status)
			require.Equal(t, store.records[50].TraceID, record.TraceID)
		})
	}
}
