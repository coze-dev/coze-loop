// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestResultSetLegacyColumnsExposeOnlyPresentKnownOutputs(t *testing.T) {
	result := &entity.MGetExperimentReportResult{ExptColumnsEvalTarget: []*entity.ExptColumnEvalTarget{{ExptID: 1, Columns: []*entity.ColumnEvalTarget{{Name: "FORNAX_eval"}}}, {ExptID: 2}}}
	for _, id := range []int64{1, 2} {
		result.ItemResults = append(result.ItemResults, &entity.ItemResult{TurnResults: []*entity.TurnResult{{ExperimentResults: []*entity.ExperimentResult{{ExperimentID: id, Payload: &entity.ExperimentTurnPayload{TargetOutput: &entity.TurnTargetOutput{EvalTargetRecord: &entity.EvalTargetRecord{EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{"FORNAX_output": {ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("{}")}, "FORNAX_eval": {}, "FORNAX_agent": {}, "unknown": {}}}}}}}}}}})
	}
	expts := map[int64]*entity.Experiment{1: {EvalConf: &entity.EvaluationConfiguration{RunModeConfig: &entity.RunModeConfig{RunMode: entity.RunModeResultSet}}}, 2: {}}
	completeResultSetOutputColumns(result, expts)
	completeResultSetOutputColumns(result, expts)
	var names []string
	for _, c := range result.ExptColumnsEvalTarget[0].Columns {
		names = append(names, c.Name)
	}
	require.Equal(t, []string{"FORNAX_eval", "FORNAX_output"}, names)
	require.Empty(t, result.ExptColumnsEvalTarget[1].Columns)
}
