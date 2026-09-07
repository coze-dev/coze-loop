// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestBuildItemCompleteEvent_NormalizesDatasetKey(t *testing.T) {
	for _, sourceType := range []entity.ExptEvalSetSourceType{
		entity.ExptEvalSetSourceType_SingleSet,
		entity.ExptEvalSetSourceType_MultiSetConfig,
	} {
		for _, tt := range []struct {
			key  string
			want string
		}{
			{key: "dataset_new_runtime", want: "dataset"},
			{key: "dataset_schema_v0_1", want: "dataset"},
			{key: "dataset_schema_v0_1_new_runtime_schema_v0_1", want: "dataset"},
			{key: "dataset_new_runtime_part", want: "dataset_new_runtime_part"},
		} {
			t.Run(fmt.Sprintf("%d/%s", sourceType, tt.key), func(t *testing.T) {
				evalSet := &entity.EvaluationSet{
					ID:                   700,
					DatasetKey:           tt.key,
					EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 800, Version: "0.0.1"},
				}
				experiment := &entity.Experiment{
					EvalSetSourceType:  sourceType,
					ExperimentGroupKey: "group-key",
					EvalSet:            evalSet,
				}
				if sourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
					experiment.EvalSet = &entity.EvaluationSet{ID: 900, DatasetKey: "primary_new_runtime"}
					experiment.EvalSetDetails = []*entity.ExptEvalSetDetail{{EvalSetID: 700, EvalSet: evalSet}}
				}
				evalSetItem := &entity.EvaluationSetItem{SpaceID: 1, EvaluationSetID: 700, ItemKey: "item_new_runtime"}
				ctx := &entity.ExptItemEvalCtx{
					Event:            &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 100, ExptRunID: 200, EvalSetItemID: 300},
					Expt:             experiment,
					EvalSetItem:      evalSetItem,
					EvalSetVersionID: 800,
				}
				fromItem := buildItemCompleteEvent(ctx)
				fromScheduler := buildItemCompleteEventFromScheduler(1, 100, 200, experiment,
					&entity.ExptEvalItem{ItemID: 300}, evalSetItem, 800)

				assert.Equal(t, tt.want, fromItem.DatasetKey)
				assert.Equal(t, fromItem, fromScheduler)
				assert.Equal(t, "item_new_runtime", fromItem.ItemKey)
				assert.Equal(t, "group-key", fromItem.ExperimentGroupKey)
				assert.Equal(t, "700", fromItem.DatasetID)
				assert.Equal(t, "800", fromItem.DatasetVersionID)
				assert.Equal(t, tt.key, evalSet.DatasetKey)
			})
		}
	}
}
