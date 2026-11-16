// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ICentralReservationReleaser 只做终端/重试路径的释放，不依赖调度器，避免与 IExptManager 形成依赖环。
type ICentralReservationReleaser interface {
	ReleaseRun(ctx context.Context, expt *entity.Experiment, runID int64, reason string) error
	ReleaseRunByID(ctx context.Context, exptID, runID, spaceID int64, reason string) error
	ReleaseItems(ctx context.Context, expt *entity.Experiment, runID int64, itemIDs []int64, reason string) error
	ReleaseExperiment(ctx context.Context, expt *entity.Experiment, reason string) error
	ReleaseExperimentByID(ctx context.Context, exptID, spaceID int64, reason string) error
}

type CentralReservationReleaser struct {
	ledger   repo.ICentralLedgerRepo
	env      component.IEnvironmentProvider
	exptRepo repo.IExperimentRepo
}

func NewCentralReservationReleaser(ledger repo.ICentralLedgerRepo, env component.IEnvironmentProvider, exptRepo repo.IExperimentRepo) ICentralReservationReleaser {
	return &CentralReservationReleaser{ledger: ledger, env: env, exptRepo: exptRepo}
}

// envOf 归属不明一律不放行（也不释放），由人工/对账在可判定后再处理。
func (r *CentralReservationReleaser) envOf(ctx context.Context, expt *entity.Experiment) (string, error) {
	if expt == nil {
		return "", errors.New("nil experiment")
	}
	meta := entity.GetSchedulingMeta(expt.EvalConf)
	if !meta.Mode.IsCentralDispatchMode() {
		return "", fmt.Errorf("experiment %d not central scheduling mode", expt.ID)
	}
	current, err := r.env.CurrentEnvID(ctx)
	if err != nil {
		return "", err
	}
	if meta.Env == "" || meta.Env != current {
		return "", fmt.Errorf("experiment %d env %q mismatch current %q", expt.ID, meta.Env, current)
	}
	return meta.Env, nil
}

func (r *CentralReservationReleaser) ReleaseRun(ctx context.Context, expt *entity.Experiment, runID int64, reason string) error {
	env, err := r.envOf(ctx, expt)
	if err != nil {
		logs.CtxWarn(ctx, "[CentralReleaser] skip release run: %v", err)
		return nil
	}
	resvs, err := r.ledger.ListByRun(ctx, env, expt.ID, runID)
	if err != nil {
		return err
	}
	return r.releaseAll(ctx, env, resvs, reason)
}

func (r *CentralReservationReleaser) ReleaseRunByID(ctx context.Context, exptID, runID, spaceID int64, reason string) error {
	expt, err := r.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return err
	}
	return r.ReleaseRun(ctx, expt, runID, reason)
}

func (r *CentralReservationReleaser) ReleaseItems(ctx context.Context, expt *entity.Experiment, runID int64, itemIDs []int64, reason string) error {
	env, err := r.envOf(ctx, expt)
	if err != nil {
		logs.CtxWarn(ctx, "[CentralReleaser] skip release items: %v", err)
		return nil
	}
	want := make(map[int64]bool, len(itemIDs))
	for _, id := range itemIDs {
		want[id] = true
	}
	resvs, err := r.ledger.ListByRun(ctx, env, expt.ID, runID)
	if err != nil {
		return err
	}
	kept := make([]*entity.Reservation, 0, len(resvs))
	for _, resv := range resvs {
		if resv != nil && want[resv.ItemID] {
			kept = append(kept, resv)
		}
	}
	return r.releaseAll(ctx, env, kept, reason)
}

func (r *CentralReservationReleaser) ReleaseExperimentByID(ctx context.Context, exptID, spaceID int64, reason string) error {
	expt, err := r.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return err
	}
	return r.ReleaseExperiment(ctx, expt, reason)
}

func (r *CentralReservationReleaser) ReleaseExperiment(ctx context.Context, expt *entity.Experiment, reason string) error {
	env, err := r.envOf(ctx, expt)
	if err != nil {
		logs.CtxWarn(ctx, "[CentralReleaser] skip release experiment: %v", err)
		return nil
	}
	all, err := r.ledger.ListAll(ctx, env, 0)
	if err != nil {
		return err
	}
	kept := make([]*entity.Reservation, 0)
	for _, resv := range all {
		if resv != nil && resv.ExptID == expt.ID {
			kept = append(kept, resv)
		}
	}
	return r.releaseAll(ctx, env, kept, reason)
}

func (r *CentralReservationReleaser) releaseAll(ctx context.Context, env string, resvs []*entity.Reservation, reason string) error {
	for _, resv := range resvs {
		if resv == nil || resv.State.IsTerminal() {
			continue
		}
		if _, err := r.ledger.Release(ctx, env, resv.ID, reason); err != nil {
			logs.CtxWarn(ctx, "[CentralReleaser] release %s fail: %v", resv.ID, err)
		}
	}
	return nil
}
