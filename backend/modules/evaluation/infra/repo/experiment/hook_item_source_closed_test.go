// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestHookClosedSourceDeletedParentOnlyManagedTerminal(t *testing.T) {
	for _, managed := range []bool{false, true} {
		for _, status := range []entity.ExptStatus{entity.ExptStatus_Processing, entity.ExptStatus_Terminating, entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated} {
			t.Run(fmt.Sprintf("managed=%v/status=%d", managed, status), func(t *testing.T) {
				marker := int32(0)
				if managed {
					marker = 1
				}
				f, key := newHookItemSourceFixture(t, gptr.Of(marker))
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", key.RunID).UpdateColumn("status", int64(status)).Error)
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", key.ExperimentID).Updates(map[string]any{"deleted_at": time.Now(), "latest_run_id": key.RunID + 100}).Error)
				got, err := readHookItemSourceForTest(context.Background(), f, key)
				if managed && entity.IsExptFinished(status) {
					require.NoError(t, err)
					require.True(t, got.Managed)
					require.Equal(t, int64(status), got.RunLog.Status)
					require.Equal(t, key.RunID, got.RunLog.ExptRunID)
				} else {
					require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
					require.Nil(t, got)
				}
			})
		}
	}
}

func TestHookClosedSourceDeletedParentRejectsCorruption(t *testing.T) {
	for _, field := range []string{"requested-space", "stored-space", "parent-space", "deleted-run", "unknown-marker", "missing-latest"} {
		t.Run(field, func(t *testing.T) {
			f, key := newHookItemSourceFixture(t, gptr.Of(int32(1)))
			require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", key.RunID).UpdateColumn("status", int64(entity.ExptStatus_Success)).Error)
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", key.ExperimentID).UpdateColumn("deleted_at", time.Now()).Error)
			request := key
			switch field {
			case "requested-space":
				request.WorkspaceID++
			case "stored-space":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", key.RunID).UpdateColumn("space_id", key.WorkspaceID+1).Error)
			case "parent-space":
				require.NoError(t, f.sql.Unscoped().Model(&model.Experiment{}).Where("id=?", key.ExperimentID).UpdateColumn("space_id", key.WorkspaceID+1).Error)
			case "deleted-run":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", key.RunID).UpdateColumn("deleted_at", time.Now()).Error)
			case "unknown-marker":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", key.RunID).UpdateColumn("lifecycle_hook_version", 9).Error)
			case "missing-latest":
				require.NoError(t, f.sql.Unscoped().Model(&model.Experiment{}).Where("id=?", key.ExperimentID).UpdateColumn("latest_run_id", 0).Error)
			}
			got, err := readHookItemSourceForTest(context.Background(), f, request)
			require.ErrorIs(t, err, entity.ErrHookGateUnavailable)
			require.Nil(t, got)
		})
	}
}
