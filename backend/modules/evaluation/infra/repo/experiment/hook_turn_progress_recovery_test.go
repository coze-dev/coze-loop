// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookTurnProgressRecoveryReplacementDoesNotInheritScores(t *testing.T) {
	for _, kind := range []string{"empty", "copied", "fresh", "partial-fresh", "concurrent-old"} {
		t.Run(kind, func(t *testing.T) {
			f := newHookProgressFixture(t)
			old := &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 11}, Registered: []*entity.RegisteredEvalResult{{VersionID: 402, Alias: "a", RecordID: 12}}, Inline: []*entity.InlineEvalResult{{InlineKey: "quality", RecordID: 13}}}
			raw, err := json.Marshal(old)
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("id=?", f.row.ID).UpdateColumn("evaluator_result_ids", raw).Error)
			base := f.read(t)
			next := *base
			next.TargetResultID = 101
			switch kind {
			case "empty":
				next.EvaluatorResultIds = nil
			case "fresh":
				next.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 21}, {VersionID: 402, Alias: "a", RecordID: 22}}, Inline: []*entity.InlineEvalResult{{InlineKey: "quality", RecordID: 23}}}
			case "partial-fresh":
				next.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 21}, {VersionID: 402, Alias: "a", RecordID: 12}}}
			case "concurrent-old":
				concurrent := *base
				concurrent.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 403, RecordID: 14}}}
				require.NoError(t, f.write(context.Background(), base, &concurrent))
			}
			require.NoError(t, f.write(context.Background(), base, &next))
			got := f.read(t)
			refs, err := hookProgressRefs(got.EvaluatorResultIds)
			require.NoError(t, err)
			for _, id := range refs {
				assert.Greater(t, id, int64(20), "no T1 record can be associated with T2")
			}
			if kind == "fresh" {
				assert.Len(t, refs, 3)
			} else if kind == "partial-fresh" {
				assert.Equal(t, map[hookProgressRefKey]int64{{version: 401}: 21}, refs)
			} else {
				assert.Empty(t, refs)
			}
			require.NoError(t, f.write(context.Background(), base, &next), "replay remains idempotent")
			after, err := hookProgressRefs(f.read(t).EvaluatorResultIds)
			require.NoError(t, err)
			assert.Equal(t, refs, after)
			current := f.read(t)
			concurrent := *current
			concurrent.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: 403, RecordID: 24}}}
			require.NoError(t, f.write(context.Background(), current, &concurrent))
			require.NoError(t, f.write(context.Background(), base, &next))
			after, err = hookProgressRefs(f.read(t).EvaluatorResultIds)
			require.NoError(t, err)
			assert.Equal(t, int64(24), after[hookProgressRefKey{version: 403}], "old replay preserves disjoint T2 scores")
		})
	}
}
