// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func (e *experimentApplication) ListWebhookDelivery(ctx context.Context, req *expt.ListWebhookDeliveryRequest) (*expt.ListWebhookDeliveryResponse, error) {
	workspaceID := req.GetWorkspaceID()
	exptID := req.GetExperimentID()
	if workspaceID <= 0 || exptID <= 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("workspace_id and experiment_id are required"))
	}

	if err := e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(workspaceID, 10),
		SpaceID:       workspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	}); err != nil {
		return nil, err
	}

	exp, err := e.exptRepo.GetByID(ctx, exptID, workspaceID)
	if err != nil {
		return nil, err
	}
	if exp == nil || exp.SpaceID != workspaceID {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("experiment_not_in_workspace"))
	}

	page := entity.NewPage(int(req.GetPageNumber()), int(req.GetPageSize()))
	deliveries, total, err := e.webhookRepo.ListByExptID(ctx, repo.ListDeliveryParams{
		SpaceID: workspaceID,
		ExptID:  exptID,
		Page:    page,
	})
	if err != nil {
		return nil, err
	}

	return &expt.ListWebhookDeliveryResponse{
		Deliveries: experiment.WebhookDeliveryDO2DTOs(deliveries),
		Total:      gptr.Of(total),
		BaseResp:   base.NewBaseResp(),
	}, nil
}
