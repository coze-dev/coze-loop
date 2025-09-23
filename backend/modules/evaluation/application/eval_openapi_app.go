// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/target"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
)

type IEvalOpenAPIApplication = evaluation.EvalOpenAPIService

type EvalOpenAPIApplication struct {
	targetSvc service.IEvalTargetService
	asyncRepo repo.IEvalAsyncRepo
	publisher events.ExptEventPublisher
}

func NewEvalOpenAPIApplication(asyncRepo repo.IEvalAsyncRepo, publisher events.ExptEventPublisher, targetSvc service.IEvalTargetService) IEvalOpenAPIApplication {
	return &EvalOpenAPIApplication{asyncRepo: asyncRepo, publisher: publisher, targetSvc: targetSvc}
}

func (e *EvalOpenAPIApplication) ReportEvalTargetInvokeResult_(ctx context.Context, req *openapi.ReportEvalTargetInvokeResultRequest) (r *openapi.ReportEvalTargetInvokeResultResponse, err error) {
	actx, err := e.asyncRepo.GetEvalAsyncCtx(ctx, strconv.FormatInt(req.GetInvokeID(), 10))
	if err != nil {
		return nil, err
	}

	outputData := target.ToInvokeOutputDataDO(req)
	outputData.TimeConsumingMS = gptr.Of(time.Now().UnixMilli() - actx.AsyncUnixMS)
	if err := e.targetSvc.ReportInvokeRecords(ctx, &entity.ReportTargetRecordParam{
		SpaceID:    req.GetWorkspaceID(),
		RecordID:   req.GetInvokeID(),
		OutputData: outputData,
		Session:    actx.Session,
	}); err != nil {
		return nil, err
	}

	if actx.Event != nil {
		if err := e.publisher.PublishExptRecordEvalEvent(ctx, actx.Event, gptr.Of(time.Second*3)); err != nil {
			return nil, err
		}
	}

	return &openapi.ReportEvalTargetInvokeResultResponse{BaseResp: base.NewBaseResp()}, nil
}
