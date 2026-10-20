// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type terminalRetryPreflight interface {
	PreflightTerminalItems(context.Context, *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryPreparation, error)
}

func TestManualRetryMySQLPreflight(t *testing.T) {
	for _, kind := range []string{"valid_read_only", "source_changed", "capacity", "ext_capacity", "evaluator_capacity", "corrupt_turn_ext"} {
		t.Run(kind, func(t *testing.T) {
			f := newManualRetryFixture(t)
			req := f.request(entity.EvaluationModeRetryItems)
			switch kind {
			case "source_changed":
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("id=?", req.Items[0].SourceItem.ID).Update("log_id", "changed").Error)
			case "capacity":
				raw := []byte(`[{"ItemIDs":[11],"padding":"` + strings.Repeat("x", 65420) + `"}]`)
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("id=?", f.run).Update("item_ids", raw).Error)
			case "ext_capacity":
				raw := []byte(`{"other":"` + strings.Repeat("x", 65450) + `"}`)
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("id=?", req.Items[0].SourceItem.ID).Update("ext", raw).Error)
				req = f.request(entity.EvaluationModeRetryItems)
			case "evaluator_capacity":
				req = f.request(entity.EvaluationModeFailRetry)
				req.Items[0].Turns[0].EvaluatorResults = &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: strings.Repeat("x", 65536), RecordID: 51}}}
			case "corrupt_turn_ext":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Update("ext", []byte(`{"bad":null}`)).Error)
			}
			before := f.snapshot()
			port, ok := f.port.(terminalRetryPreflight)
			require.True(t, ok)
			plan, err := port.PreflightTerminalItems(context.Background(), req)
			require.Equal(t, before, f.snapshot(), "preflight must not write a receipt, reset, or allocate log IDs")
			if kind == "capacity" || kind == "ext_capacity" || kind == "evaluator_capacity" || kind == "corrupt_turn_ext" {
				require.Error(t, err)
				require.Nil(t, plan)
				biz, ok := kerrors.FromBizStatusError(err)
				if kind == "corrupt_turn_ext" {
					require.False(t, ok)
				} else {
					require.True(t, ok)
					require.Equal(t, int32(601204104), biz.BizStatusCode())
				}
				return
			}
			require.NoError(t, err)
			if kind == "source_changed" {
				require.Empty(t, plan.Items)
				require.Equal(t, []*entity.ManualRetryRejectedItem{{ItemID: 11, Reason: "source_changed"}}, plan.RejectedItems)
				return
			}
			require.Len(t, plan.Items, 1)
			require.Empty(t, plan.RejectedItems)
			require.Equal(t, int64(1700000000123), plan.SnapshotAtMS)
			receipt, err := f.port.RequeueTerminalItems(context.Background(), req)
			require.NoError(t, err)
			require.Equal(t, int32(1), receipt.AcceptedCount)
			got, err := f.port.FindReceipt(context.Background(), f.space, f.expt, f.run, req.RequestID, req.PayloadHash)
			require.NoError(t, err)
			require.Equal(t, receipt.RequestID, got.RequestID)
			require.Equal(t, receipt.AcceptedCount, got.AcceptedCount)
			require.Equal(t, receipt.SnapshotAtMS, got.SnapshotAtMS)
			require.Equal(t, receipt.RejectedItems, got.RejectedItems)
		})
	}
}
