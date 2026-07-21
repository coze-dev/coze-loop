// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"sort"
	"strconv"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	exptdomain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const maxStandardEvalOutputMGetItemIDs = 100

func (e *experimentApplication) MGetExperimentStandardEvalOutputs(ctx context.Context, req *expt.MGetExperimentStandardEvalOutputsRequest) (*expt.MGetExperimentStandardEvalOutputsResponse, error) {
	if req == nil || len(req.GetItemIds()) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("item_ids is empty"))
	}
	if len(req.GetItemIds()) > maxStandardEvalOutputMGetItemIDs {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("item_ids exceeds maximum of 100"))
	}
	if err := e.authStandardEvalOutput(ctx, req.GetWorkspaceID()); err != nil {
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

	items, err := buildItemStandardEvalOutputs(result, standardEvalOutputBuildOptions{
		ExptID:                   req.GetExptID(),
		SourceTargetIDByTargetID: e.resolveSourceTargetIDs(ctx, req.GetWorkspaceID(), result),
		MQMeta:                   e.resolveStandardEvalOutputMQMeta(ctx, req.GetWorkspaceID(), req.GetExptID()),
	})
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
	if err := e.authStandardEvalOutput(ctx, req.GetWorkspaceID()); err != nil {
		return nil, err
	}

	// item_id_only: 精简查询，items 每项仅填 item_id（单表单列 GROUP BY，不加载轨迹/评测大对象）。
	if req.GetItemIDOnly() {
		itemIDs, err := e.resultSvc.GetItemIDListByExptID(ctx, req.GetExptID(), req.GetWorkspaceID())
		if err != nil {
			return nil, err
		}
		items := make([]*expt.ItemStandardEvalOutput, 0, len(itemIDs))
		for _, id := range itemIDs {
			items = append(items, &expt.ItemStandardEvalOutput{ExptID: gptr.Of(req.GetExptID()), ItemID: gptr.Of(id)})
		}
		return &expt.ListExperimentStandardEvalOutputsResponse{
			Items:    items,
			Total:    gptr.Of(int64(len(items))),
			BaseResp: base.NewBaseResp(),
		}, nil
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

	items, err := buildItemStandardEvalOutputs(result, standardEvalOutputBuildOptions{
		ExptID:                   req.GetExptID(),
		SourceTargetIDByTargetID: e.resolveSourceTargetIDs(ctx, req.GetWorkspaceID(), result),
		MQMeta:                   e.resolveStandardEvalOutputMQMeta(ctx, req.GetWorkspaceID(), req.GetExptID()),
	})
	if err != nil {
		return nil, err
	}

	return &expt.ListExperimentStandardEvalOutputsResponse{Items: items, Total: gptr.Of(result.Total), BaseResp: base.NewBaseResp()}, nil
}

func (e *experimentApplication) authStandardEvalOutput(ctx context.Context, workspaceID int64) error {
	// 走空间级读权限校验；外部 caller（如 stone.cozeloop.eval_analysis_platform）通过 auth_whitelist 放行。
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

// resolveSourceTargetIDs 收集结果中 distinct 的 eval_target_id(=EvalTargetRecord.TargetID)，
// 反查 EvalTarget 得到 target 级恒定的 SourceTargetID(与 version 无关)。
// GetEvalTarget 的 DAO 仅按主键 id 查、不带 space 过滤，这里对返回做 SpaceID 校验，
// 防跨空间越权返回；查询失败 / 跨空间 / 空串均降级为不填，不阻断主链路。
func (e *experimentApplication) resolveSourceTargetIDs(ctx context.Context, spaceID int64, result *entity.MGetExperimentReportResult) map[int64]string {
	out := map[int64]string{}
	if result == nil {
		return out
	}
	for _, item := range experimentReportItemResults(result) {
		if item == nil {
			continue
		}
		for _, turnResult := range item.TurnResults {
			if turnResult == nil {
				continue
			}
			for _, er := range turnResult.ExperimentResults {
				if er == nil || er.Payload == nil || er.Payload.TargetOutput == nil || er.Payload.TargetOutput.EvalTargetRecord == nil {
					continue
				}
				targetID := er.Payload.TargetOutput.EvalTargetRecord.TargetID
				if targetID == 0 {
					continue
				}
				if _, ok := out[targetID]; ok {
					continue
				}
				target, err := e.evalTargetService.GetEvalTarget(ctx, targetID)
				if err != nil {
					logs.CtxWarn(ctx, "resolveSourceTargetIDs GetEvalTarget failed, target_id=%d, err=%v", targetID, err)
					out[targetID] = ""
					continue
				}
				if target == nil || target.SpaceID != spaceID {
					logs.CtxWarn(ctx, "resolveSourceTargetIDs space mismatch or nil, target_id=%d, want_space=%d", targetID, spaceID)
					out[targetID] = ""
					continue
				}
				out[targetID] = target.SourceTargetID
			}
		}
	}
	return out
}

// resolveStandardEvalOutputMQMeta 加载实验详情，抽取实验级 MQ 元信息（与 buildItemCompleteEvent 对齐）。
// 加载失败时返回 nil、降级为不填 MQ 字段，不阻断标准输出主链路。
func (e *experimentApplication) resolveStandardEvalOutputMQMeta(ctx context.Context, spaceID, exptID int64) *standardEvalOutputMQMeta {
	session := entity.NewSession(ctx)
	expt, err := e.manager.GetDetail(ctx, exptID, spaceID, session)
	if err != nil || expt == nil {
		logs.CtxWarn(ctx, "resolveStandardEvalOutputMQMeta GetDetail failed, expt_id=%d, space_id=%d, err=%v", exptID, spaceID, err)
		return nil
	}
	meta := &standardEvalOutputMQMeta{
		ExptWorkspaceID:    expt.SpaceID,
		ExptRunID:          expt.LatestRunID,
		ExperimentGroupKey: expt.ExperimentGroupKey,
		EvalTargetID:       expt.TargetID,
		PrimaryEvalSetID:   expt.EvalSetID,
		EvalSetByID:        map[int64]*entity.EvaluationSet{},
	}
	if expt.Target != nil {
		meta.EvalTargetWorkspaceID = expt.Target.SpaceID
		meta.SourceTargetID = expt.Target.SourceTargetID
	}
	if expt.CreatedAt != nil {
		meta.ExptCreateTime = expt.CreatedAt.Unix()
	}
	// 归属集详情：多评测集从 EvalSetDetails 收集，单评测集/老实验用主集 EvalSet。
	// dataset_workspace_id 取任一集的 SpaceID（同空间场景与 expt.SpaceID 一致）。
	for _, d := range expt.EvalSetDetails {
		if d != nil && d.EvalSet != nil {
			meta.EvalSetByID[d.EvalSetID] = d.EvalSet
			if meta.DatasetWorkspaceID == 0 {
				meta.DatasetWorkspaceID = d.EvalSet.SpaceID
			}
		}
	}
	if expt.EvalSet != nil {
		meta.EvalSetByID[expt.EvalSet.ID] = expt.EvalSet
		if meta.DatasetWorkspaceID == 0 {
			meta.DatasetWorkspaceID = expt.EvalSet.SpaceID
		}
	}
	if meta.DatasetWorkspaceID == 0 {
		meta.DatasetWorkspaceID = expt.SpaceID
	}
	return meta
}

type standardEvalOutputBuildOptions struct {
	ExptID               int64
	EvaluatorByVersionID map[int64]*entity.ColumnEvaluator
	// SourceTargetIDByTargetID: eval_target_id(=EvalTargetRecord.TargetID) -> 业务侧原始对象 ID。
	// 由 application 层反查 EvalTarget 预先解析，纯函数 builder 只读；缺失时对应 source_target_id 留空。
	SourceTargetIDByTargetID map[int64]string
	// MQMeta: 实验级 MQ 元信息（同实验所有 item 相同），由 application 层加载 Experiment 详情预先解析，
	// 与 item-complete(success) MQ 消息体对齐；nil 时对应 MQ 字段留空、不阻断主链路。
	MQMeta *standardEvalOutputMQMeta
}

// standardEvalOutputMQMeta 承载实验级 MQ 元信息，取值与 buildItemCompleteEvent 对齐。
// dataset_version_id / dataset_version_name / dataset_key 按 item 归属集分流（多评测集），
// 故 per-item 部分放在 builder 里从 payload 取，此处仅承载实验级恒定字段。
type standardEvalOutputMQMeta struct {
	ExptWorkspaceID       int64
	ExptRunID             int64
	ExperimentGroupKey    string
	EvalTargetID          int64
	EvalTargetWorkspaceID int64
	SourceTargetID        string
	DatasetWorkspaceID    int64
	// ExptCreateTime: 实验创建时间（秒），来源 experiment.created_at。
	ExptCreateTime int64
	// EvalSetByID: 归属集 id -> EvaluationSet（含 version），用于按 item 的 dataset_id 分流取版本信息。
	EvalSetByID map[int64]*entity.EvaluationSet
	// PrimaryEvalSetID: 主集 id（单评测集/老实验回退用）。
	PrimaryEvalSetID int64
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
	res := newItemStandardEvalOutput(item, opt)
	if itemKey := itemKeyFromItem(item); itemKey != "" {
		res.ItemKey = gptr.Of(itemKey)
	}
	if !isItemStandardEvalOutputContentReady(item) {
		return res, nil
	}
	if out, ok := buildReportedItemStandardEvalOutput(item, opt); ok {
		return out, nil
	}
	std := buildStandardEvalOutputJSON(item, opt)

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
		res := newItemStandardEvalOutput(item, opt)
		if itemKey := itemKeyFromItem(item); itemKey != "" {
			res.ItemKey = gptr.Of(itemKey)
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

func isItemStandardEvalOutputContentReady(item *entity.ItemResult) bool {
	return item != nil && item.SystemInfo != nil && item.SystemInfo.RunState == entity.ItemRunState_Success
}

func newItemStandardEvalOutput(item *entity.ItemResult, opt standardEvalOutputBuildOptions) *expt.ItemStandardEvalOutput {
	res := &expt.ItemStandardEvalOutput{ExptID: gptr.Of(opt.ExptID)}
	if dk := datasetKeyFromItem(item); dk != "" {
		res.DatasetKey = gptr.Of(dk)
	}
	if item != nil {
		res.ItemID = gptr.Of(item.ItemID)
		if item.SystemInfo != nil {
			status := exptdomain.ItemRunState(item.SystemInfo.RunState)
			res.Status = &status
			if item.SystemInfo.EndTime != nil {
				res.ItemEndTime = gptr.Of(item.SystemInfo.EndTime.Unix())
			}
		}
	}
	fillStandardEvalOutputMQMeta(res, item, opt)
	return res
}

// fillStandardEvalOutputMQMeta 把实验级 + item 级 MQ 元信息平铺到顶层字段（与 item-complete MQ 对齐）。
// meta 为 nil（详情加载失败）时跳过实验级字段，item 级字段仍尽力从 payload 填充。
func fillStandardEvalOutputMQMeta(res *expt.ItemStandardEvalOutput, item *entity.ItemResult, opt standardEvalOutputBuildOptions) {
	meta := opt.MQMeta
	if meta != nil {
		if meta.ExptWorkspaceID != 0 {
			res.ExptWorkspaceID = gptr.Of(meta.ExptWorkspaceID)
		}
		if meta.ExptRunID != 0 {
			res.ExptRunID = gptr.Of(meta.ExptRunID)
		}
		if meta.ExperimentGroupKey != "" {
			res.ExperimentGroupKey = gptr.Of(meta.ExperimentGroupKey)
		}
		if meta.EvalTargetID != 0 {
			res.EvalTargetID = gptr.Of(meta.EvalTargetID)
		}
		if meta.EvalTargetWorkspaceID != 0 {
			res.EvalTargetWorkspaceID = gptr.Of(meta.EvalTargetWorkspaceID)
		}
		if meta.DatasetWorkspaceID != 0 {
			res.DatasetWorkspaceID = gptr.Of(meta.DatasetWorkspaceID)
		}
		if meta.ExptCreateTime != 0 {
			res.ExperimentCreateTime = gptr.Of(meta.ExptCreateTime)
		}
	}

	// source_target_id: 优先用按 target_id 反查的结果（resolveSourceTargetIDs），回退实验级 target。
	if item != nil {
		if targetID := firstTargetIDFromItem(item, opt.ExptID); targetID != 0 {
			res.EvalTargetID = gptr.Of(targetID)
			if v, ok := opt.SourceTargetIDByTargetID[targetID]; ok && v != "" {
				res.SourceTargetID = gptr.Of(v)
			}
		}
	}
	if res.SourceTargetID == nil && meta != nil && meta.SourceTargetID != "" {
		res.SourceTargetID = gptr.Of(meta.SourceTargetID)
	}

	// expt_run_id: 若详情未给出（未跑），回退用 item payload 里的 EvalTargetRecord.ExperimentRunID。
	if res.ExptRunID == nil {
		if runID := firstExperimentRunIDFromItem(item, opt.ExptID); runID != 0 {
			res.ExptRunID = gptr.Of(runID)
		}
	}

	// dataset_id / dataset_version_id / dataset_version_name: 按 item 归属集分流。
	datasetID := datasetIDFromItem(item, opt.ExptID)
	if datasetID != 0 {
		res.DatasetID = gptr.Of(datasetID)
	}
	if meta != nil {
		es := meta.evalSetForItem(datasetID)
		if es != nil {
			if res.DatasetID == nil && es.ID != 0 {
				res.DatasetID = gptr.Of(es.ID)
			}
			if ver := es.EvaluationSetVersion; ver != nil {
				if ver.ID != 0 {
					res.DatasetVersionID = gptr.Of(ver.ID)
				}
				if ver.Version != "" {
					res.DatasetVersionName = gptr.Of(ver.Version)
				}
			}
		}
	}
}

// evalSetForItem 按 item 的 dataset_id 找归属集；命中不到时回退主集（单评测集/老实验）。
func (m *standardEvalOutputMQMeta) evalSetForItem(datasetID int64) *entity.EvaluationSet {
	if m == nil {
		return nil
	}
	if datasetID != 0 {
		if es, ok := m.EvalSetByID[datasetID]; ok {
			return es
		}
		// 多评测集里没命中归属集：不误用主集，返回 nil 避免张冠李戴。
		if len(m.EvalSetByID) > 1 {
			return nil
		}
	}
	if m.PrimaryEvalSetID != 0 {
		return m.EvalSetByID[m.PrimaryEvalSetID]
	}
	return nil
}

// datasetIDFromItem 从 item payload 的 EvalSet 取归属集 id。
func datasetIDFromItem(item *entity.ItemResult, exptID int64) int64 {
	for _, payload := range standardPayloads(item, exptID) {
		if payload != nil && payload.EvalSet != nil && payload.EvalSet.EvalSetID != 0 {
			return payload.EvalSet.EvalSetID
		}
	}
	return 0
}

// firstTargetIDFromItem 取 item 首个 payload 的 EvalTargetRecord.TargetID。
func firstTargetIDFromItem(item *entity.ItemResult, exptID int64) int64 {
	for _, payload := range standardPayloads(item, exptID) {
		if payload != nil && payload.TargetOutput != nil && payload.TargetOutput.EvalTargetRecord != nil {
			return payload.TargetOutput.EvalTargetRecord.TargetID
		}
	}
	return 0
}

// firstExperimentRunIDFromItem 取 item 首个 payload 的 EvalTargetRecord.ExperimentRunID。
func firstExperimentRunIDFromItem(item *entity.ItemResult, exptID int64) int64 {
	for _, payload := range standardPayloads(item, exptID) {
		if payload != nil && payload.TargetOutput != nil && payload.TargetOutput.EvalTargetRecord != nil {
			return payload.TargetOutput.EvalTargetRecord.ExperimentRunID
		}
	}
	return 0
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
	if item == nil {
		return ""
	}
	if item.Ext != nil && item.Ext["dataset_key"] != "" {
		return item.Ext["dataset_key"]
	}
	for _, payload := range standardPayloads(item, 0) {
		if payload == nil || payload.EvalSet == nil || payload.EvalSet.DatasetKey == "" {
			continue
		}
		return payload.EvalSet.DatasetKey
	}
	return ""
}

func buildStandardEvalOutputJSON(item *entity.ItemResult, opt standardEvalOutputBuildOptions) standardEvalOutputJSON {
	if std, ok := parseReportedStandardEvalOutput(item, opt); ok {
		return std
	}
	return standardEvalOutputJSON{
		Source: map[string]any{"type": "evaluation", "expt_id": opt.ExptID, "item_id": item.ItemID, "dataset_key": datasetKeyFromItem(item), "item_key": itemKeyFromItem(item)},
		Detail: map[string]any{"item_id": item.ItemID, "item_key": itemKeyFromItem(item), "item_index": item.ItemIndex, "system_info": item.SystemInfo, "turn_count": len(standardTurns(item, opt.ExptID))},
		Rounds: standardTurns(item, opt.ExptID),
		Agent:  standardAgent(item, opt.ExptID, opt),
		Output: standardOutput(item, opt.ExptID),
		Eval:   standardEval(item, opt.ExptID, opt),
		Extra:  standardExtra(item),
	}
}

func itemKeyFromItem(item *entity.ItemResult) string {
	if item == nil {
		return ""
	}
	if item.Ext != nil && item.Ext["item_key"] != "" {
		return item.Ext["item_key"]
	}
	for _, payload := range standardPayloads(item, 0) {
		if payload == nil || payload.EvalSet == nil || payload.EvalSet.ItemKey == "" {
			continue
		}
		return payload.EvalSet.ItemKey
	}
	return ""
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

func standardAgent(item *entity.ItemResult, exptID int64, opt standardEvalOutputBuildOptions) map[string]any {
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
		// source_target_id 为业务侧原始对象 ID（如 promptID / sandbox agent 外部标识），
		// 需按 target_id 反查 EvalTarget 得到；未解析到时留空。
		"source_target_id": opt.SourceTargetIDByTargetID[firstTargetID(first)],
		"runtime_param":    runtimeParam,
		"runs":             runs,
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
	if item == nil {
		return payloads
	}
	for _, turnResult := range item.TurnResults {
		if turnResult == nil {
			continue
		}
		for _, er := range turnResult.ExperimentResults {
			if er == nil || (exptID != 0 && er.ExperimentID != exptID) || er.Payload == nil {
				continue
			}
			payloads = append(payloads, er.Payload)
		}
	}
	return payloads
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
