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

func (e *experimentApplication) MGetExperimentStandardEvalOutputs(ctx context.Context, req *expt.MGetExperimentStandardEvalOutputsRequest) (*expt.MGetExperimentStandardEvalOutputsResponse, error) {
	if req == nil || len(req.GetItemIds()) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("item_ids is empty"))
	}
	if err := e.authStandardEvalOutput(ctx, req.GetWorkspaceID(), req.GetAPIKey()); err != nil {
		return nil, err
	}

	param := &entity.MGetExperimentResultParam{
		SpaceID:        req.GetWorkspaceID(),
		ExptIDs:        []int64{req.GetExptID()},
		BaseExptID:     gptr.Of(req.GetExptID()),
		Page:           entity.NewPage(1, maxInt(len(req.GetItemIds()), 1)),
		UseAccelerator: true,
		FullTrajectory: req.GetFullTrajectory(),
		FilterAccelerators: map[int64]*entity.ExptTurnResultFilterAccelerator{
			req.GetExptID(): {
				ItemIDs: []*entity.FieldFilter{{Op: "IN", Values: int64sToAnys(req.GetItemIds())}},
			},
		},
	}

	result, err := e.resultSvc.MGetExperimentResult(ctx, param)
	if err != nil {
		return nil, err
	}

	items, err := buildItemStandardEvalOutputs(experimentReportItemResults(result), standardEvalOutputBuildOptions{
		ExptID:     req.GetExptID(),
		ExptRunID:  req.GetExptRunID(),
		Sections:   req.GetSections(),
		IncludeRaw: req.GetIncludeRaw(),
	})
	if err != nil {
		return nil, err
	}
	sortStandardItemsByRequestedItemIDs(items, req.GetItemIds())

	return &expt.MGetExperimentStandardEvalOutputsResponse{
		Items:    items,
		BaseResp: base.NewBaseResp(),
	}, nil
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
		FullTrajectory: req.GetFullTrajectory(),
	}

	result, err := e.resultSvc.MGetExperimentResult(ctx, param)
	if err != nil {
		return nil, err
	}

	items, err := buildItemStandardEvalOutputs(experimentReportItemResults(result), standardEvalOutputBuildOptions{
		ExptID:     req.GetExptID(),
		ExptRunID:  req.GetExptRunID(),
		Sections:   req.GetSections(),
		IncludeRaw: req.GetIncludeRaw(),
	})
	if err != nil {
		return nil, err
	}

	return &expt.ListExperimentStandardEvalOutputsResponse{
		Items:    items,
		Total:    gptr.Of(result.Total),
		BaseResp: base.NewBaseResp(),
	}, nil
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

func int64sToAnys(vals []int64) []any {
	res := make([]any, 0, len(vals))
	for _, v := range vals {
		res = append(res, v)
	}
	return res
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

type standardEvalOutputBuildOptions struct {
	ExptID     int64
	ExptRunID  int64
	Sections   []string
	IncludeRaw bool
}

type standardEvalOutputJSON struct {
	DetailID string `json:"detail_id,omitempty"`
	Source   any    `json:"source,omitempty"`
	Detail   any    `json:"detail,omitempty"`
	Rounds   any    `json:"rounds,omitempty"`
	Agent    any    `json:"agent,omitempty"`
	Output   any    `json:"output,omitempty"`
	Eval     any    `json:"eval,omitempty"`
	Extra    any    `json:"extra,omitempty"`
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
	raw, err := json.MarshalString(std)
	if err != nil {
		return nil, err
	}

	res := &expt.ItemStandardEvalOutput{
		ExptID:    opt.ExptID,
		ExptRunID: opt.ExptRunID,
		ItemID:    item.ItemID,
		DetailID:  gptr.Of(std.DetailID),
	}
	if item != nil && item.Ext != nil && item.Ext["item_key"] != "" {
		res.ItemKey = gptr.Of(item.Ext["item_key"])
	}
	if opt.IncludeRaw {
		res.RawJSON = gptr.Of(raw)
	}

	sections := sectionSet(opt.Sections)
	setJSON := func(name string, val any, setter func(string)) error {
		if len(sections) > 0 && !sections[name] {
			return nil
		}
		s, err := json.MarshalString(val)
		if err != nil {
			return err
		}
		setter(s)
		return nil
	}
	if err := setJSON("source", std.Source, func(v string) { res.Source = gptr.Of(v) }); err != nil {
		return nil, err
	}
	if err := setJSON("detail", std.Detail, func(v string) { res.Detail = gptr.Of(v) }); err != nil {
		return nil, err
	}
	if err := setJSON("rounds", std.Rounds, func(v string) { res.Rounds = gptr.Of(v) }); err != nil {
		return nil, err
	}
	if err := setJSON("agent", std.Agent, func(v string) { res.Agent = gptr.Of(v) }); err != nil {
		return nil, err
	}
	if err := setJSON("output", std.Output, func(v string) { res.Output = gptr.Of(v) }); err != nil {
		return nil, err
	}
	if err := setJSON("eval", std.Eval, func(v string) { res.Eval = gptr.Of(v) }); err != nil {
		return nil, err
	}
	if err := setJSON("extra", std.Extra, func(v string) { res.Extra = gptr.Of(v) }); err != nil {
		return nil, err
	}
	return res, nil
}

func sectionSet(sections []string) map[string]bool {
	if len(sections) == 0 {
		return nil
	}
	res := make(map[string]bool, len(sections))
	for _, s := range sections {
		res[s] = true
	}
	return res
}

func buildStandardEvalOutputJSON(item *entity.ItemResult, opt standardEvalOutputBuildOptions) standardEvalOutputJSON {
	if std, ok := parseReportedStandardEvalOutput(item, opt); ok {
		return std
	}
	turns := standardTurns(item, opt.ExptID, opt.ExptRunID)
	return standardEvalOutputJSON{
		DetailID: strconv.FormatInt(opt.ExptRunID, 10) + "|" + strconv.FormatInt(item.ItemID, 10),
		Source: map[string]any{
			"type":        "evaluation",
			"expt_id":     opt.ExptID,
			"expt_run_id": opt.ExptRunID,
			"item_id":     item.ItemID,
		},
		Detail: map[string]any{
			"item_id":     item.ItemID,
			"item_index":  item.ItemIndex,
			"system_info": item.SystemInfo,
			"turn_count":  len(turns),
		},
		Rounds: turns,
		Agent:  standardAgent(item, opt.ExptID, opt.ExptRunID),
		Output: standardOutput(item, opt.ExptID, opt.ExptRunID),
		Eval:   standardEval(item, opt.ExptID, opt.ExptRunID),
		Extra:  standardExtra(item),
	}
}

func parseReportedStandardEvalOutput(item *entity.ItemResult, opt standardEvalOutputBuildOptions) (standardEvalOutputJSON, bool) {
	for _, payload := range standardPayloads(item, opt.ExptID, opt.ExptRunID) {
		if payload == nil || payload.TargetOutput == nil || payload.TargetOutput.EvalTargetRecord == nil || payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData == nil {
			continue
		}
		fields := payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields
		actualOutput := fields[consts.EvalTargetOutputFieldKeyActualOutput]
		if actualOutput == nil || actualOutput.GetText() == "" || !json.Valid([]byte(actualOutput.GetText())) {
			continue
		}
		parsed := map[string]any{}
		if err := json.Unmarshal([]byte(actualOutput.GetText()), &parsed); err != nil {
			continue
		}
		if !looksLikeStandardEvalOutput(parsed) {
			continue
		}
		std := standardEvalOutputJSON{
			DetailID: stringFromAny(parsed["detail_id"]),
			Source:   parsed["source"],
			Detail:   parsed["detail"],
			Rounds:   parsed["rounds"],
			Agent:    parsed["agent"],
			Output:   parsed["output"],
			Eval:     parsed["eval"],
			Extra:    parsed["extra"],
		}
		if std.DetailID == "" {
			std.DetailID = strconv.FormatInt(opt.ExptRunID, 10) + "|" + strconv.FormatInt(item.ItemID, 10)
		}
		return std, true
	}
	return standardEvalOutputJSON{}, false
}

func looksLikeStandardEvalOutput(parsed map[string]any) bool {
	if len(parsed) == 0 {
		return false
	}
	_, hasDetailID := parsed["detail_id"]
	_, hasRounds := parsed["rounds"]
	_, hasOutput := parsed["output"]
	_, hasEval := parsed["eval"]
	_, hasAgent := parsed["agent"]
	return hasDetailID || hasRounds || hasOutput || hasEval || hasAgent
}

func stringFromAny(v any) string {
	s, _ := v.(string)
	return s
}

func standardTurns(item *entity.ItemResult, exptID, exptRunID int64) []map[string]any {
	rounds := make([]map[string]any, 0)
	for _, turnResult := range item.TurnResults {
		if turnResult == nil {
			continue
		}
		for _, er := range turnResult.ExperimentResults {
			if er == nil || er.ExperimentID != exptID || er.Payload == nil {
				continue
			}
			if !payloadBelongsToRun(er.Payload, exptRunID) {
				continue
			}
			rounds = append(rounds, map[string]any{
				"turn_id":    er.Payload.TurnID,
				"turn_index": turnResult.TurnIndex,
				"kind":       "eval_set_turn",
				"eval_set":   er.Payload.EvalSet,
			})
		}
	}
	return rounds
}

func standardAgent(item *entity.ItemResult, exptID, exptRunID int64) map[string]any {
	agent := map[string]any{"runs": []any{}}
	runs := make([]any, 0)
	for _, payload := range standardPayloads(item, exptID, exptRunID) {
		tr := payload.TargetOutput
		if tr == nil || tr.EvalTargetRecord == nil {
			continue
		}
		rec := tr.EvalTargetRecord
		runs = append(runs, map[string]any{
			"target_record_id":  rec.ID,
			"target_id":         rec.TargetID,
			"target_version_id": rec.TargetVersionID,
			"status":            rec.Status,
			"trace_id":          rec.TraceID,
			"log_id":            rec.LogID,
			"runtime_param":     runtimeParamFromTargetRecord(rec),
		})
	}
	agent["runs"] = runs
	return agent
}

func standardOutput(item *entity.ItemResult, exptID, exptRunID int64) map[string]any {
	output := map[string]any{"turns": map[string]any{}}
	turns := map[string]any{}
	for _, payload := range standardPayloads(item, exptID, exptRunID) {
		tr := payload.TargetOutput
		if tr == nil || tr.EvalTargetRecord == nil || tr.EvalTargetRecord.EvalTargetOutputData == nil {
			continue
		}
		data := tr.EvalTargetRecord.EvalTargetOutputData
		turns[strconv.FormatInt(payload.TurnID, 10)] = map[string]any{
			"target_record_id":  tr.EvalTargetRecord.ID,
			"output_fields":     data.OutputFields,
			"ext":               data.Ext,
			"usage":             data.EvalTargetUsage,
			"error":             data.EvalTargetRunError,
			"time_consuming_ms": data.TimeConsumingMS,
		}
	}
	output["turns"] = turns
	return output
}

func standardEval(item *entity.ItemResult, exptID, exptRunID int64) map[string]any {
	eval := map[string]any{"turns": map[string]any{}}
	turns := map[string]any{}
	for _, payload := range standardPayloads(item, exptID, exptRunID) {
		eo := payload.EvaluatorOutput
		if eo == nil {
			continue
		}
		records := map[string]*entity.EvaluatorRecord{}
		for key, record := range eo.EvaluatorRecords {
			if record == nil || record.ExperimentRunID != exptRunID {
				continue
			}
			records[strconv.FormatInt(key, 10)] = record
		}
		turns[strconv.FormatInt(payload.TurnID, 10)] = map[string]any{
			"weighted_score":    eo.WeightedScore,
			"evaluator_records": records,
		}
	}
	eval["turns"] = turns
	return eval
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
			turns[strconv.FormatInt(er.Payload.TurnID, 10)] = map[string]any{
				"system_info": er.Payload.SystemInfo,
				"annotations": er.Payload.AnnotateResult,
				"analysis":    er.Payload.AnalysisRecord,
			}
		}
	}
	return map[string]any{
		"item_ext": item.Ext,
		"turns":    turns,
	}
}

func standardPayloads(item *entity.ItemResult, exptID, exptRunID int64) []*entity.ExperimentTurnPayload {
	payloads := make([]*entity.ExperimentTurnPayload, 0)
	for _, turnResult := range item.TurnResults {
		if turnResult == nil {
			continue
		}
		for _, er := range turnResult.ExperimentResults {
			if er == nil || er.ExperimentID != exptID || er.Payload == nil {
				continue
			}
			if !payloadBelongsToRun(er.Payload, exptRunID) {
				continue
			}
			payloads = append(payloads, er.Payload)
		}
	}
	return payloads
}

func payloadBelongsToRun(payload *entity.ExperimentTurnPayload, exptRunID int64) bool {
	if payload == nil {
		return false
	}
	if payload.TargetOutput != nil && payload.TargetOutput.EvalTargetRecord != nil {
		return payload.TargetOutput.EvalTargetRecord.ExperimentRunID == exptRunID
	}
	if payload.EvaluatorOutput != nil {
		for _, record := range payload.EvaluatorOutput.EvaluatorRecords {
			if record != nil && record.ExperimentRunID == exptRunID {
				return true
			}
		}
	}
	return exptRunID == 0
}

func runtimeParamFromTargetRecord(record *entity.EvalTargetRecord) map[string]string {
	if record == nil || record.EvalTargetInputData == nil {
		return nil
	}
	if record.EvalTargetInputData.Ext == nil {
		return nil
	}
	if v, ok := record.EvalTargetInputData.Ext[consts.TargetExecuteExtRuntimeParamKey]; ok {
		return map[string]string{consts.TargetExecuteExtRuntimeParamKey: v}
	}
	return nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
