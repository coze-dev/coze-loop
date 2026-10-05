// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IHookRunPlanStarter adds request selection capture without changing IExptManager.
type IHookRunPlanStarter interface {
	LogRunWithPlanSeed(context.Context, int64, int64, entity.ExptRunMode, int64, string, *entity.Session) error
}

var _ IHookRunPlanStarter = (*ExptMangerImpl)(nil)

func (e *ExptMangerImpl) LogRunWithPlanSeed(ctx context.Context, exptID, runID int64, mode entity.ExptRunMode, spaceID int64, rawItemIDs string, session *entity.Session) error {
	if e.hooks == nil {
		return e.LogRun(ctx, exptID, runID, mode, spaceID, nil, session)
	}
	return e.logHookRun(ctx, exptID, runID, mode, spaceID, nil, session, &rawItemIDs)
}

func hookSelectionItemIDs(count int64, raw string) (bool, []int64, error) {
	if count <= 0 || raw == "" {
		return false, nil, nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return false, nil, errors.New("HOOK_SELECTION_INVALID")
	}
	return true, ids, nil
}

func setHookSelectionItems(log *entity.ExptRunLog, ids []int64) {
	log.ItemIds = nil
	if len(ids) > 0 {
		log.ItemIds = []entity.ExptRunLogItems{{ItemIDs: slices.Clone(ids), CreateAt: gptr.Of(time.Now().Unix())}}
	}
}

func newHookSelectionSeed(expt *entity.Experiment, log *entity.ExptRunLog, raw *string) (*entity.HookSelectionSeed, error) {
	fingerprint, err := entity.HookSelectionConfigFingerprint(expt)
	if err != nil {
		return nil, errors.New("HOOK_SELECTION_INVALID")
	}
	seed := &entity.HookSelectionSeed{Version: 1, TrialRunItemCount: expt.TrialRunItemCount, ConfigFingerprint: fingerprint}
	if entity.ExptRunMode(log.Mode) == entity.EvaluationModeTrialRun && expt.TrialRunItemCount > 0 {
		if raw != nil {
			explicit, ids, err := hookSelectionItemIDs(expt.TrialRunItemCount, *raw)
			if err != nil {
				return nil, err
			}
			seed.HasExplicitItemIDs = explicit
			setHookSelectionItems(log, ids)
		} else {
			seed.HasExplicitItemIDs = len(log.GetItemIDs()) > 0
		}
	}
	return seed, nil
}

func (e *ExptMangerImpl) restoreHookSelectionForReplay(ctx context.Context, log *entity.ExptRunLog, stored *entity.HookStoredRun, raw *string) error {
	if raw == nil || stored.Mode != entity.EvaluationModeTrialRun {
		return nil
	}
	snapshot, err := e.hooks.Codec.DecodeSnapshot(ctx, stored.State.Key, e.hooks.ExecutionScope, stored.Snapshot)
	if err != nil {
		return entity.ErrHookStoreCorrupt
	}
	seed := snapshot.Input().Selection
	if seed == nil {
		return nil
	}
	explicit, ids, err := hookSelectionItemIDs(seed.TrialRunItemCount, *raw)
	if err != nil {
		return err
	}
	if explicit != seed.HasExplicitItemIDs {
		return entity.ErrHookStoreConflict
	}
	setHookSelectionItems(log, ids)
	return nil
}
