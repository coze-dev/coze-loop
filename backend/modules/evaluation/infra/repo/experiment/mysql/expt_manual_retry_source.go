// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// The caller holds the experiment and canonical item row locks in both
// preflight and acceptance. Single-set starts have no stored item refs.
func (d *ExptManualRetryDAOImpl) lockManualRetrySource(ctx context.Context, r *entity.RequeueTerminalItemsRequest, item *lockedManualRetryItem, expt *model.Experiment, opts []db.Option) (string, error) {
	if source := item.prepared.SingleSetSource; source != nil {
		actual := entity.PreparedRetrySingleSetSource{
			SourceType: entity.ExptEvalSetSourceType(expt.EvalSetSourceType),
			EvalSetID:  expt.EvalSetID, EvalSetVersionID: expt.EvalSetVersionID,
			EvalSetSpaceID: expt.EvalSetSpaceID, TargetID: expt.TargetID,
			TargetVersionID: expt.TargetVersionID, TargetSpaceID: expt.TargetSpaceID,
			TargetType: entity.EvalTargetType(expt.TargetType),
		}
		if (actual.SourceType != 0 && actual.SourceType != entity.ExptEvalSetSourceType_SingleSet) ||
			actual.EvalSetID <= 0 || actual.EvalSetVersionID <= 0 || actual.EvalSetVersionID == actual.EvalSetID ||
			actual != *source || item.prepared.FrozenItemRef != nil || item.item.ItemVersionID < 0 ||
			(item.log != nil && item.log.ItemVersionID != item.prepared.ItemVersionID) {
			return "source_changed", nil
		}
		return "", nil
	}
	// Preserve the original multi-set stored-ref check and its missing-ref error.
	frozen, err := NewExptItemRefDAO(d.provider).GetByExptIDAndItemID(ctx, r.SpaceID, r.ExptID, item.item.ItemID, opts...)
	if err != nil {
		return "", err
	}
	if frozen.ItemConfig != nil {
		var cfg *entity.ExptItemConfig
		if err := json.Unmarshal(*frozen.ItemConfig, &cfg); err != nil {
			return "", err
		}
	}
	if !reflect.DeepEqual(convert.NewExptItemRefConvertor().PO2DO(frozen), item.prepared.FrozenItemRef) || frozen.ItemVersionID != item.prepared.ItemVersionID {
		return "source_changed", nil
	}
	return "", nil
}
