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

// standardEvalOutputFornaxPrefix 评测对象自报 standard eval output 字段时使用的前缀，
// 用于与用户自定义的普通 ext_output 字段区分、避免重名（见《PSM类评测对象Schema对齐》）。
const standardEvalOutputFornaxPrefix = "FORNAX_"

// standardEvalOutputMergeableFields 平台组装 standard eval output 时，支持
// “评测对象上报优先 + 平台兜底 + 子字段深合并” 的字段集合。
// 不含 source：source 恒由平台生成，忽略任何 FORNAX_source 上报。
var standardEvalOutputMergeableFields = []string{"detail", "rounds", "agent", "output", "eval", "extra"}

// lookupFornaxField 在评测对象上报的 output_fields 中查找某标准字段对应的 Content：
// 先查带 FORNAX_ 前缀的 key（新协议），未命中再 fallback 到裸 key（向前兼容存量上报）。
// 两者都无（或值为 nil）时返回 false，表示对象未上报该字段、由平台兜底。
func lookupFornaxField(fields map[string]*entity.Content, field string) (*entity.Content, bool) {
	if len(fields) == 0 {
		return nil, false
	}
	if c, ok := fields[standardEvalOutputFornaxPrefix+field]; ok && c != nil {
		return c, true
	}
	if c, ok := fields[field]; ok && c != nil {
		return c, true
	}
	return nil, false
}

// standardFieldContentMergeable 判断评测对象上报的字段 Content 能否参与结构化子字段深合并。
// 两种情况无法结构化、只能原样透出（对象优先）：
//   - 内容被省略的大对象（ContentOmitted 或 FullContent 非空）：Text 仅为裁剪后的预览片段，
//     全量在对象存储（full_content.uri），平台侧没有完整数据可合并；
//   - Text 为空或非合法 JSON：无法反序列化成结构，没有“子字段”可递归。
func standardFieldContentMergeable(c *entity.Content) bool {
	if c == nil {
		return false
	}
	if gptr.Indirect(c.ContentOmitted) || c.FullContent != nil {
		return false
	}
	text := c.GetText()
	if text == "" || !json.Valid([]byte(text)) {
		return false
	}
	return true
}

// normalizeToJSONValue 把任意 Go 值经 JSON round-trip 归一为 map[string]any / []any / 标量，
// 使平台兜底值与评测对象上报的已解析 JSON 值在同一套原生类型上做深合并。失败时返回原值。
func normalizeToJSONValue(v any) any {
	if v == nil {
		return nil
	}
	text, err := json.MarshalString(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return v
	}
	return out
}

// deepMergeStandardEvalOutput 深度合并平台兜底值与评测对象上报值，冲突以对象为准：
//   - 两侧均为 JSON object(map) 时逐 key 递归合并（只在一侧的 key 保留该侧）；
//   - 其余情况（类型冲突 / 标量 / 数组 / null）由对象整体覆盖平台。
func deepMergeStandardEvalOutput(platform, object any) any {
	pm, pOK := platform.(map[string]any)
	om, oOK := object.(map[string]any)
	if pOK && oOK {
		merged := make(map[string]any, len(pm)+len(om))
		for k, v := range pm {
			merged[k] = v
		}
		for k, ov := range om {
			if pv, exists := merged[k]; exists {
				merged[k] = deepMergeStandardEvalOutput(pv, ov)
			} else {
				merged[k] = ov
			}
		}
		return merged
	}
	return object
}

// putStandardField 仅在值非空（空串跳过）时写入 map，避免输出一堆空占位 key。
func putStandardField(m map[string]any, key, val string) {
	if val != "" {
		m[key] = val
	}
}

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
				if target == nil {
					logs.CtxWarn(ctx, "resolveSourceTargetIDs target nil, target_id=%d", targetID)
					out[targetID] = ""
					continue
				}
				// 跨空间共享: 评测对象归来源空间 B, target.SpaceID != 调用方 spaceID(A) 属正常;
				// targetID 来自本实验自身结果记录(发起时已 AuthorizeRead 授权), 回填其恒定 SourceTargetID
				// (非内容, 仅标识) 不构成越权。仅同空间(B==A)时保留原语义, 跨空间放行。
				if target.SpaceID != spaceID {
					logs.CtxInfo(ctx, "resolveSourceTargetIDs cross-space target, target_id=%d, target_space=%d, caller_space=%d", targetID, target.SpaceID, spaceID)
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
	meta.ExptCreatedBy = expt.CreatedBy
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
	// ExptCreatedBy: 实验创建人 userID，来源 experiment.created_by（实验级恒定）。
	ExptCreatedBy string
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
	// 平台兜底基线：buildStandardEvalOutputJSON 已内含旧的整包 / 裸 key 复用路径。
	std := buildStandardEvalOutputJSON(item, opt)
	// 评测对象自报的 ext_output（FORNAX_ 前缀优先，裸 key 兜底），逐字段叠加到基线上。
	// source 恒由平台生成、不被对象覆盖，故不在可合并字段内、也保持现状不 emit。
	fields := reportedStandardOutputFields(item, opt.ExptID)

	var err error
	if res.Detail, err = mergeStandardEvalOutputField(std.Detail, fields, "detail"); err != nil {
		return nil, err
	}
	if res.Rounds, err = mergeStandardEvalOutputField(std.Rounds, fields, "rounds"); err != nil {
		return nil, err
	}
	if res.Agent, err = mergeStandardEvalOutputField(std.Agent, fields, "agent"); err != nil {
		return nil, err
	}
	if res.Output, err = mergeStandardEvalOutputField(std.Output, fields, "output"); err != nil {
		return nil, err
	}
	if res.Eval, err = mergeStandardEvalOutputField(std.Eval, fields, "eval"); err != nil {
		return nil, err
	}
	if res.Extra, err = mergeStandardEvalOutputField(std.Extra, fields, "extra"); err != nil {
		return nil, err
	}
	return res, nil
}

// reportedStandardOutputFields 取评测对象自报的 output_fields：在该 item 的各 turn payload 中，
// 返回第一个含有任一标准字段（FORNAX_ 前缀或裸 key）的 OutputFields；均无则返回 nil。
func reportedStandardOutputFields(item *entity.ItemResult, exptID int64) map[string]*entity.Content {
	for _, payload := range standardPayloads(item, exptID) {
		if payload == nil || payload.TargetOutput == nil || payload.TargetOutput.EvalTargetRecord == nil || payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData == nil {
			continue
		}
		fields := payload.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields
		for _, f := range standardEvalOutputMergeableFields {
			if _, ok := lookupFornaxField(fields, f); ok {
				return fields
			}
		}
	}
	return nil
}

// mergeStandardEvalOutputField 逐字段实现「对象优先 + 平台兜底」：
//   - 对象未报该字段 → 平台兜底值；
//   - 对象报了但内容无法结构化（省略大对象 / 非 JSON）→ 对象 Content 原样透出（对象优先，保留省略语义）；
//   - rounds 字段：对象报了 → 整体用对象的，平台不再补（每一轮由对象自报）；
//   - 其余字段：对象报了合法 JSON → 与平台兜底子字段级深合并，冲突以对象为准。
func mergeStandardEvalOutputField(platformVal any, fields map[string]*entity.Content, field string) (*expt.StandardEvalOutputContent, error) {
	c, ok := lookupFornaxField(fields, field)
	if !ok {
		return inlineJSONContent(platformVal)
	}
	if !standardFieldContentMergeable(c) {
		return contentToStandardEvalOutputContent(c), nil
	}
	var objVal any
	if err := json.Unmarshal([]byte(c.GetText()), &objVal); err != nil {
		return contentToStandardEvalOutputContent(c), nil
	}
	// rounds：对象上报即整体采用对象的，平台不合并、不补轮次。
	if field == "rounds" {
		return inlineJSONContent(objVal)
	}
	merged := deepMergeStandardEvalOutput(normalizeToJSONValue(platformVal), objVal)
	return inlineJSONContent(merged)
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
			// ItemEndTime 来源 expt_item_result.updated_at，表示当前/latest run 终态同步到主结果表的时间，
			// 不代表精确的执行结束时刻；非终态不输出。
			if entity.IsItemRunFinished(item.SystemInfo.RunState) && item.SystemInfo.EndTime != nil {
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
		if meta.ExptCreatedBy != "" {
			res.CreatedBy = gptr.Of(meta.ExptCreatedBy)
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
		Source: map[string]any{"type": "evaluation", "expt_id": int64String(opt.ExptID), "item_id": int64String(item.ItemID), "dataset_key": datasetKeyFromItem(item), "item_key": itemKeyFromItem(item)},
		Detail: map[string]any{"item_id": int64String(item.ItemID), "item_key": itemKeyFromItem(item), "item_index": item.ItemIndex, "system_info": item.SystemInfo, "turn_count": len(standardTurns(item, opt.ExptID))},
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
		// 平台补的轮次只填拿得到的字段，拿不到的（start/end_time、无 trace 等）不硬塞 0/空串占位。
		round := map[string]any{
			"round_id": standardRoundID(payload),
			"round_no": i + 1,
		}
		if q := userQueryFromPayload(payload); q != "" {
			round["user_query"] = q
		}
		if l := latencyFromPayload(payload); l != 0 {
			round["latency"] = l
		}
		if tokens := tokensFromPayload(payload); len(tokens) > 0 {
			round["tokens"] = tokens
		}
		if c := contextFromPayload(payload); len(c) > 0 {
			round["context"] = c
		}
		rounds = append(rounds, round)
	}
	return rounds
}

func standardAgent(item *entity.ItemResult, exptID int64, opt standardEvalOutputBuildOptions) map[string]any {
	var first *entity.EvalTargetRecord
	for _, payload := range standardPayloads(item, exptID) {
		tr := payload.TargetOutput
		if tr == nil || tr.EvalTargetRecord == nil {
			continue
		}
		if first == nil {
			first = tr.EvalTargetRecord
			break
		}
	}
	runtimeParam := runtimeParamObjectFromTargetRecord(first)
	// agent 只保留评测对象元信息（对齐文档 FORNAX_agent）+ runtime_param；
	// 不回填 runs / target_id / target_version_id / source_target_id（顶层 ItemStandardEvalOutput
	// 已有 eval_target_id / source_target_id 等 MQ meta 字段，不在此重复）。
	// 空值不填 key（D11）：无值不放进 map，避免空占位。
	agent := map[string]any{}
	if runtimeParam != nil {
		agent["runtime_param"] = runtimeParam
	}
	putStandardField(agent, "agent_id", int64String(firstTargetID(first)))
	putStandardField(agent, "model_name", stringFromRuntimeParam(runtimeParam, "model_name", "model", "model_id"))
	putStandardField(agent, "agent_name", stringFromRuntimeParam(runtimeParam, "agent_name", "agent", "name"))
	putStandardField(agent, "agent_version", stringFromRuntimeParam(runtimeParam, "agent_version", "version"))
	putStandardField(agent, "thinking_effort", stringFromRuntimeParam(runtimeParam, "thinking_effort", "effort"))
	putStandardField(agent, "context_window", stringFromRuntimeParam(runtimeParam, "context_window", "context_window_size", "main_context_window_size"))
	return agent
}

func standardOutput(item *entity.ItemResult, exptID int64) map[string]any {
	payloads := standardPayloads(item, exptID)
	var detailOutput map[string]*entity.Content
	if len(payloads) > 0 {
		last := payloads[len(payloads)-1]
		if last.TargetOutput != nil && last.TargetOutput.EvalTargetRecord != nil && last.TargetOutput.EvalTargetRecord.EvalTargetOutputData != nil {
			detailOutput = last.TargetOutput.EvalTargetRecord.EvalTargetOutputData.OutputFields
		}
	}
	// 平台兜底只补 detail.output；file_diff 平台侧无数据、为空不回填（对象要则自报 FORNAX_output）。
	return map[string]any{"detail": map[string]any{"output": detailOutput}}
}

func standardEval(item *entity.ItemResult, exptID int64, opt standardEvalOutputBuildOptions) map[string]any {
	payloads := standardPayloads(item, exptID)
	var detailEval map[string]any
	for _, payload := range payloads {
		detailEval = standardEvalResult(payload, opt)
	}
	if detailEval == nil {
		detailEval = map[string]any{"type": "score", "score": nil, "reason": "", "results": map[string]any{}}
	}
	// 平台兜底只补 detail，不补 round 粒度的 rounds（对象要 round 粒度自行上报 FORNAX_eval.rounds）。
	return map[string]any{"task_config": standardEvalTaskConfig(item), "detail": map[string]any{"run_status": itemRunStatus(item), "eval_result": detailEval}}
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

// standardRoundID 平台兜底轮次的 round_id：直接用 TurnID（对象未上报 rounds 时平台补的每轮标识）。
func standardRoundID(payload *entity.ExperimentTurnPayload) string {
	if payload == nil {
		return ""
	}
	return strconv.FormatInt(payload.TurnID, 10)
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
	// 只填有值的 token 统计，无值不塞 0。
	tokens := map[string]any{}
	if v := usage.GetInputTokens(); v != 0 {
		tokens["prompt_tokens"] = v
	}
	if v := usage.GetOutputTokens(); v != 0 {
		tokens["completion_tokens"] = v
	}
	if v := usage.GetTotalTokens(); v != 0 {
		tokens["total_tokens"] = v
	}
	return tokens
}

func contextFromPayload(payload *entity.ExperimentTurnPayload) map[string]any {
	// 只填拿得到的 trace 关联字段，拿不到的不硬塞空串/0（start_time/end_time 平台侧无数据源，不填）。
	ctx := map[string]any{}
	if payload == nil {
		return ctx
	}
	logID := ""
	if payload.SystemInfo != nil && payload.SystemInfo.LogID != nil {
		logID = *payload.SystemInfo.LogID
	}
	if payload.TargetOutput != nil && payload.TargetOutput.EvalTargetRecord != nil {
		rec := payload.TargetOutput.EvalTargetRecord
		if rec.LogID != "" {
			logID = rec.LogID
		}
		if rec.TraceID != "" {
			ctx["trace_id"] = rec.TraceID
		}
	}
	if logID != "" {
		ctx["log_id"] = logID
	}
	return ctx
}

func firstTargetID(record *entity.EvalTargetRecord) int64 {
	if record == nil {
		return 0
	}
	return record.TargetID
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
			resultKey := evaluatorResultKey(opt, key, record)
			if score == nil && record.GetScore() != nil {
				score = *record.GetScore()
			}
			if reason == "" {
				reason = record.GetReasoning()
			}
			// key 即 evaluator_version_id；evaluator_id 从 ColumnEvaluator 反查。
			// 二者均为 i64 雪花，inline JSON 须 string 化防精度丢失。
			// 空值不填 key（D11）：name/version/alias/id/version_id 无值时不放进 map。
			entry := map[string]any{
				"type":   "score",
				"score":  record.GetScore(),
				"reason": record.GetReasoning(),
			}
			putStandardField(entry, "evaluator_version_id", int64String(key))
			putStandardField(entry, "evaluator_name", evaluatorName(opt, key, record))
			putStandardField(entry, "evaluator_version", evaluatorVersion(opt, key, record))
			putStandardField(entry, "evaluator_alias", record.Alias)
			if eid := evaluatorID(opt, key); eid != 0 {
				entry["evaluator_id"] = int64String(eid)
			}
			results[resultKey] = entry
		}
	}
	return map[string]any{"type": "score", "score": score, "reason": reason, "results": results}
}

// evaluatorResultKey 生成 results map 的唯一 key = 评估器名 + 版本号 + 别名。
//   - name / version 从 ColumnEvaluator（按 evaluator_version_id 反查）取；
//   - alias 区分同评估器版本的多实例（judge_A/judge_B）；
//
// 三者组合在单 item results 内不撞（同评估器多别名靠 alias 区分、不同评估器名不同）。
// name 反查不到（老数据 / inline）时退化用 record 自身 version_id / inline_key 兜底。
func evaluatorResultKey(opt standardEvalOutputBuildOptions, key int64, record *entity.EvaluatorRecord) string {
	name := ""
	version := ""
	if meta := opt.EvaluatorByVersionID[key]; meta != nil {
		name = gptr.Indirect(meta.Name)
		version = gptr.Indirect(meta.Version)
	}
	alias := ""
	inlineKey := ""
	if record != nil {
		alias = record.Alias
		inlineKey = record.InlineKey
	}
	if name != "" {
		return name + ":" + version + ":" + alias
	}
	// 兜底（inline / 反查不到 ColumnEvaluator）：versionID + alias(+inlineKey)，避免撞 key
	rk := entity.EncodeEvaluatorInstanceKey(key, alias)
	if inlineKey != "" {
		rk += "#" + inlineKey
	}
	return rk
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

// evaluatorID 从 ColumnEvaluator（按 evaluator_version_id 索引）反查 evaluator_id；未命中返回 0。
func evaluatorID(opt standardEvalOutputBuildOptions, key int64) int64 {
	if meta := opt.EvaluatorByVersionID[key]; meta != nil {
		return meta.EvaluatorID
	}
	return 0
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
