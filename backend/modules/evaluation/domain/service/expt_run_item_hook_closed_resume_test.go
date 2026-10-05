// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestHookClosedResumeTerminalOriginalRunNoop(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated} {
		for _, newer := range []bool{false, true} {
			for _, flags := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
				t.Run(fmt.Sprintf("%d/newer=%v/flags=%v", status, newer, flags), func(t *testing.T) {
					f := newItemHookFixture(t, true, true)
					f.ports.source.RunLog.Status = int64(status)
					if newer {
						f.ports.source.LatestRunID = 4
						f.expt.LatestRunID = 4
					}
					f.ports.gates = []entity.HookGateState{entity.HookGateClosed}
					f.event.AsyncReportTrigger, f.event.AsyncEvaluatorReportTrigger = flags[0], flags[1]
					f.turnLogs = []*entity.ExptTurnResultRunLog{{ID: 101, SpaceID: 1, ExptID: 2, ExptRunID: 3, ItemID: 4, TurnID: 0, TargetResultID: 71, EvaluatorResultIds: &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 81, Alias: "a", RecordID: 91}}}}}
					beforeLog := *f.turnLogs[0]
					beforeItem := *f.item
					beforeExpt := *f.expt
					require.NoError(t, f.svc.Eval(context.Background(), f.event))
					f.noExecution(t)
					require.Equal(t, []string{"source"}, f.ports.trace)
					require.Empty(t, f.ports.admitted)
					require.Empty(t, f.publisher.events)
					require.Equal(t, beforeLog, *f.turnLogs[0])
					require.Equal(t, beforeItem, *f.item)
					require.Equal(t, beforeExpt, *f.expt)
				})
			}
		}
	}
}

func TestHookClosedResumeUnconfirmedOrCorruptStillRejects(t *testing.T) {
	for _, kind := range []string{"closed-active", "superseded-active", "terminating", "unknown-status", "wrong-run", "wrong-space", "missing-latest"} {
		t.Run(kind, func(t *testing.T) {
			f := newItemHookFixture(t, true, true)
			f.event.AsyncReportTrigger = true
			f.event.AsyncEvaluatorReportTrigger = true
			f.ports.gates = []entity.HookGateState{entity.HookGateClosed}
			switch kind {
			case "superseded-active":
				f.ports.source.LatestRunID = 4
			case "terminating":
				f.ports.source.RunLog.Status = int64(entity.ExptStatus_Terminating)
			case "unknown-status":
				f.ports.source.RunLog.Status = 999
			case "wrong-run":
				f.ports.source.RunLog.Status = int64(entity.ExptStatus_Success)
				f.ports.source.RunLog.ExptRunID = 9
			case "wrong-space":
				f.ports.source.RunLog.Status = int64(entity.ExptStatus_Success)
				f.ports.source.RunLog.SpaceID = 9
			case "missing-latest":
				f.ports.source.RunLog.Status = int64(entity.ExptStatus_Success)
				f.ports.source.LatestRunID = 0
			}
			err := f.svc.Eval(context.Background(), f.event)
			f.noExecution(t)
			if kind == "closed-active" || kind == "superseded-active" || kind == "terminating" {
				require.Error(t, err)
			} else {
				require.Len(t, f.publisher.events, 1, "uncertain source must retry, not silently acknowledge")
			}
		})
	}
}
