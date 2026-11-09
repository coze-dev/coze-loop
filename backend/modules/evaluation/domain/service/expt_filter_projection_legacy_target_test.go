// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	jsoncodec "github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/stretchr/testify/require"
)

func TestBuildExptFilterProjectionLegacyTargetItemMetadata(t *testing.T) {
	fixture := newProjectionFixture(t)
	fixture.expt.TargetVersionID = 55
	fixture.targets[0].TargetVersionID = 55
	fixture.targets[0].ItemID = 0
	service := legacyTargetService(fixture)
	rows, err := service.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(3), rows[0].ItemID)
	require.Equal(t, "preview", rows[0].EvalTargetData["custom"])
	require.Equal(t, int64(0), fixture.targets[0].ItemID, "projection must not patch source metadata")
}

func TestBuildExptFilterProjectionLegacyTargetUsageMetrics(t *testing.T) {
	f := newProjectionFixture(t)
	f.expt.TargetVersionID = 55
	f.targets[0].TargetVersionID = 55
	f.targets[0].ItemID = 0
	var output entity.EvalTargetOutputData
	require.NoError(t, jsoncodec.Unmarshal([]byte(`{"OutputFields":{"actual_output":{"text":"answer"}},"EvaluatorUsage":{"InputTokens":17,"OutputTokens":23,"TotalTokens":40},"TimeConsumingMS":11}`), &output))
	f.targets[0].EvalTargetOutputData = &output
	s := legacyTargetService(f)
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Equal(t, map[string]int64{"input_tokens": 17, "output_tokens": 23, "total_tokens": 40, "total_latency": 11}, rows[0].EvalTargetMetrics)
	require.Equal(t, "answer", rows[0].EvalTargetData["actual_output"])
}

func TestBuildExptFilterProjectionLegacyTargetRejectsDuplicateOwners(t *testing.T) {
	f := newProjectionFixture(t)
	f.expt.TargetVersionID = 55
	f.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_SingleSet
	f.refs = nil
	f.targets[0].TargetVersionID = 55
	f.targets[0].ItemID = 0
	f.turns = append(f.turns, &entity.ExptTurnResult{ID: 12, SpaceID: 1, ExptID: 2, ExptRunID: 4, ItemID: 4, TargetResultID: 50})
	f.items = append(f.items, &entity.ExptItemResult{ID: 13, SpaceID: 1, ExptID: 2, ItemID: 4, Status: entity.ItemRunState_Success})
	s := legacyTargetService(f)
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3, 4})
	require.ErrorContains(t, err, "referenced by multiple turns")
	require.Empty(t, rows)
}

func TestBuildExptFilterProjectionLegacyTargetOwnershipGuards(t *testing.T) {
	cases := map[string]func(*projectionFixture){
		"positive item mismatch":    func(f *projectionFixture) { f.targets[0].ItemID = 99 },
		"negative item":             func(f *projectionFixture) { f.targets[0].ItemID = -1 },
		"wrong space":               func(f *projectionFixture) { f.targets[0].SpaceID = 99 },
		"wrong turn":                func(f *projectionFixture) { f.targets[0].TurnID = 1 },
		"wrong legacy run":          func(f *projectionFixture) { f.targets[0].ExperimentRunID = 5 },
		"missing target run":        func(f *projectionFixture) { f.targets[0].ExperimentRunID = 0 },
		"missing source run":        func(f *projectionFixture) { f.turns[0].ExptRunID = 0 },
		"missing frozen version":    func(f *projectionFixture) { f.expt.TargetVersionID = 0 },
		"wrong target version":      func(f *projectionFixture) { f.targets[0].TargetVersionID = 56 },
		"duplicate returned record": func(f *projectionFixture) { copy := *f.targets[0]; f.targets = append(f.targets, &copy) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newProjectionFixture(t)
			f.expt.TargetVersionID = 55
			f.targets[0].TargetVersionID = 55
			f.targets[0].ItemID = 0
			change(f)
			s := legacyTargetService(f)
			rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			require.Error(t, err)
			require.Empty(t, rows)
		})
	}
}

func TestBuildExptFilterProjectionModernTargetStillAllowsValidReusedRun(t *testing.T) {
	f := newProjectionFixture(t)
	f.expt.TargetVersionID = 55
	f.targets[0].TargetVersionID = 55
	f.targets[0].ExperimentRunID = 99
	s := legacyTargetService(f)
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Zero(t, s.ExptTurnResultRepo.(*legacyTargetRunLogRepo).calls, "normal positive item metadata must not add runlog reads")
}

type legacyTargetRunLogRepo struct {
	repo.IExptTurnResultRepo
	logs        []*entity.ExptTurnResultRunLog
	err         error
	calls       int
	logsByRun   map[int64][]*entity.ExptTurnResultRunLog
	errorsByRun map[int64]error
	queriedRuns []int64
}

func (r *legacyTargetRunLogRepo) GetItemTurnRunLogs(_ context.Context, expt, run, item, space int64) ([]*entity.ExptTurnResultRunLog, error) {
	r.calls++
	r.queriedRuns = append(r.queriedRuns, run)
	if expt != 2 || item != 3 || space != 1 || (r.logsByRun == nil && run != 4) {
		return nil, fmt.Errorf("wrong runlog identity tuple")
	}
	if r.logsByRun != nil {
		return r.logsByRun[run], r.errorsByRun[run]
	}
	return r.logs, r.err
}
func legacyTargetService(f *projectionFixture) ExptResultServiceImpl {
	s := f.service()
	s.ExptTurnResultRepo = &legacyTargetRunLogRepo{IExptTurnResultRepo: s.ExptTurnResultRepo, logs: []*entity.ExptTurnResultRunLog{{ID: 15, SpaceID: 1, ExptID: 2, ExptRunID: 4, ItemID: 3, TurnID: 0, TargetResultID: 50}}}
	return s
}
func TestBuildExptFilterProjectionLegacyTargetRejectsSingleItemMispoint(t *testing.T) {
	f := newProjectionFixture(t)
	f.expt.TargetVersionID = 55
	f.targets[0].TargetVersionID = 55
	f.targets[0].ItemID = 0
	f.turns[0].TargetResultID = 51
	f.targets[0].ID = 51
	s := legacyTargetService(f)
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.Error(t, err, "the current item runlog still points to target50, not the mis-pointed target51")
	require.Empty(t, rows)
}

func TestBuildExptFilterProjectionLegacyTargetRequiresExactRunlog(t *testing.T) {
	cases := map[string]func(*legacyTargetRunLogRepo){
		"missing":          func(r *legacyTargetRunLogRepo) { r.logs = nil },
		"read failure":     func(r *legacyTargetRunLogRepo) { r.err = errors.New("read unavailable") },
		"wrong space":      func(r *legacyTargetRunLogRepo) { r.logs[0].SpaceID = 9 },
		"wrong experiment": func(r *legacyTargetRunLogRepo) { r.logs[0].ExptID = 99 },
		"old run":          func(r *legacyTargetRunLogRepo) { r.logs[0].ExptRunID = 99 },
		"other item":       func(r *legacyTargetRunLogRepo) { r.logs[0].ItemID = 4 },
		"other turn":       func(r *legacyTargetRunLogRepo) { r.logs[0].TurnID = 1 },
		"other target":     func(r *legacyTargetRunLogRepo) { r.logs[0].TargetResultID = 51 },
		"duplicate anchor": func(r *legacyTargetRunLogRepo) { copy := *r.logs[0]; r.logs = append(r.logs, &copy) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newProjectionFixture(t)
			f.expt.TargetVersionID = 55
			f.targets[0].TargetVersionID = 55
			f.targets[0].ItemID = 0
			s := legacyTargetService(f)
			reader := s.ExptTurnResultRepo.(*legacyTargetRunLogRepo)
			change(reader)
			got, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			require.Error(t, err)
			require.Empty(t, got)
		})
	}
}

func TestBuildExptFilterProjectionLegacyTargetReadsRunlogOncePerItem(t *testing.T) {
	f := newProjectionFixture(t)
	f.expt.TargetVersionID = 55
	f.targets[0].TargetVersionID = 55
	f.targets[0].ItemID = 0
	turn := *f.turns[0]
	turn.ID = 12
	turn.TurnID = 1
	turn.TargetResultID = 51
	f.turns = append(f.turns, &turn)
	target := *f.targets[0]
	target.ID = 51
	target.TurnID = 1
	f.targets = append(f.targets, &target)
	s := legacyTargetService(f)
	reader := s.ExptTurnResultRepo.(*legacyTargetRunLogRepo)
	reader.logs = append(reader.logs, &entity.ExptTurnResultRunLog{ID: 16, SpaceID: 1, ExptID: 2, ExptRunID: 4, ItemID: 3, TurnID: 1, TargetResultID: 51})
	got, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, 1, reader.calls)
}

func legacyTargetReuseFixture(t *testing.T) (*projectionFixture, ExptResultServiceImpl, *legacyTargetRunLogRepo) {
	f := newProjectionFixture(t)
	f.expt.TargetVersionID = 55
	f.targets[0].TargetVersionID = 55
	f.targets[0].ItemID = 0
	f.targets[0].ExperimentRunID = 5
	s := legacyTargetService(f)
	reader := s.ExptTurnResultRepo.(*legacyTargetRunLogRepo)
	original := *reader.logs[0]
	original.ID = 16
	original.ExptRunID = 5
	reader.logsByRun = map[int64][]*entity.ExptTurnResultRunLog{4: reader.logs, 5: {&original}}
	return f, s, reader
}

func TestBuildExptFilterProjectionLegacyTargetReusesOriginalRun(t *testing.T) {
	f, s, reader := legacyTargetReuseFixture(t)
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(3), rows[0].ItemID)
	require.Equal(t, "preview", rows[0].EvalTargetData["custom"])
	require.ElementsMatch(t, []int64{4, 5}, reader.queriedRuns)
	require.Equal(t, int64(0), f.targets[0].ItemID)
	require.Equal(t, int64(5), f.targets[0].ExperimentRunID)
	require.Equal(t, int64(4), f.turns[0].ExptRunID)
}

func TestBuildExptFilterProjectionLegacyTargetReuseRequiresBothRunlogs(t *testing.T) {
	cases := map[string]func(*projectionFixture, *legacyTargetRunLogRepo){
		"changed current reference": func(f *projectionFixture, r *legacyTargetRunLogRepo) {
			f.turns[0].TargetResultID, f.targets[0].ID = 51, 51
		},
		"changed current reference and anchor": func(f *projectionFixture, r *legacyTargetRunLogRepo) {
			f.turns[0].TargetResultID, f.targets[0].ID = 51, 51
			r.logsByRun[4][0].TargetResultID = 51
		},
		"current anchor missing":  func(_ *projectionFixture, r *legacyTargetRunLogRepo) { r.logsByRun[4] = nil },
		"original anchor missing": func(_ *projectionFixture, r *legacyTargetRunLogRepo) { r.logsByRun[5] = nil },
		"original nil anchor": func(_ *projectionFixture, r *legacyTargetRunLogRepo) {
			r.logsByRun[5] = []*entity.ExptTurnResultRunLog{nil}
		},
		"original read error": func(_ *projectionFixture, r *legacyTargetRunLogRepo) {
			r.errorsByRun = map[int64]error{5: errors.New("read unavailable")}
		},
		"original wrong space":      func(_ *projectionFixture, r *legacyTargetRunLogRepo) { r.logsByRun[5][0].SpaceID = 9 },
		"original wrong experiment": func(_ *projectionFixture, r *legacyTargetRunLogRepo) { r.logsByRun[5][0].ExptID = 9 },
		"original wrong run":        func(_ *projectionFixture, r *legacyTargetRunLogRepo) { r.logsByRun[5][0].ExptRunID = 9 },
		"original wrong item":       func(_ *projectionFixture, r *legacyTargetRunLogRepo) { r.logsByRun[5][0].ItemID = 9 },
		"original wrong turn":       func(_ *projectionFixture, r *legacyTargetRunLogRepo) { r.logsByRun[5][0].TurnID = 1 },
		"original wrong target":     func(_ *projectionFixture, r *legacyTargetRunLogRepo) { r.logsByRun[5][0].TargetResultID = 51 },
		"original duplicate anchor": func(_ *projectionFixture, r *legacyTargetRunLogRepo) {
			duplicate := *r.logsByRun[5][0]
			duplicate.ID = 17
			r.logsByRun[5] = append(r.logsByRun[5], &duplicate)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f, s, reader := legacyTargetReuseFixture(t)
			change(f, reader)
			rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			require.Error(t, err)
			require.Empty(t, rows)
		})
	}
}

func TestBuildExptFilterProjectionLegacyTargetReuseCachesBothRuns(t *testing.T) {
	f, s, reader := legacyTargetReuseFixture(t)
	turn := *f.turns[0]
	turn.ID, turn.TurnID, turn.TargetResultID = 12, 1, 51
	f.turns = append(f.turns, &turn)
	target := *f.targets[0]
	target.ID, target.TurnID = 51, 1
	f.targets = append(f.targets, &target)
	for _, run := range []int64{4, 5} {
		anchor := *reader.logsByRun[run][0]
		anchor.ID, anchor.TurnID, anchor.TargetResultID = 20+run, 1, 51
		reader.logsByRun[run] = append(reader.logsByRun[run], &anchor)
	}
	rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.ElementsMatch(t, []int64{4, 5}, reader.queriedRuns)
}
