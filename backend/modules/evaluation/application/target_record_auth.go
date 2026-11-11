// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"strconv"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func targetRecordAuthorization(record *entity.EvalTargetRecord) *rpc.AuthorizationParam {
	if record.ExperimentOwned() {
		return &rpc.AuthorizationParam{
			ObjectID:      strconv.FormatInt(record.SpaceID, 10),
			SpaceID:       record.SpaceID,
			ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
		}
	}
	return &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(record.TargetID, 10),
		SpaceID:       record.SpaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_EvaluationTarget)}},
	}
}
