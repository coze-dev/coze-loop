// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func NewExptItemTurnEvalAsyncCtx() *ExptItemTurnEvalAsyncCtxConverter {
	return &ExptItemTurnEvalAsyncCtxConverter{}
}

type ExptItemTurnEvalAsyncCtxConverter struct{}

func (ExptItemTurnEvalAsyncCtxConverter) FromDO(actx *entity.EvalAsyncCtx) ([]byte, error) {
	bytes, err := json.Marshal(actx)
	if err != nil {
		return nil, errorx.Wrapf(err, "EvalAsyncCtx json marshal failed")
	}
	return bytes, nil
}

func (ExptItemTurnEvalAsyncCtxConverter) ToDO(b []byte) (*entity.EvalAsyncCtx, error) {
	actx := &entity.EvalAsyncCtx{}
	bytes := toBytes(b)
	if err := lo.TernaryF(
		len(bytes) > 0,
		func() error { return json.Unmarshal(bytes, actx) },
		func() error { return nil },
	); err != nil {
		return nil, errorx.Wrapf(err, "QuotaSpaceExpt json unmarshal failed")
	}
	return actx, nil
}
