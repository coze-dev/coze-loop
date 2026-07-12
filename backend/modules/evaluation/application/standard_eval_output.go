// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"sort"
	"strconv"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

const standardEvalOutputContentTypeJSON = "application/json"
const maxStandardEvalOutputMGetItemIDs = 100

func (e *experimentApplication) MGetExperimentStandardEvalOutputs(ctx context.Context, req *expt.MGetExperimentStandardEvalOutputsRequest) (*expt.MGetExperimentStandardEvalOutputsResponse, error) {
	if req == nil || len(req.GetItemIds()) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("item_ids is empty"))
	}
	if len(req.GetItemIds()) > maxStandardEvalOutputMGetItemIDs {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("item_ids exceeds maximum of 100"))
	}
	if err := e.authStandardEvalOutput(ctx, req.GetWorkspaceID(), req.GetAPIKey()); err != nil {
		return nil, err
	}

	param := &entity.MGetExperimentResultParam{
		SpaceID:        req.GetWorkspaceID(),
		ExptIDs:        []int64{req.GetExptID()},
		BaseExptID:     gptr.Of(req.GetExptID()),
		ItemIDs:        req.GetItemIds(),
		UseAccelerator: false,
	}

	result, err := e.resultSvc.MGetExperimentResult(ctx, param)
	if err != nil {
		return nil, err
	}

	items, err := buildItemStandardEvalOutputs(experimentReportItemResults(result), standardEvalOutputBuildOptions{ExptID: req.GetExptID()})
	if err != nil {
		return nil, err
	}
	sortStandardItemsByRequestedItemIDs(items, req.GetItemIds())

	return &expt.MGetExperimentStandardEvalOutputsResponse{Items: items, BaseResp: base.NewBaseResp()}, nil
}

func (e *experimentApplication) ListExperimentStandardEvalOutputs(ctx context.Context, req *expt.ListExperimentStandardEvalOutputsRequest) (*expt.ListExperimentStandardEvalOutputsResponse, error) {
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	if err := e.authStandardEvalOutput(ctx, req.GetWorkspaceID(), req.GetAPIKey()); err != nil {
		return nil, err
	}

	param := &entity.MGetExperimentResultParam{
		SpaceID:        req.GetWorkspaceID(),
		ExptIDs:        []int64{req.GetExptID()},
		BaseExptID:     gptr.Of(req.GetExptID()),
		Page:           entity.NewPage(int(req.GetPageNumber()), int(req.GetPageSize())),
		UseAccelerator: true,
	}

	result, err := e.resultSvc.MGetExperimentResult(ctx, param)
	if err != nil {
		return nil, err
	}

	items, err := buildItemStandardEvalOutputs(experimentReportItemResults(result), standardEvalOutputBuildOptions{ExptID: req.GetExptID()})
	if err != nil {
		return nil, err
	}

	return &expt.ListExperimentStandardEvalOutputsResponse{Items: items, Total: gptr.Of(result.Total), BaseResp: base.NewBaseResp()}, nil
}

func (e *experimentApplication) authStandardEvalOutput(ctx context.Context, workspaceID int64, apiKey string) error {
	if apiKey != "" && e.configer != nil && apiKey == e.configer.GetStandardEvalOutputAPIKey(ctx) {
		return nil
	}
	return e.auth.Authorization(ctx, &rpc.AuthorizationParam{
		ObjectID:      strconv.FormatInt(workspaceID, 10),
		SpaceID:       workspaceID,
		ActionObjects: []*rpc.ActionObject{{Action: gptr.Of(consts.ActionReadExpt), EntityType: gptr.Of(rpc.AuthEntityType_Space)}},
	})
}

func sortStandardItemsByRequestedItemIDs(items []*expt.ItemStandardEvalOutput, itemIDs []int64) {
	order := make(map[int64]int, len(itemIDs))
	for i, id := range itemIDs {
		if _, ok := order[id]; !ok {
			order[id] = i
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		oi, okI := order[items[i].GetItemID()]
		oj, okJ := order[items[j].GetItemID()]
		if okI && okJ {
			return oi < oj
		}
		if okI != okJ {
			return okI
		}
		return items[i].GetItemID() < items[j].GetItemID()
	})
}

func experimentReportItemResults(r *entity.MGetExperimentReportResult) []*entity.ItemResult {
	if r == nil {
		return nil
	}
	return r.ItemResults
}

type standardEvalOutputBuildOptions struct{ ExptID int64 }

type standardEvalOutputJSON struct {
	Source any `json:"source,omitempty"`
	Detail any `json:"detail,omitempty"`
	Rounds any `json:"rounds,omitempty"`
	Agent  any `json:"agent,omitempty"`
	Output any `json:"output,omitempty"`
	Eval   any `json:"eval,omitempty"`
	Extra  any `json:"extra,omitempty"`
}

func buildItemStandardEvalOutputs(itemResults []*entity.ItemResult, opt standardEvalOutputBuildOptions) ([]*expt.ItemStandardEvalOutput, error) {
	items := make([]*expt.ItemStandardEvalOutput, 0, len(itemResults))
	for _, item := range itemResults {
		if item == nil {
			continue
		}
		out, err := buildItemStandardEvalOutput(item, opt)
		if err != nil {
			return nil, err
		}
		items = append(items, out)
	}
	return items, nil
}

func buildItemStandardEvalOutput(item *entity.ItemResult, opt standardEvalOutputBuildOptions) (*expt.ItemStandardEvalOutput, error) {
	std := buildStandardEvalOutputJSON(item, opt)
	res := &expt.ItemStandardEvalOutput{ExptID: opt.ExptID, ItemID: item.ItemID, DatasetKey: datasetKeyFromItem(item)}
	if item != nil && item.Ext != nil && item.Ext["item_key"] != "" {
		res.ItemKey = gptr.Of(item.Ext["item_key"])
	}

	var err error
	if res.Source, err = inlineJSONContent(std.Source); err != nil {
		return nil, err
	}
	if res.Detail, err = inlineJSONContent(std.Detail); err != nil {
		return nil, err
	}
	if res.Rounds, err = inlineJSONContent(std.Rounds); err != nil {
		return nil, err
	}
	if res.Agent, err = inlineJSONContent(std.Agent); err != nil {
		return nil, err
	}
	if res.Output, err = inlineJSONContent(std.Output); err != nil {
		return nil, err
	}
	if res.Eval, err = inlineJSONContent(std.Eval); err != nil {
		return nil, err
	}
	if res.Extra, err = inlineJSONContent(std.Extra); err != nil {
		return nil, err
	}
	return res, nil
}

func inlineJSONContent(val any) (*expt.StandardEvalOutputContent, error) {
	text, err := json.MarshalString(val)
	if err != nil {
		return nil, err
	}
	return &expt.StandardEvalOutputContent{
		ContentType: gptr.Of(standardEvalOutputContentTypeJSON),
		Text:        gptr.Of(text),
		Storage:     expt.StandardEvalOutputContentStoragePtr(expt.StandardEvalOutputContentStorage_Inline),
		Bytes:       gptr.Of(int64(len(text))),
	}, nil
}

func datasetKeyFromItem(item *entity.ItemResult) string {
	if item == nil || item.Ext == nil {
		return ""
	}
	return item.Ext["dataset_key"]
}

func buildStandardEvalOutputJSON(item *entity.ItemResult, opt standardEvalOutputBuildOptions) standardEvalOutputJSON {
	if std, ok := parseReportedStandardEvalOutput(item, opt); ok {
		return std
	}
	return standardEvalOutputJSON{
		Source: map[string]any{"type": "evaluation", "expt_id": opt.ExptID, "item_id": item.ItemID, "dataset_key": datasetKeyFromItem(item), "item_key": itemKeyFromItem(item)},
		Detail: map[string]any{"item_id": item.ItemID, "item_key": itemKeyFromItem(item), "item_index": item.ItemIndex, "system_info": item.SystemInfo, "turn_count": len(standardTurns(item, opt.ExptID))},
		Rounds: standardTurns(item, opt.ExptID),
		Agent:  standardAgent(item, opt.ExptID),
		Output: standardOutput(item, opt.ExptID),
		Eval:   standardEval(item, opt.ExptID),
		Extra:  standardExtra(item),
	}
}

func itemKeyFromItem(item *entity.ItemResult) string {
	if item == nil || item.Ext == nil {
		return ""
	}
	return item.Ext["item_key"]
}

func parseReportedStandardEvalOutput(item *entity.ItemResult, opt standardEvalOutputBuildOptions) (standardEvalOutputJSON, bool) {
	for _, payload := range standardPayloads(item, opt.ExptID) {
		if payload == nil || payload.TargetOutput == nil || payload.TargetOutput.EvalTargetRecord == nil || payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData == nil {
			continue
		}
		fields := payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields
		actualOutput := fields[consts.EvalTargetOutputFieldKeyActualOutput]
		if actualOutput == nil || actualOutput.GetText() == "" || !json.Valid([]byte(actualOutput.GetText())) {
			continue
		}
		parsed := map[string]any{}
		if err := json.Unmarshal([]byte(actualOutput.GetText()), &parsed); err != nil || !looksLikeStandardEvalOutput(parsed) {
			continue
		}
		return standardEvalOutputJSON{Source: parsed["source"], Detail: parsed["detail"], Rounds: parsed["rounds"], Agent: parsed["agent"], Output: parsed["output"], Eval: parsed["eval"], Extra: parsed["extra"]}, true
	}
	return standardEvalOutputJSON{}, false
}

func looksLikeStandardEvalOutput(parsed map[string]any) bool {
	if len(parsed) == 0 {
		return false
	}
	_, hasDetailID := parsed["detail_id"]
	_, hasSource := parsed["source"]
	_, hasRounds := parsed["rounds"]
	_, hasOutput := parsed["output"]
	_, hasEval := parsed["eval"]
	_, hasAgent := parsed["agent"]
	// 收窄识别条件，避免普通 JSON actual_output={"output":"..."} 被误判。
	return hasDetailID && hasSource && hasRounds && hasOutput && (hasEval || hasAgent)
}

func standardTurns(item *entity.ItemResult, exptID int64) []map[string]any {
	rounds := make([]map[string]any, 0)
	for _, payload := range standardPayloads(item, exptID) {
		rounds = append(rounds, map[string]any{"turn_id": payload.TurnID, "kind": "eval_set_turn", "eval_set": payload.EvalSet})
	}
	return rounds
}

func standardAgent(item *entity.ItemResult, exptID int64) map[string]any {
	runs := make([]any, 0)
	for _, payload := range standardPayloads(item, exptID) {
		tr := payload.TargetOutput
		if tr == nil || tr.EvalTargetRecord == nil {
			continue
		}
		rec := tr.EvalTargetRecord
		runs = append(runs, map[string]any{"target_record_id": rec.ID, "target_id": rec.TargetID, "target_version_id": rec.TargetVersionID, "experiment_run_id": rec.ExperimentRunID, "status": rec.Status, "trace_id": rec.TraceID, "log_id": rec.LogID, "runtime_param": runtimeParamFromTargetRecord(rec)})
	}
	return map[string]any{"runs": runs}
}

func standardOutput(item *entity.ItemResult, exptID int64) map[string]any {
	turns := map[string]any{}
	for _, payload := range standardPayloads(item, exptID) {
		tr := payload.TargetOutput
		if tr == nil || tr.EvalTargetRecord == nil || tr.EvalTargetRecord.EvalTargetOutputData == nil {
			continue
		}
		data := tr.EvalTargetRecord.EvalTargetOutputData
		turns[strconv.FormatInt(payload.TurnID, 10)] = map[string]any{"target_record_id": tr.EvalTargetRecord.ID, "output_fields": data.OutputFields, "ext": data.Ext, "usage": data.EvalTargetUsage, "error": data.EvalTargetRunError, "time_consuming_ms": data.TimeConsumingMS}
	}
	return map[string]any{"turns": turns}
}

func standardEval(item *entity.ItemResult, exptID int64) map[string]any {
	turns := map[string]any{}
	for _, payload := range standardPayloads(item, exptID) {
		eo := payload.EvaluatorOutput
		if eo == nil {
			continue
		}
		records := map[string]*entity.EvaluatorRecord{}
		for key, record := range eo.EvaluatorRecords {
			if record == nil {
				continue
			}
			records[strconv.FormatInt(key, 10)] = record
		}
		turns[strconv.FormatInt(payload.TurnID, 10)] = map[string]any{"weighted_score": eo.WeightedScore, "evaluator_records": records}
	}
	return map[string]any{"turns": turns}
}

func standardExtra(item *entity.ItemResult) map[string]any {
	turns := map[string]any{}
	for _, turnResult := range item.TurnResults {
		if turnResult == nil {
			continue
		}
		for _, er := range turnResult.ExperimentResults {
			if er == nil || er.Payload == nil {
				continue
			}
			turns[strconv.FormatInt(er.Payload.TurnID, 10)] = map[string]any{"system_info": er.Payload.SystemInfo, "annotations": er.Payload.AnnotateResult, "analysis": er.Payload.AnalysisRecord}
		}
	}
	return map[string]any{"item_ext": item.Ext, "turns": turns}
}

func standardPayloads(item *entity.ItemResult, exptID int64) []*entity.ExperimentTurnPayload {
	payloads := make([]*entity.ExperimentTurnPayload, 0)
	for _, turnResult := range item.TurnResults {
		if turnResult == nil {
			continue
		}
		for _, er := range turnResult.ExperimentResults {
			if er == nil || er.ExperimentID != exptID || er.Payload == nil {
				continue
			}
			payloads = append(payloads, er.Payload)
		}
	}
	return payloads
}

func runtimeParamFromTargetRecord(record *entity.EvalTargetRecord) map[string]string {
	if record == nil || record.EvalTargetInputData == nil || record.EvalTargetInputData.Ext == nil {
		return nil
	}
	if v, ok := record.EvalTargetInputData.Ext[consts.TargetExecuteExtRuntimeParamKey]; ok {
		return map[string]string{consts.TargetExecuteExtRuntimeParamKey: v}
	}
	return nil
}
