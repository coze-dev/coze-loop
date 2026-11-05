// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/stretchr/testify/require"
)

type projectionSourceTargetRepo struct {
	repo.IEvalTargetRepo
	f *projectionFixture
}

func TestProjectionFreezesBusinessDateInDeploymentTimezone(t *testing.T) {
	midnight := time.Date(2025, 1, 2, 0, 30, 0, 0, time.Local)
	_, offset := midnight.Zone()
	f := newProjectionFixture(t)
	start := midnight.In(time.FixedZone("source-zone", offset-2*60*60))
	f.expt.StartAt = &start
	rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Equal(t, "2025-01-02", rows[0].CreatedDate.Format("2006-01-02"))
	require.Equal(t, "2025-01-01", f.expt.StartAt.Format("2006-01-02"), "source experiment must remain unchanged")
}

func TestProjectionReadsPersistedCrossSpaceItemReference(t *testing.T) {
	f := newProjectionFixture(t)
	converter := convert.NewExptItemRefConvertor()
	f.refs[0] = converter.PO2DO(converter.DO2PO(f.refs[0]))
	require.Zero(t, f.refs[0].EvalSetSourceSpaceID)
	rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.NoError(t, err)
	require.Equal(t, int64(30), rows[0].EvalSetID)
	require.Zero(t, f.refs[0].EvalSetSourceSpaceID, "builder must not mutate persisted source")
}

func (r projectionSourceTargetRepo) ListEvalTargetRecordByIDsAndSpaceID(ctx context.Context, space int64, ids []int64) ([]*entity.EvalTargetRecord, error) {
	return (projectionTargetService{f: r.f}).BatchGetRecordByIDs(ctx, space, ids)
}

type projectionSourceEvaluatorRepo struct {
	repo.IEvaluatorRecordRepo
	f *projectionFixture
}

func (r projectionSourceEvaluatorRepo) BatchGetEvaluatorRecord(ctx context.Context, ids []int64, deleted, full bool, _ ...entity.GetEvaluatorRecordOptionFn) ([]*entity.EvaluatorRecord, error) {
	return (projectionEvaluatorService{f: r.f}).BatchGetEvaluatorRecord(ctx, ids, deleted, full)
}

func TestProjectionMaintenanceConstructorUsesOnlyReadRepositories(t *testing.T) {
	f := newProjectionFixture(t)
	old := f.service()
	builder := NewExptFilterProjectionBuilder(old.ExperimentRepo, old.ExptItemResultRepo, old.ExptTurnResultRepo, old.ExptAnnotateRepo, old.exptTurnResultFilterRepo, old.exptItemRefRepo, projectionSourceTargetRepo{f: f}, projectionSourceEvaluatorRepo{f: f}, nil)
	rows, err := builder.BuildExptFilterProjection(context.Background(), f.expt.SpaceID, f.expt.ID, []int64{f.turns[0].ItemID})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.Contains(t, rows[0].EvaluatorScore, "key11")
}
