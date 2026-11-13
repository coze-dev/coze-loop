// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// failingRunScheduleRepo 模拟新表未部署/DB 抖动：任何写入都失败。
type failingRunScheduleRepo struct {
	upsertCalls int
}

func (f *failingRunScheduleRepo) Upsert(context.Context, *entity.ExptRunSchedule) error {
	f.upsertCalls++
	return errors.New("Error 1146 (42S02): Table 'fornax_evaluation.expt_run_schedule' doesn't exist")
}

func (f *failingRunScheduleRepo) Get(context.Context, int64, int64, int64) (*entity.ExptRunSchedule, error) {
	return nil, errors.New("not found")
}

func (f *failingRunScheduleRepo) ScanActive(context.Context, string, int) ([]*entity.ExptRunSchedule, error) {
	return nil, nil
}

func (f *failingRunScheduleRepo) UpdateStatus(context.Context, int64, int64, int64, int32, int64) error {
	return errors.New("table not exist")
}

// 旧实验（legacy）的投影写入失败时，run 创建必须继续成功，不能把旧派发链带崩。
func TestInitRunSchedule_LegacyProjectionFailureDoesNotBlock(t *testing.T) {
	repo := &failingRunScheduleRepo{}
	m := &ExptMangerImpl{scheduleRepo: repo}
	expt := &entity.Experiment{
		ID:       100,
		ExptType: entity.ExptType_Offline,
		EvalConf: &entity.EvaluationConfiguration{ScheduleMode: 0, Priority: 7},
	}

	central, err := m.initRunSchedule(context.Background(), expt, 200, 300)
	require.NoError(t, err)
	require.False(t, central)
	require.Equal(t, 1, repo.upsertCalls, "legacy projection should be attempted best-effort")
}

// 配置关闭（centralEnabled=false）同样走 legacy：投影失败不得返回错误。
func TestInitRunSchedule_CentralDisabledProjectionFailureDoesNotBlock(t *testing.T) {
	repo := &failingRunScheduleRepo{}
	m := &ExptMangerImpl{scheduleRepo: repo}
	expt := &entity.Experiment{
		ID:       101,
		ExptType: entity.ExptType_Offline,
		EvalConf: &entity.EvaluationConfiguration{ScheduleMode: 1, Priority: 9},
	}

	central, err := m.initRunSchedule(context.Background(), expt, 201, 301)
	require.NoError(t, err)
	require.False(t, central)
	require.Equal(t, 1, repo.upsertCalls)
}

// 旧实验无投影依赖：连 repo 都没有时也必须成功。
func TestInitRunSchedule_LegacyWithoutRepoReturnsSuccess(t *testing.T) {
	m := &ExptMangerImpl{}
	expt := &entity.Experiment{ID: 102, ExptType: entity.ExptType_Offline}

	central, err := m.initRunSchedule(context.Background(), expt, 202, 302)
	require.NoError(t, err)
	require.False(t, central)
}
