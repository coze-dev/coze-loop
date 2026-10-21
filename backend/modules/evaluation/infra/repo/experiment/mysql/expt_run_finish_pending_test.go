// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

// Port boundary coverage complements the service test that calls real LogRun.
func TestRunFinishPendingAdmissionSQLPorts(t *testing.T) {
	for _, stage := range []string{"preflight", "requeue"} {
		t.Run(stage, func(t *testing.T) {
			f := newManualRetryFixture(t)
			require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.run).Update("status", int64(entity.ExptStatus_Pending)).Error)
			request := f.request(entity.EvaluationModeRetryItems)
			if stage == "preflight" {
				plan, err := f.port.PreflightTerminalItems(context.Background(), request)
				require.NoError(t, err)
				require.Len(t, plan.Items, 1)
			} else {
				receipt, err := f.port.RequeueTerminalItems(context.Background(), request)
				require.NoError(t, err)
				require.Equal(t, int32(1), receipt.AcceptedCount)
			}
			var run model.ExptRunLog
			require.NoError(t, f.conn.Where("space_id=? AND expt_run_id=?", f.space, f.run).Take(&run).Error)
			require.Equal(t, int64(entity.ExptStatus_Pending), gptr.Indirect(run.Status))
		})
	}
}
func TestRunFinishPendingDoesNotAdmitInactiveExperimentOrRun(t *testing.T) {
	for _, state := range []entity.ExptStatus{entity.ExptStatus_Pending, entity.ExptStatus_Terminating, entity.ExptStatus_Draining, entity.ExptStatus_Success, entity.ExptStatus_Failed} {
		t.Run(fmt.Sprint(state), func(t *testing.T) {
			f := newManualRetryFixture(t)
			request := f.request(entity.EvaluationModeRetryItems)
			require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("status", int32(state)).Error)
			before := f.snapshot()
			_, err := f.port.PreflightTerminalItems(context.Background(), request)
			require.Error(t, err)
			_, err = f.port.RequeueTerminalItems(context.Background(), request)
			require.Error(t, err)
			require.Equal(t, before, f.snapshot())
		})
	}
	for _, state := range []entity.ExptStatus{entity.ExptStatus_Terminating, entity.ExptStatus_Draining, entity.ExptStatus_Success, entity.ExptStatus_Failed} {
		t.Run("run_"+fmt.Sprint(state), func(t *testing.T) {
			f := newManualRetryFixture(t)
			request := f.request(entity.EvaluationModeRetryItems)
			require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(state)).Error)
			before := f.snapshot()
			_, err := f.port.PreflightTerminalItems(context.Background(), request)
			require.Error(t, err)
			_, err = f.port.RequeueTerminalItems(context.Background(), request)
			require.Error(t, err)
			require.Equal(t, before, f.snapshot())
		})
	}
}
