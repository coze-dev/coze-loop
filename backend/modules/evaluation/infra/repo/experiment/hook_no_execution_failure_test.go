// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHookNoExecutionFailureGenericPlatformCause(t *testing.T) {
	for _, admitted := range []bool{false, true} {
		t.Run(fmt.Sprint(admitted), func(t *testing.T) {
			f := newExecutionFixture(t, 1)
			page := executionWrite(t, f)
			complete, err := f.init.CompleteExecutionInitialization(context.Background(), f.completeInput(page.RunVersion))
			require.NoError(t, err)
			require.True(t, complete.Initialized)
			if admitted {
				run, err := f.repo.GetRun(context.Background(), f.key)
				require.NoError(t, err)
				_, err = f.repo.AdmitItem(context.Background(), entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: f.manifests[0].Frozen.ItemID})
				require.NoError(t, err)
			}
			m := f.manifests[0]
			var before model.ExptLifecycleRunItem
			require.NoError(t, f.sql.First(&before, m.Frozen.ID).Error)
			r := &hookFinalizationRepo{provider: f.p}
			message := errno.SerializeErr(errors.New("platform setup failed before PreEval"))
			var original *entity.HookTerminationItem
			err = r.itemArchiveTransaction(context.Background(), f.key, "local", func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
				active, err := hookSchedulerFailureActive(tx, f.key, expt, life)
				if err != nil {
					return err
				}
				require.True(t, active)
				original, _, err = readHookArchiveItem(tx, f.key, m.Frozen.ItemID)
				if err != nil {
					return err
				}
				return markHookNoExecutionFailure(tx, f.key, original, message)
			})
			require.NoError(t, err)
			marked, err := r.ReadHookArchiveItem(context.Background(), f.key, "local", m.Frozen.ItemID)
			require.NoError(t, err)
			require.True(t, marked.Manifest.NoExecutionFailure)
			require.Equal(t, message, string(marked.Item.ErrMsg))
			require.Empty(t, marked.Turns)
			require.Equal(t, int32(entity.ExptItemResultStateLogged), marked.Item.ResultState)
			var after model.ExptLifecycleRunItem
			require.NoError(t, f.sql.First(&after, m.Frozen.ID).Error)
			require.Equal(t, before.AdmittedAt, after.AdmittedAt)
			receipt, err := f.init.ReadExecutionInitializationPage(context.Background(), f.readInput())
			require.NoError(t, err)
			require.True(t, receipt.Items[0].Manifest.NoExecutionFailure)
			_, err = f.init.WriteExecutionInitializationPage(context.Background(), f.writeInput(receipt.RunVersion))
			require.Error(t, err, "an old initializer must not erase the flag")
			replay := f.writeInput(receipt.RunVersion)
			replay.Items = []entity.HookExecutionManifest{marked.Manifest}
			_, err = f.init.WriteExecutionInitializationPage(context.Background(), replay)
			require.NoError(t, err)
			_, err = r.ArchiveHookItem(context.Background(), entity.HookItemArchiveInput{Key: f.key, ExecutionScope: "local", ItemID: m.Frozen.ItemID, Prepared: marked})
			require.NoError(t, err)
			stats, err := r.ReadFinalizationStats(context.Background(), f.key, "local")
			require.NoError(t, err)
			require.Equal(t, int32(1), stats.Items.Fail)
			require.Equal(t, int32(2), stats.Turns.Fail)
			_, err = f.init.CompleteExecutionInitialization(context.Background(), f.completeInput(receipt.RunVersion))
			require.NoError(t, err)
			var counters model.ExptStats
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&counters).Error)
			require.Equal(t, int32(1), counters.FailCnt)
			require.Zero(t, counters.PendingCnt, "initializer replay must not reset failure counts")
			var count int64
			require.NoError(t, f.sql.Unscoped().Model(&model.ExptTurnResultRunLog{}).Where("expt_run_id=?", f.key.RunID).Count(&count).Error)
			require.Zero(t, count)
			err = r.itemArchiveTransaction(context.Background(), f.key, "local", func(tx *gorm.DB, _ *model.Experiment, _ *model.ExptLifecycleRun) error {
				return markHookNoExecutionFailure(tx, f.key, original, message)
			})
			require.NoError(t, err)
			var log model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&log, m.ItemRunLogID).Error)
			require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(log.ResultState), "helper replay must not regress archive completion")
		})
	}
}
