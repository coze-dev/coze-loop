package loop_span

import (
	"strconv"

	"github.com/coze-dev/coze-loop/backend/pkg/json"
	time_util "github.com/coze-dev/coze-loop/backend/pkg/time"
)

const (
	StepTypeAgent = "agent"
	StepTypeModel = "model"
	StepTypeTool  = "tool"
)

type StepType = string

type TrajectoryList []*Trajectory

type Trajectory struct {
	// trace_id
	ID *string `json:"id"`
	// 根节点，记录整个轨迹的信息
	RootStep *RootStep `json:"root_step"`
	// agent step列表，记录轨迹中agent执行信息
	AgentSteps []*AgentStep `json:"agent_steps"`
}

type RootStep struct {
	// 唯一ID，trace导入时取span_id
	ID *string `json:"id"`
	// name，trace导入时取span_name
	Name *string `json:"name"`
	// 输入
	Input *string `json:"input"`
	// 输出
	Output *string `json:"output"`
	// 系统属性
	Metadata  map[string]string `json:"metadata"`
	BasicInfo *BasicInfo        `json:"basic_info"`
}

type AgentStep struct {
	// 基础属性
	ID *string `json:"id"`
	// 父ID， trace导入时取parent_span_id
	ParentID *string `json:"parent_id"`
	// name，trace导入时取span_name
	Name *string `json:"name"`
	// 输入
	Input *string `json:"input"`
	// 输出
	Output *string `json:"output"`
	// 子节点，agent执行内部经历了哪些步骤
	Steps []*Step `json:"steps"`
	// 系统属性
	Metadata  map[string]string `json:"metadata"`
	BasicInfo *BasicInfo        `json:"basic_info"`
}

type Step struct {
	// 基础属性
	ID *string `json:"id"`
	// 父ID， trace导入时取parent_span_id
	ParentID *string `json:"parent_id"`
	// 类型
	Type *StepType `json:"type"`
	// name，trace导入时取span_name
	Name *string `json:"name"`
	// 输入
	Input *string `json:"input"`
	// 输出
	Output *string `json:"output"`
	// 各种类型补充信息
	ModelInfo *ModelInfo `json:"model_info"`
	// 系统属性
	Metadata  map[string]string `json:"metadata"`
	BasicInfo *BasicInfo        `json:"basic_info"`
}

type ModelInfo struct {
	InputTokens               int64  `json:"input_tokens"`
	OutputTokens              int64  `json:"output_tokens"`
	LatencyFirstResp          string `json:"latency_first_resp"` // 单位毫秒
	ReasoningTokens           int64  `json:"reasoning_tokens"`
	InputReadCachedTokens     int64  `json:"input_read_cached_tokens"`
	InputCreationCachedTokens int64  `json:"input_creation_cached_tokens"`
}

type BasicInfo struct {
	// 单位毫秒
	StartedAt string `json:"started_at"`
	// 单位毫秒
	Duration string `json:"duration"`
	Error    *Error `json:"error"`
}

type Error struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
}

func BuildTrajectoryFromSpans(spanList SpanList) *Trajectory {
	if len(spanList) == 0 {
		return nil
	}

	// 构建span映射，便于查找
	spanMap := make(map[string]*Span)
	for _, span := range spanList {
		spanMap[span.SpanID] = span
	}

	var trajectoryID *string

	// 找到root节点
	var rootSpan *Span
	for _, span := range spanList {
		if span.ParentID == "" || span.ParentID == "0" {
			rootSpan = span
			trajectoryID = &span.SpanID
			break
		}
	}

	// 构建根节点步骤
	var rootStep *RootStep
	var rootSpanID string
	if rootSpan != nil {
		rootStep = &RootStep{
			ID:        &rootSpan.SpanID,
			Name:      &rootSpan.SpanName,
			Input:     &rootSpan.Input,
			Output:    &rootSpan.Output,
			BasicInfo: buildBasicInfo(rootSpan),
		}
		rootSpanID = rootSpan.SpanID
	}

	// 收集所有agent节点（包括root节点）
	agentSpans := make([]*Span, 0)
	if rootSpan != nil {
		agentSpans = append(agentSpans, rootSpan)
	}
	for _, span := range spanList {
		if span.SpanType == "agent" && span.SpanID != rootSpanID {
			agentSpans = append(agentSpans, span)
		}
	}

	// 构建agent步骤
	agentSteps := make([]*AgentStep, 0, len(agentSpans))
	for _, agentSpan := range agentSpans {
		if agentSpan == nil {
			continue
		}
		if trajectoryID == nil {
			trajectoryID = &agentSpan.SpanID
		}
		agentStep := &AgentStep{
			ID:        &agentSpan.SpanID,
			ParentID:  &agentSpan.ParentID,
			Name:      &agentSpan.SpanName,
			Input:     &agentSpan.Input,
			Output:    &agentSpan.Output,
			BasicInfo: buildBasicInfo(agentSpan),
			Steps:     buildAgentSteps(agentSpan, spanMap),
		}
		agentSteps = append(agentSteps, agentStep)
	}

	trajectory := &Trajectory{
		ID:         trajectoryID,
		RootStep:   rootStep,
		AgentSteps: agentSteps,
	}

	return trajectory
}

// buildBasicInfo 构建基础信息
func buildBasicInfo(span *Span) *BasicInfo {
	if span == nil {
		return nil
	}
	startedAt := time_util.MicroSec2MillSec(span.StartTime)     // ms
	duration := time_util.MicroSec2MillSec(span.DurationMicros) // ms

	// 构建错误信息
	var errorInfo *Error
	if span.StatusCode != 0 {
		errorMsg := ""
		if errMsg, ok := span.TagsString["error"]; ok {
			errorMsg = errMsg
		}
		errorInfo = &Error{
			Code: span.StatusCode,
			Msg:  errorMsg,
		}
	}

	return &BasicInfo{
		StartedAt: strconv.FormatInt(startedAt, 10),
		Duration:  strconv.FormatInt(duration, 10),
		Error:     errorInfo,
	}
}

// buildAgentSteps 构建agent的子步骤
func buildAgentSteps(agentSpan *Span, spanMap map[string]*Span) []*Step {
	if agentSpan == nil {
		return nil
	}
	steps := make([]*Step, 0)

	// 获取agent的直接子节点
	childSpans := getDirectChildren(agentSpan, spanMap)

	for _, childSpan := range childSpans {
		// 深度遍历每个分支收集所有普通子节点，每个分支直到遇到agent节点为止
		branchSteps := collectOtherSteps(childSpan, spanMap)
		steps = append(steps, branchSteps...)

		// 对每个直接子节点，向下深度遍历找到每个分支的第一个agent/model/tool节点
		agentModelToolSteps := findAgentModelToolNode(childSpan, spanMap)
		if len(agentModelToolSteps) > 0 {
			steps = append(steps, agentModelToolSteps...)
		}
	}

	return steps
}

// getDirectChildren 获取直接子节点
func getDirectChildren(parentSpan *Span, spanMap map[string]*Span) []*Span {
	if parentSpan == nil {
		return nil
	}
	children := make([]*Span, 0)

	for _, span := range spanMap {
		if span.ParentID == parentSpan.SpanID {
			children = append(children, span)
		}
	}

	// 按开始时间排序
	for i := 0; i < len(children); i++ {
		for j := i + 1; j < len(children); j++ {
			if children[i].StartTime > children[j].StartTime {
				children[i], children[j] = children[j], children[i]
			}
		}
	}

	return children
}

// buildStep 构建步骤
func buildStep(span *Span) *Step {
	if span == nil {
		return nil
	}
	stepType := getStepType(span)

	step := &Step{
		ID:        &span.SpanID,
		ParentID:  &span.ParentID,
		Type:      &stepType,
		Name:      &span.SpanName,
		Input:     &span.Input,
		Output:    &span.Output,
		BasicInfo: buildBasicInfo(span),
	}

	// 如果是model类型，添加model信息
	if stepType == StepTypeModel {
		step.ModelInfo = buildModelInfo(span)
	}

	return step
}

// findAgentModelToolNode 向下深度遍历，找到每个分支的第一个agent/model/tool节点
func findAgentModelToolNode(startSpan *Span, spanMap map[string]*Span) []*Step {
	if startSpan == nil {
		return nil
	}

	steps := make([]*Step, 0)
	stepType := getStepType(startSpan)

	// 如果当前节点就是agent/model/tool，直接返回
	if stepType == StepTypeAgent || stepType == StepTypeModel || stepType == StepTypeTool {
		steps = append(steps, buildStep(startSpan))
		return steps
	}

	// 如果是other节点，继续向下遍历
	children := getDirectChildren(startSpan, spanMap)
	for _, child := range children {
		if result := findAgentModelToolNode(child, spanMap); len(result) > 0 {
			steps = append(steps, result...)
		}
	}

	return steps
}

// collectOtherSteps 深度遍历分支，收集任意层级的普通子节点，直到遇到agent节点为止
func collectOtherSteps(startSpan *Span, spanMap map[string]*Span) []*Step {
	if startSpan == nil {
		return nil
	}

	steps := make([]*Step, 0)
	stepType := getStepType(startSpan)

	// 如果当前节点是agent节点，停止遍历
	if stepType == StepTypeAgent {
		return steps
	}

	// 如果是普通节点，添加到结果中，然后继续向下遍历
	if stepType != StepTypeModel && stepType != StepTypeTool {
		steps = append(steps, buildStep(startSpan))
	}

	// 获取当前节点的子节点，继续深度遍历
	children := getDirectChildren(startSpan, spanMap)
	for _, child := range children {
		childSteps := collectOtherSteps(child, spanMap)
		if len(childSteps) > 0 {
			steps = append(steps, childSteps...)
		}
	}

	return steps
}

// getStepType 获取步骤类型
func getStepType(span *Span) StepType {
	if span == nil {
		return ""
	}
	switch span.SpanType {
	case "agent":
		return StepTypeAgent
	case "model":
		return StepTypeModel
	case "tool":
		return StepTypeTool
	default:
		if span.ParentID == "" || span.ParentID == "0" {
			return StepTypeTool
		}
		return span.SpanType // 默认返回SpanType，既不是root，也不是agent/model/tool
	}
}

// buildModelInfo 构建模型信息
func buildModelInfo(span *Span) *ModelInfo {
	if span == nil {
		return nil
	}
	modelInfo := &ModelInfo{}

	// 从tags中提取模型相关信息
	if inputTokens, ok := span.TagsLong["input_tokens"]; ok {
		modelInfo.InputTokens = inputTokens
	}
	if outputTokens, ok := span.TagsLong["output_tokens"]; ok {
		modelInfo.OutputTokens = outputTokens
	}
	if latencyFirstResp, ok := span.TagsLong["latency_first_resp"]; ok {
		modelInfo.LatencyFirstResp = strconv.FormatInt(time_util.MicroSec2MillSec(latencyFirstResp), 10)
	}
	if reasoningTokens, ok := span.TagsLong["reasoning_tokens"]; ok {
		modelInfo.ReasoningTokens = reasoningTokens
	}
	if inputReadCachedTokens, ok := span.TagsLong["input_cached_tokens"]; ok {
		modelInfo.InputReadCachedTokens = inputReadCachedTokens
	}
	if inputCreationCachedTokens, ok := span.TagsLong["input_creation_cached_tokens"]; ok {
		modelInfo.InputCreationCachedTokens = inputCreationCachedTokens
	}

	return modelInfo
}

func (t *Trajectory) MarshalString() (string, error) {
	return json.MarshalString(t)
}
