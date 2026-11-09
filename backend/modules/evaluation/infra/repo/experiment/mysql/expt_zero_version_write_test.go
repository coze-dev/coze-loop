// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestAcceptedZeroVersionSourceCanWriteTurn(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "known_version_control"
		if legacy {
			name = "accepted_legacy_turn_log"
		}
		t.Run(name, func(t *testing.T) {
			f := newCurrentResultFixture(t, "")
			if legacy {
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", 0).Error)
			}
			source := f.source()
			result := f.result(source)
			result.ItemVersionID = source.Current.ItemVersionID
			applied, err := f.port.StoreCurrentTurnRunResult(context.Background(), source, result)
			require.NoError(t, err, "the accepted legacy source must remain writable using its authoritative version")
			require.True(t, applied)
			var stored model.ExptTurnResultRunLog
			require.NoError(t, f.conn.Where("id=?", result.ID).Take(&stored).Error)
			require.Equal(t, int32(entity.TurnRunState_Success), stored.Status)
		})
	}
}

func TestAcceptedZeroVersionSourcePersistsItemCompletion(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "known_version_control"
		if legacy {
			name = "accepted_legacy_item_log"
		}
		t.Run(name, func(t *testing.T) {
			f := newCurrentResultFixture(t, "")
			if legacy {
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", 0).Error)
			}
			source := f.source()
			applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, FailPendingTurns: true})
			require.NoError(t, err)
			require.True(t, applied)
			var stored model.ExptItemResultRunLog
			require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&stored).Error)
			require.Equal(t, int32(entity.ItemRunState_Fail), stored.Status, "applied completion must change the accepted legacy item run log")
			require.Equal(t, int32(entity.ExptItemResultStateLogged), *stored.ResultState)
		})
	}
}

func TestAcceptedZeroVersionSourceCanWriteTurnWithPhysicalVersion(t *testing.T) {
	f := newCurrentResultFixture(t, "")
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", 0).Error)
	source := f.source()
	result := f.result(source)
	require.Zero(t, result.ItemVersionID, "the execution writer must preserve the accepted physical legacy version")
	applied, err := f.port.StoreCurrentTurnRunResult(context.Background(), source, result)
	require.NoError(t, err)
	require.True(t, applied)
	var stored model.ExptTurnResultRunLog
	require.NoError(t, f.conn.Where("id=?", result.ID).Take(&stored).Error)
	require.Equal(t, int32(entity.TurnRunState_Success), stored.Status)
}

func TestAcceptedZeroVersionSourcePublishesCanonicalTurn(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	ctx := context.Background()
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{
		"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged),
	}).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{
		"status": int32(entity.TurnRunState_Success), "evaluator_result_ids": []byte(`null`),
	}).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", 0).Error)

	port := NewExptResultPublishDAO(f.provider)
	source, err := port.GetItemResultPublishSource(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	publication := &entity.ExptItemResultPublication{Source: source}
	for _, old := range source.Source.CanonicalTurns {
		turn := *old
		log := source.Source.Turns[old.TurnID]
		turn.Status = int32(log.Status)
		turn.TargetResultID = log.TargetResultID
		turn.TraceID = log.TraceID
		turn.LogID = log.LogID
		turn.ErrMsg = log.ErrMsg
		turn.ExptRunID = source.Source.Current.RunID
		turn.WeightedScore = nil
		publication.Turns = append(publication.Turns, &turn)
	}
	applied, err := port.CommitItemResult(ctx, publication)
	require.NoError(t, err)
	require.True(t, applied)
	var stored model.ExptTurnResult
	require.NoError(t, f.conn.Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Take(&stored).Error)
	require.Equal(t, int32(entity.TurnRunState_Success), stored.Status)
	require.Equal(t, f.run, stored.ExptRunID)
	var itemLog model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&itemLog).Error)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), *itemLog.ResultState)
}
