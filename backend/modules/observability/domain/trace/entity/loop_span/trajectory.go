package loop_span

const (
	StepTypeAgent = "agent"
	StepTypeModel = "model"
	StepTypeTool  = "tool"
)

type StepType = string

type TrajectoryList []*Trajectory

type Trajectory struct {
	// trace_id
	ID *int64
	// 根节点，记录整个轨迹的信息
	RootStep *RootStep
	// agent step列表，记录轨迹中agent执行信息
	AgentSteps []*AgentStep
}

type RootStep struct {
	// 唯一ID，trace导入时取span_id
	ID *string
	// name，trace导入时取span_name
	Name *string
	// 输入
	Input *string
	// 输出
	Output *string
	// 系统属性
	Metadata  map[string]string
	BasicInfo *BasicInfo
}

type AgentStep struct {
	// 基础属性
	ID *string
	// 父ID， trace导入时取parent_span_id
	ParentID *string
	// name，trace导入时取span_name
	Name *string
	// 输入
	Input *string
	// 输出
	Output *string
	// 子节点，agent执行内部经历了哪些步骤
	Steps []*Step
	// 系统属性
	Metadata  map[string]string
	BasicInfo *BasicInfo
}

type Step struct {
	// 基础属性
	ID *string
	// 父ID， trace导入时取parent_span_id
	ParentID *string
	// 类型
	Type *StepType
	// name，trace导入时取span_name
	Name *string
	// 输入
	Input *string
	// 输出
	Output *string
	// 各种类型补充信息
	ModelInfo *ModelInfo
	// 系统属性
	Metadata  map[string]string
	BasicInfo *BasicInfo
}

type ModelInfo struct {
	InputTokens  *int64
	OutputTokens *int64
}

type BasicInfo struct {
	// 单位微秒
	StartedAt *int64
	// 单位微秒
	Duration *int64
	Error    *Error
}

type Error struct {
	Code *int32
	Msg  *string
}
