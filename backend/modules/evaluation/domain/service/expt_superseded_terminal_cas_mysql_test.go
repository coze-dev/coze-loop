// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

type terminalRaceRunLogRepo struct {
	repo.IExptRunLogRepo
	reads          sync.WaitGroup
	successWritten chan struct{}
}

func (r *terminalRaceRunLogRepo) Get(ctx context.Context, exptID, exptRunID int64) (*entity.ExptRunLog, error) {
	run, err := r.IExptRunLogRepo.Get(ctx, exptID, exptRunID)
	if err == nil && run != nil && !entity.IsExptFinished(entity.ExptStatus(run.Status)) {
		r.reads.Done()
		r.reads.Wait()
	}
	return run, err
}

func (r *terminalRaceRunLogRepo) Update(ctx context.Context, exptID, exptRunID, spaceID int64, fields map[string]any) error {
	status := fields["status"].(int64)
	if entity.ExptStatus(status) == entity.ExptStatus_Terminated {
		<-r.successWritten
	}
	err := r.IExptRunLogRepo.Update(ctx, exptID, exptRunID, spaceID, fields)
	if entity.ExptStatus(status) == entity.ExptStatus_Success {
		close(r.successWritten)
	}
	return err
}

func (r *terminalRaceRunLogRepo) UpdateStatusIfCurrent(ctx context.Context, exptID, exptRunID, spaceID, expectedStatus, status int64) (bool, error) {
	updater, ok := r.IExptRunLogRepo.(interface {
		UpdateStatusIfCurrent(context.Context, int64, int64, int64, int64, int64) (bool, error)
	})
	if !ok {
		return false, fmt.Errorf("terminal status CAS missing")
	}
	if entity.ExptStatus(status) == entity.ExptStatus_Terminated {
		<-r.successWritten
	}
	applied, err := updater.UpdateStatusIfCurrent(ctx, exptID, exptRunID, spaceID, expectedStatus, status)
	if entity.ExptStatus(status) == entity.ExptStatus_Success {
		close(r.successWritten)
	}
	return applied, err
}

func TestSupersededTerminalWritesCannotOverwriteEachOther(t *testing.T) {
	f := newLateWriteFixture(t, "")
	ctx := context.Background()
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", f.space, f.expt).
		Update("latest_run_id", f.run+1).Error)
	baseRepo := experimentrepo.NewExptRunLogRepo(experimentmysql.NewExptRunLogDAO(f.provider))
	raceRepo := &terminalRaceRunLogRepo{IExptRunLogRepo: baseRepo, successWritten: make(chan struct{})}
	raceRepo.reads.Add(2)
	manager := &ExptMangerImpl{runLogRepo: raceRepo}

	errs := make(chan error, 2)
	go func() { errs <- manager.finishSupersededRunLog(ctx, f.expt, f.run, f.space) }()
	go func() {
		errs <- manager.finishSupersededRunLog(ctx, f.expt, f.run, f.space, entity.WithStatus(entity.ExptStatus_Terminated))
	}()
	retryErrors := 0
	for range 2 {
		err := <-errs
		if errno.NeedMQRetry(err) {
			retryErrors++
		} else {
			require.NoError(t, err)
		}
	}
	run, err := baseRepo.Get(ctx, f.expt, f.run)
	require.NoError(t, err)
	t.Logf("SUPERSEDED_TERMINAL_RACE status=%d retry_errors=%d", run.Status, retryErrors)
	require.Equal(t, 1, retryErrors, "the losing terminal writer must retry instead of overwriting the first terminal state")
	require.Equal(t, int64(entity.ExptStatus_Success), run.Status)
}
