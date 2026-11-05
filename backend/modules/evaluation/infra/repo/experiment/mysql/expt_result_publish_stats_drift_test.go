// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

// F5: a legacy processing_cnt=0 row must not be driven negative by publication.
func TestResultPublishMySQLClampsLegacyProcessingDrift(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	ctx := context.Background()
	require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Update("processing_cnt", 0).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{
		"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged),
	}).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{
		"status": int32(entity.TurnRunState_Success), "evaluator_result_ids": []byte(`null`),
	}).Error)
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
	ok, err := port.CommitItemResult(ctx, publication)
	require.NoError(t, err)
	require.True(t, ok)
	var stats model.ExptStats
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Take(&stats).Error)
	require.Equal(t, int32(0), stats.ProcessingCnt)
	require.Equal(t, int32(1), stats.SuccessCnt)
}
