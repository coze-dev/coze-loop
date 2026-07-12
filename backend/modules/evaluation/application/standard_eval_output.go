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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

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
		SpaceID:                   req.GetWorkspaceID(),
		ExptIDs:                   []int64{req.GetExptID()},
		BaseExptID:                gptr.Of(req.GetExptID()),
		ItemIDs:                   req.GetItemIds(),
		UseAccelerator:            false,
		FullTrajectory:            true,
		LoadEvaluatorFullContent:  gptr.Of(true),
		LoadEvalTargetFullContent: gptr.Of(true),
	}

	result, err := e.resultSvc.MGetExperimentResult(ctx, param)
	if err != nil {
		return nil, err
	}

	items, err := buildItemStandardEvalOutputs(result, standardEvalOutputBuildOptions{ExptID: req.GetExptID()})
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
		SpaceID:                   req.GetWorkspaceID(),
		ExptIDs:                   []int64{req.GetExptID()},
		BaseExptID:                gptr.Of(req.GetExptID()),
		Page:                      entity.NewPage(int(req.GetPageNumber()), int(req.GetPageSize())),
		UseAccelerator:            true,
		FullTrajectory:            true,
		LoadEvaluatorFullContent:  gptr.Of(true),
		LoadEvalTargetFullContent: gptr.Of(true),
	}

	result, err := e.resultSvc.MGetExperimentResult(ctx, param)
	if err != nil {
		return nil, err
	}

	items, err := buildItemStandardEvalOutputs(result, standardEvalOutputBuildOptions{ExptID: req.GetExptID()})
	if err != nil {
		return nil, err
	}

	return &expt.ListExperimentStandardEvalOutputsResponse{Items: items, Total: gptr.Of(result.Total), BaseResp: base.NewBaseResp()}, nil
}

func (e *experimentApplication) authStandardEvalOutput(ctx context.Context, workspaceID int64, apiKey string) error {
	// TODO: standard eval output 鉴权临时移除（BOE 自测阶段直接放行），后续恢复 api_key 校验。
	return nil
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
	ExptID               int64
	EvaluatorByVersionID map[int64]*entity.ColumnEvaluator
}

type standardEvalOutputJSON struct {
	Source any `json:"source,omitempty"`
	Detail any `json:"detail,omitempty"`
	Rounds any `json:"rounds,omitempty"`
	Agent  any `json:"agent,omitempty"`
	Output any `json:"output,omitempty"`
	Eval   any `json:"eval,omitempty"`
	Extra  any `json:"extra,omitempty"`
}

func buildItemStandardEvalOutputs(result *entity.MGetExperimentReportResult, opt standardEvalOutputBuildOptions) ([]*expt.ItemStandardEvalOutput, error) {
	itemResults := experimentReportItemResults(result)
	opt.EvaluatorByVersionID = evaluatorByVersionID(result)
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
	if out, ok := buildReportedItemStandardEvalOutput(item, opt); ok {
		return out, nil
	}
	std := buildStandardEvalOutputJSON(item, opt)
	res := &expt.ItemStandardEvalOutput{ExptID: opt.ExptID, ItemID: item.ItemID, DatasetKey: datasetKeyFromItem(item)}
	if item != nil && item.Ext != nil && item.Ext["item_key"] != "" {
		res.ItemKey = gptr.Of(item.Ext["item_key"])
	}

	var err error
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

func buildReportedItemStandardEvalOutput(item *entity.ItemResult, opt standardEvalOutputBuildOptions) (*expt.ItemStandardEvalOutput, bool) {
	for _, payload := range standardPayloads(item, opt.ExptID) {
		if payload == nil || payload.TargetOutput == nil || payload.TargetOutput.EvalTargetRecord == nil || payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData == nil {
			continue
		}
		fields := payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields
		if !looksLikeStandardEvalOutputFields(fields) {
			continue
		}
		res := &expt.ItemStandardEvalOutput{ExptID: opt.ExptID, ItemID: item.ItemID, DatasetKey: datasetKeyFromItem(item)}
		if item != nil && item.Ext != nil && item.Ext["item_key"] != "" {
			res.ItemKey = gptr.Of(item.Ext["item_key"])
		}
		res.Detail = contentToStandardEvalOutputContent(fields["detail"])
		res.Rounds = contentToStandardEvalOutputContent(fields["rounds"])
		res.Agent = contentToStandardEvalOutputContent(fields["agent"])
		res.Output = contentToStandardEvalOutputContent(fields["output"])
		res.Eval = contentToStandardEvalOutputContent(fields["eval"])
		res.Extra = contentToStandardEvalOutputContent(fields["extra"])
		return res, true
	}
	return nil, false
}

func inlineJSONContent(val any) (*expt.StandardEvalOutputContent, error) {
	text, err := json.MarshalString(val)
	if err != nil {
		return nil, err
	}
	return &expt.StandardEvalOutputContent{
		Text:           gptr.Of(text),
		ContentOmitted: gptr.Of(false),
	}, nil
}

func contentToStandardEvalOutputContent(content *entity.Content) *expt.StandardEvalOutputContent {
	if content == nil {
		return nil
	}
	res := &expt.StandardEvalOutputContent{
		Text:           content.Text,
		ContentOmitted: content.ContentOmitted,
		FullContent:    objectStorageToStandardFullContent(content.FullContent, content.FullContentBytes),
	}
	return res
}

func objectStorageToStandardFullContent(storage *entity.ObjectStorage, bytes *int32) *expt.StandardEvalOutputFullContent {
	if storage == nil && bytes == nil {
		return nil
	}
	res := &expt.StandardEvalOutputFullContent{}
	if storage != nil {
		if storage.Provider != nil {
			res.Provider = gptr.Of(storage.Provider.String())
		}
		res.URI = storage.URI
		res.URL = storage.URL
	}
	if bytes != nil {
		res.Bytes = gptr.Of(int64(*bytes))
	}
	return res
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
		Eval:   standardEval(item, opt.ExptID, opt),
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
		if std, ok := parseStandardEvalOutputFields(fields); ok {
			return std, true
		}
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

func parseStandardEvalOutputFields(fields map[string]*entity.Content) (standardEvalOutputJSON, bool) {
	if !looksLikeStandardEvalOutputFields(fields) {
		return standardEvalOutputJSON{}, false
	}

	return standardEvalOutputJSON{
		Source: contentValue(fields["source"]),
		Detail: contentValue(fields["detail"]),
		Rounds: contentValue(fields["rounds"]),
		Agent:  contentValue(fields["agent"]),
		Output: contentValue(fields["output"]),
		Eval:   contentValue(fields["eval"]),
		Extra:  contentValue(fields["extra"]),
	}, true
}

func looksLikeStandardEvalOutputFields(fields map[string]*entity.Content) bool {
	if len(fields) == 0 {
		return false
	}
	_, hasSource := fields["source"]
	_, hasRounds := fields["rounds"]
	_, hasOutput := fields["output"]
	_, hasEval := fields["eval"]
	_, hasAgent := fields["agent"]
	return hasSource && hasRounds && hasOutput && (hasEval || hasAgent)
}

func contentValue(content *entity.Content) any {
	if content == nil {
		return nil
	}
	text := content.GetText()
	if text != "" {
		var parsed any
		if json.Valid([]byte(text)) && json.Unmarshal([]byte(text), &parsed) == nil {
			return parsed
		}
		return text
	}
	return content
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
	payloads := standardPayloads(item, exptID)
	rounds := make([]map[string]any, 0, len(payloads))
	for i, payload := range payloads {
		roundID := standardRoundID(payload)
		rounds = append(rounds, map[string]any{
			"round_id":   roundID,
			"round_no":   i + 1,
			"user_query": userQueryFromPayload(payload),
			"latency":    latencyFromPayload(payload),
			"start_time": startTimeFromPayload(payload),
			"end_time":   endTimeFromPayload(payload),
			"tokens":     tokensFromPayload(payload),
			"context":    contextFromPayload(payload),
		})
	}
	return rounds
}

func standardAgent(item *entity.ItemResult, exptID int64) map[string]any {
	var first *entity.EvalTargetRecord
	runs := make([]any, 0)
	for _, payload := range standardPayloads(item, exptID) {
		tr := payload.TargetOutput
		if tr == nil || tr.EvalTargetRecord == nil {
			continue
		}
		rec := tr.EvalTargetRecord
		if first == nil {
			first = rec
		}
		runs = append(runs, map[string]any{"target_record_id": rec.ID, "experiment_run_id": rec.ExperimentRunID, "status": rec.Status, "trace_id": rec.TraceID, "log_id": rec.LogID})
	}
	runtimeParam := runtimeParamObjectFromTargetRecord(first)
	return map[string]any{
		"agent_id":          int64String(firstTargetID(first)),
		"model_name":        stringFromRuntimeParam(runtimeParam, "model_name", "model", "model_id"),
		"agent_name":        stringFromRuntimeParam(runtimeParam, "agent_name", "agent", "name"),
		"agent_version":     stringFromRuntimeParam(runtimeParam, "agent_version", "version"),
		"thinking_effort":   stringFromRuntimeParam(runtimeParam, "thinking_effort", "effort"),
		"context_window":    stringFromRuntimeParam(runtimeParam, "context_window", "context_window_size", "main_context_window_size"),
		"target_id":         firstTargetID(first),
		"target_version_id": firstTargetVersionID(first),
		"runtime_param":     runtimeParam,
		"runs":              runs,
	}
}

func standardOutput(item *entity.ItemResult, exptID int64) map[string]any {
	payloads := standardPayloads(item, exptID)
	rounds := map[string]any{}
	var detailOutput map[string]*entity.Content
	for _, payload := range payloads {
		tr := payload.TargetOutput
		if tr == nil || tr.EvalTargetRecord == nil || tr.EvalTargetRecord.EvalTargetOutputData == nil {
			continue
		}
		data := tr.EvalTargetRecord.EvalTargetOutputData
		out := data.OutputFields
		if detailOutput == nil {
			detailOutput = out
		}
		rounds[standardRoundID(payload)] = map[string]any{"output": out, "file_diff": []any{}}
	}
	if len(payloads) > 0 {
		last := payloads[len(payloads)-1]
		if last.TargetOutput != nil && last.TargetOutput.EvalTargetRecord != nil && last.TargetOutput.EvalTargetRecord.EvalTargetOutputData != nil {
			detailOutput = last.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields
		}
	}
	return map[string]any{"detail": map[string]any{"file_diff": []any{}, "output": detailOutput}, "rounds": rounds}
}

func standardEval(item *entity.ItemResult, exptID int64, opt standardEvalOutputBuildOptions) map[string]any {
	payloads := standardPayloads(item, exptID)
	rounds := map[string]any{}
	var detailEval map[string]any
	for _, payload := range payloads {
		evalResult := standardEvalResult(payload, opt)
		rounds[standardRoundID(payload)] = map[string]any{"run_status": turnRunStatus(payload), "eval_result": evalResult}
		detailEval = evalResult
	}
	if detailEval == nil {
		detailEval = map[string]any{"type": "score", "score": nil, "reason": "", "results": map[string]any{}}
	}
	return map[string]any{"task_config": standardEvalTaskConfig(item), "detail": map[string]any{"run_status": itemRunStatus(item), "eval_result": detailEval}, "rounds": rounds}
}

func standardExtra(item *entity.ItemResult) map[string]any {
	return map[string]any{}
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

func standardRoundID(payload *entity.ExperimentTurnPayload) string {
	if payload == nil {
		return "round_0"
	}
	return "round_" + strconv.FormatInt(payload.TurnID, 10)
}

func userQueryFromPayload(payload *entity.ExperimentTurnPayload) string {
	if payload == nil || payload.EvalSet == nil || payload.EvalSet.Turn == nil {
		return ""
	}
	for _, field := range payload.EvalSet.Turn.FieldDataList {
		if field == nil || field.Content == nil {
			continue
		}
		key := field.Key
		if key == "query" || key == "input" || key == "user_query" {
			return field.Content.GetText()
		}
	}
	return ""
}

func latencyFromPayload(payload *entity.ExperimentTurnPayload) int64 {
	if payload == nil || payload.TargetOutput == nil || payload.TargetOutput.EvalTargetRecord == nil || payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData == nil || payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.TimeConsumingMS == nil {
		return 0
	}
	return *payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.TimeConsumingMS
}

func tokensFromPayload(payload *entity.ExperimentTurnPayload) map[string]any {
	var usage *entity.EvalTargetUsage
	if payload != nil && payload.TargetOutput != nil && payload.TargetOutput.EvalTargetRecord != nil && payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData != nil {
		usage = payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.EvalTargetUsage
	}
	return map[string]any{
		"prompt_tokens":                gptr.Indirect(gptr.Of(usage.GetInputTokens())),
		"completion_tokens":            gptr.Indirect(gptr.Of(usage.GetOutputTokens())),
		"total_tokens":                 gptr.Indirect(gptr.Of(usage.GetTotalTokens())),
		"reasoning_tokens":             0,
		"input_cached_tokens":          0,
		"input_creation_cached_tokens": 0,
	}
}

func contextFromPayload(payload *entity.ExperimentTurnPayload) map[string]any {
	ctx := map[string]any{"log_id": "", "message_id": "", "thread_id": "", "trace_id": "", "start_time": int64(0), "end_time": int64(0)}
	if payload == nil {
		return ctx
	}
	if payload.SystemInfo != nil && payload.SystemInfo.LogID != nil {
		ctx["log_id"] = *payload.SystemInfo.LogID
	}
	if payload.TargetOutput != nil && payload.TargetOutput.EvalTargetRecord != nil {
		rec := payload.TargetOutput.EvalTargetRecord
		if rec.LogID != "" {
			ctx["log_id"] = rec.LogID
		}
		ctx["trace_id"] = rec.TraceID
	}
	return ctx
}

func startTimeFromPayload(payload *entity.ExperimentTurnPayload) int64 { return 0 }

func endTimeFromPayload(payload *entity.ExperimentTurnPayload) int64 { return 0 }

func firstTargetID(record *entity.EvalTargetRecord) int64 {
	if record == nil {
		return 0
	}
	return record.TargetID
}

func firstTargetVersionID(record *entity.EvalTargetRecord) int64 {
	if record == nil {
		return 0
	}
	return record.TargetVersionID
}

func int64String(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

func runtimeParamObjectFromTargetRecord(record *entity.EvalTargetRecord) map[string]any {
	if record == nil || record.EvalTargetInputData == nil || record.EvalTargetInputData.Ext == nil {
		return nil
	}
	raw := record.EvalTargetInputData.Ext[consts.TargetExecuteExtRuntimeParamKey]
	if raw == "" {
		return nil
	}
	parsed := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return map[string]any{consts.TargetExecuteExtRuntimeParamKey: raw}
	}
	return parsed
}

func stringFromRuntimeParam(runtimeParam map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := runtimeParam[key]; ok && v != nil {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				return strconv.FormatInt(int64(t), 10)
			case int64:
				return strconv.FormatInt(t, 10)
			case int:
				return strconv.Itoa(t)
			}
		}
	}
	return ""
}

func standardEvalTaskConfig(item *entity.ItemResult) map[string]any {
	items := make([]map[string]any, 0, 1)
	entry := map[string]any{"dataset_key": datasetKeyFromItem(item)}
	if k := itemKeyFromItem(item); k != "" {
		entry["item_key"] = k
	}
	items = append(items, entry)
	return map[string]any{"items": items, "mode": "", "max_round": 0}
}

func standardEvalResult(payload *entity.ExperimentTurnPayload, opt standardEvalOutputBuildOptions) map[string]any {
	results := map[string]any{}
	var score any
	var reason string
	if payload != nil && payload.EvaluatorOutput != nil {
		if payload.EvaluatorOutput.WeightedScore != nil {
			score = *payload.EvaluatorOutput.WeightedScore
		}
		for key, record := range payload.EvaluatorOutput.EvaluatorRecords {
			if record == nil {
				continue
			}
			resultKey := evaluatorResultKey(key, record)
			if score == nil && record.GetScore() != nil {
				score = *record.GetScore()
			}
			if reason == "" {
				reason = record.GetReasoning()
			}
			results[resultKey] = map[string]any{
				"evaluator_name":    evaluatorName(opt, key, record),
				"evaluator_version": evaluatorVersion(opt, key, record),
				"evaluator_alias":   record.Alias,
				"type":              "score",
				"score":             record.GetScore(),
				"reason":            record.GetReasoning(),
			}
		}
	}
	return map[string]any{"type": "score", "score": score, "reason": reason, "results": results}
}

func evaluatorResultKey(key int64, record *entity.EvaluatorRecord) string {
	if record != nil {
		if record.Alias != "" {
			return record.Alias
		}
		if record.InlineKey != "" {
			return record.InlineKey
		}
	}
	return strconv.FormatInt(key, 10)
}

func evaluatorName(opt standardEvalOutputBuildOptions, key int64, record *entity.EvaluatorRecord) string {
	if meta := opt.EvaluatorByVersionID[key]; meta != nil && meta.Name != nil {
		return *meta.Name
	}
	return ""
}

func evaluatorVersion(opt standardEvalOutputBuildOptions, key int64, record *entity.EvaluatorRecord) string {
	if meta := opt.EvaluatorByVersionID[key]; meta != nil && meta.Version != nil {
		return *meta.Version
	}
	return ""
}

func turnRunStatus(payload *entity.ExperimentTurnPayload) map[string]any {
	status := "unknown"
	failedReason := ""
	if payload != nil && payload.SystemInfo != nil {
		status = turnRunStateString(payload.SystemInfo.TurnRunState)
		if payload.SystemInfo.Error != nil && payload.SystemInfo.Error.Message != nil {
			failedReason = *payload.SystemInfo.Error.Message
		}
	}
	return map[string]any{"status": status, "failed_reason": failedReason}
}

func itemRunStatus(item *entity.ItemResult) map[string]any {
	status := "unknown"
	failedReason := ""
	if item != nil && item.SystemInfo != nil {
		status = itemRunStateString(item.SystemInfo.RunState)
		if item.SystemInfo.Error != nil && item.SystemInfo.Error.Message != nil {
			failedReason = *item.SystemInfo.Error.Message
		}
	}
	return map[string]any{"status": status, "failed_reason": failedReason}
}

func turnRunStateString(state entity.TurnRunState) string {
	switch state {
	case entity.TurnRunState_Success:
		return "completed"
	case entity.TurnRunState_Fail:
		return "failed"
	case entity.TurnRunState_Processing:
		return "processing"
	case entity.TurnRunState_Queueing:
		return "queueing"
	case entity.TurnRunState_Terminal:
		return "terminated"
	default:
		return "unknown"
	}
}

func itemRunStateString(state entity.ItemRunState) string {
	switch state {
	case entity.ItemRunState_Success:
		return "completed"
	case entity.ItemRunState_Fail:
		return "failed"
	case entity.ItemRunState_Processing:
		return "processing"
	case entity.ItemRunState_Queueing:
		return "queueing"
	case entity.ItemRunState_Terminal:
		return "terminated"
	default:
		return "unknown"
	}
}

func evaluatorByVersionID(result *entity.MGetExperimentReportResult) map[int64]*entity.ColumnEvaluator {
	res := map[int64]*entity.ColumnEvaluator{}
	if result == nil {
		return res
	}
	for _, col := range result.ColumnEvaluators {
		if col != nil {
			res[col.EvaluatorVersionID] = col
		}
	}
	for _, exptCol := range result.ExptColumnEvaluators {
		if exptCol == nil {
			continue
		}
		for _, col := range exptCol.ColumnEvaluators {
			if col != nil {
				res[col.EvaluatorVersionID] = col
			}
		}
	}
	return res
}
