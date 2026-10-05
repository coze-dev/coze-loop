// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

type boundInitDataset struct {
	EvaluationSetItemService
	calls  atomic.Int64
	fail   bool
	denied map[int64]bool
}

func (s *boundInitDataset) BatchGetEvaluationSetItems(_ context.Context, in *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	s.calls.Add(1)
	if s.fail {
		return nil, fmt.Errorf("source deliberately unavailable")
	}
	var items []*entity.EvaluationSetItem
	for _, id := range in.ItemIDs {
		if s.denied[id] {
			return nil, fmt.Errorf("committed source must not be reloaded")
		}
		items = append(items, &entity.EvaluationSetItem{ID: id, ItemID: id, SpaceID: in.SpaceID, EvaluationSetID: in.EvaluationSetID, Turns: []*entity.Turn{{ID: 0, ItemID: id, EvalSetID: in.EvaluationSetID}}})
	}
	return items, nil
}

func (s *boundInitDataset) GetEvaluationSetItemVersion(_ context.Context, space, set, item int64, version *int64, _ *string) (*entity.EvaluationSetItemVersion, error) {
	s.calls.Add(1)
	if s.fail || s.denied[item] {
		return nil, fmt.Errorf("committed version must not be reloaded")
	}
	return &entity.EvaluationSetItemVersion{ItemID: item, ItemVersionID: *version, Turns: []*entity.Turn{{ID: 0, ItemID: item, EvalSetID: set}}}, nil
}

type boundInitFixture struct {
	*finalizationManagerFixture
	items   []entity.HookPlanItem
	dataset *boundInitDataset
	loader  hook.PlanPageLoader
	binding *entity.HookExecutionInitializationBinding
}

// Plans are prepared manually; Manager capture, decoding, loader and SQL are real.
func newBoundInitFixture(t *testing.T, n int, mode entity.ExptRunMode, before, after bool, state ...string) *boundInitFixture {
	t.Helper()
	f := foundationManagerFixture(t, before, after)
	foundationCreate(t, f, mode)
	ctx := context.Background()
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptItemRef{}).Error)
	})
	b := &boundInitFixture{finalizationManagerFixture: f, dataset: new(boundInitDataset)}
	digest := entity.NewHookPlanDigest()
	for i := 0; i < n; i++ {
		item := entity.HookPlanItem{ID: finalizationTestIDs.Add(1), ItemID: finalizationTestIDs.Add(1), SourceSpaceID: f.space, EvalSetID: 71}
		if i%2 == 1 {
			item.SourceSpaceID, item.EvalSetID, item.EvalSetVersionID = f.space+50, 81, 82
			item.ItemVersionID = 7
		}
		b.items = append(b.items, item)
	}
	for start := 0; start < n; start += 100 {
		page := b.items[start:min(start+100, n)]
		run := finalizationRead(t, f)
		_, err := f.repo.AppendPlanPage(ctx, entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, StartOrdinal: int64(start), Cursor: run.PlanCursor, NextCursor: fmt.Sprint(start + len(page)), Items: page})
		require.NoError(t, err)
		digest, err = entity.AppendHookPlanDigest(digest, page)
		require.NoError(t, err)
	}
	run := finalizationRead(t, f)
	var err error
	if len(state) == 0 || state[0] != "unprepared" {
		_, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Count: int64(n), Hash: digest.Hash})
		require.NoError(t, err)
	}
	if before && len(state) == 0 {
		run = finalizationRead(t, f)
		config, hash, err := f.manager.hooks.Codec.DecodePhase(ctx, f.key, "local", run.Snapshot, entity.HookPhaseBefore)
		require.NoError(t, err)
		scope := entity.HookAttemptScope{Key: f.key, OperationID: run.State.Before.ID, Phase: entity.HookPhaseBefore, ExecutionScope: "local", SnapshotHash: hash, ExpectedVersion: run.Operations[0].Version}
		for _, op := range run.Operations {
			if op.Phase == entity.HookPhaseBefore {
				scope.ExpectedVersion = op.Version
			}
		}
		claim, err := f.repo.ClaimAttempt(ctx, entity.HookClaimAttemptInput{HookAttemptScope: scope, Owner: "bound-init-test", AttemptID: finalizationTestIDs.Add(1), Config: config})
		require.NoError(t, err)
		require.NotNil(t, claim.Claim)
		t.Cleanup(func() {
			require.NoError(t, f.sql.Where("operation_id=?", scope.OperationID).Delete(&model.ExptLifecycleHookAttempt{}).Error)
		})
		scope.ExpectedVersion = claim.Claim.Version
		_, err = f.repo.CompleteAttempt(ctx, entity.HookCompleteAttemptInput{HookRenewAttemptInput: entity.HookRenewAttemptInput{HookAttemptScope: scope, HookAttemptIdentity: claim.Claim.HookAttemptIdentity}, Config: config, Outcome: entity.HookOutcome{Code: entity.HookSucceeded}, CompletedAt: claim.Claim.StartedAt})
		require.NoError(t, err)
	}
	b.loader, err = NewHookFrozenPlanLoader(HookFrozenPlanLoaderDependencies{Plans: exptinfra.NewHookPlanRepo(f.p), Items: b.dataset,
		Versions: &loaderVersions{value: &entity.EvaluationSetVersion{ID: 82, SpaceID: f.space + 50, EvaluationSetID: 81, EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 83, SpaceID: f.space + 50, EvaluationSetID: 81}}},
		Sets:     &loaderSets{value: &entity.EvaluationSet{ID: 71, SpaceID: f.space, EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 73, SpaceID: f.space, EvaluationSetID: 71}}}}})
	require.NoError(t, err)
	return b
}

func boundInitializationRepo(t *testing.T, f *boundInitFixture) repo.IHookExecutionInitializationRepo {
	t.Helper()
	binding, err := LoadHookExecutionInitializationBinding(context.Background(), f.repo, f.manager.hooks.Codec, f.key, "local")
	require.NoError(t, err)
	f.binding = binding
	r, err := exptinfra.NewBoundHookExecutionInitializationRepo(f.p, binding)
	require.NoError(t, err)
	return r
}

func (f *boundInitFixture) initialize(t *testing.T, r repo.IHookExecutionInitializationRepo) (entity.HookExecutionInitializationCompletion, error) {
	t.Helper()
	deps := HookFrozenExecutionInitializerDependencies{Repository: r, Loader: f.loader, IDs: activeReferenceIDs{}}
	var s hook.ExecutionInitializer
	var err error
	if f.binding == nil {
		s, err = NewHookFrozenExecutionInitializer(deps)
	} else {
		s, err = NewBoundHookFrozenExecutionInitializer(deps, f.binding)
	}
	require.NoError(t, err)
	return s.InitializeExecution(context.Background(), f.key, "local")
}

func TestHookBoundInitializationRoundtripMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		for _, phases := range [][2]bool{{true, false}, {false, true}, {true, true}} {
			t.Run(fmt.Sprintf("%d/%v", mode, phases), func(t *testing.T) {
				f := newBoundInitFixture(t, 2, mode, phases[0], phases[1])
				r := boundInitializationRepo(t, f)
				result, err := f.initialize(t, r)
				require.NoError(t, err, "explicit binding must allow component initialization without widening ordinary scope")
				require.True(t, result.Initialized)
				reader := exptinfra.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(f.p))
				for i, item := range f.items {
					ref, err := reader.GetByExptIDAndItemID(context.Background(), f.space, f.expt, item.ItemID)
					require.NoError(t, err)
					require.NotNil(t, ref)
					require.Equal(t, item.EvalSetID, ref.EvalSetID)
					require.Equal(t, item.EvalSetVersionID, ref.EvalSetVersionID)
					require.Equal(t, item.ItemVersionID, ref.ItemVersionID)
					require.Equal(t, int32(i), ref.OrderIdx)
					wantAlias := "first"
					wantVersion := int64(111)
					wantSpace := int64(0)
					if i == 1 {
						wantAlias, wantVersion, wantSpace = "second", 222, f.space+50
					}
					require.Equal(t, wantAlias, ref.ItemConfig.EvaluatorConfs[0].Alias)
					require.Equal(t, wantVersion, ref.ItemConfig.EvaluatorConfs[0].EvaluatorVersionID)
					require.Equal(t, wantSpace, ref.ItemConfig.EvalSetSourceSpaceID)
					var row model.ExptLifecycleRunItem
					require.NoError(t, f.sql.First(&row, item.ID).Error)
					var m map[string]json.RawMessage
					require.NoError(t, json.Unmarshal(*row.ExecutionManifest, &m))
					require.NotEmpty(t, m["item_ref"])
					var manifest entity.HookExecutionManifest
					require.NoError(t, json.Unmarshal(*row.ExecutionManifest, &manifest))
					var rawRef model.ExptItemRef
					require.NoError(t, f.sql.First(&rawRef, ref.ID).Error)
					digest := sha256.Sum256(*rawRef.ItemConfig)
					require.Equal(t, ref.ID, manifest.ItemRef.ID)
					require.Equal(t, hex.EncodeToString(digest[:]), manifest.ItemRef.ConfigHash)
					require.NotContains(t, string(*row.ExecutionManifest), "private-target")
					require.NotContains(t, string(*row.ExecutionManifest), "original-user")
				}
				require.False(t, finalizationRead(t, f.finalizationManagerFixture).State.After.Activated)
				require.False(t, entity.HookExecutionInitializationRequired(true, mode, entity.ExptType_Offline, entity.ExptEvalSetSourceType_MultiSetConfig))
			})
		}
	}
}
