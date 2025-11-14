package entity

type EvaluatorTagLangType string

const (
	EvaluatorTagLangType_Zh EvaluatorTagLangType = "zh-CN" // 中文
	EvaluatorTagLangType_En EvaluatorTagLangType = "en-US" // 英文
)

// EvaluatorTagKey Evaluator筛选字段
type EvaluatorTagKey string

const (
	EvaluatorTagKey_Category         EvaluatorTagKey = "Category"         // 类型筛选 (LLM/Code)
	EvaluatorTagKey_TargetType       EvaluatorTagKey = "TargetType"       // 评估对象 (文本/图片/视频等)
	EvaluatorTagKey_Objective        EvaluatorTagKey = "Objective"        // 评估目标 (任务完成/内容质量等)
	EvaluatorTagKey_BusinessScenario EvaluatorTagKey = "BusinessScenario" // 业务场景 (安全风控/AI Coding等)
	EvaluatorTagKey_BoxType          EvaluatorTagKey = "BoxType"          // 黑白盒类型
	EvaluatorTagKey_Name             EvaluatorTagKey = "Name"             // 评估器名称
)

type EvaluatorTagKeyType int32

const (
	EvaluatorTagKeyType_Evaluator         EvaluatorTagKeyType = 1 // 评估器标签
	EvaluatorTagKeyType_EvaluatorTemplate EvaluatorTagKeyType = 2 // 评估器模板标签
)

// EvaluatorFilterOption Evaluator筛选器选项
type EvaluatorFilterOption struct {
	SearchKeyword *string           `json:"search_keyword,omitempty"` // 模糊搜索关键词，在所有tag中搜索
	Filters       *EvaluatorFilters `json:"filters,omitempty"`        // 筛选条件
}

// EvaluatorFilters Evaluator筛选条件
type EvaluatorFilters struct {
	FilterConditions []*EvaluatorFilterCondition `json:"filter_conditions,omitempty"` // 筛选条件列表
	LogicOp          *FilterLogicOp              `json:"logic_op,omitempty"`          // 逻辑操作符
    SubFilters       []*EvaluatorFilters          `json:"sub_filters,omitempty"`       // 子条件组（支持嵌套）
}

// FilterLogicOp 筛选逻辑操作符
type FilterLogicOp int32

const (
	FilterLogicOp_Unknown FilterLogicOp = 0
	FilterLogicOp_And     FilterLogicOp = 1 // 与操作
	FilterLogicOp_Or      FilterLogicOp = 2 // 或操作
)

// EvaluatorFilterCondition Evaluator筛选条件
type EvaluatorFilterCondition struct {
	TagKey   EvaluatorTagKey             `json:"tag_key"`  // 筛选字段
	Operator EvaluatorFilterOperatorType `json:"operator"` // 操作符
	Value    string                      `json:"value"`    // 操作值
}

// EvaluatorFilterOperatorType Evaluator筛选操作符
type EvaluatorFilterOperatorType int32

const (
	EvaluatorFilterOperatorType_Unknown   EvaluatorFilterOperatorType = 0
	EvaluatorFilterOperatorType_Equal     EvaluatorFilterOperatorType = 1 // 等于
	EvaluatorFilterOperatorType_NotEqual  EvaluatorFilterOperatorType = 2 // 不等于
	EvaluatorFilterOperatorType_In        EvaluatorFilterOperatorType = 3 // 包含于
	EvaluatorFilterOperatorType_NotIn     EvaluatorFilterOperatorType = 4 // 不包含于
	EvaluatorFilterOperatorType_Like      EvaluatorFilterOperatorType = 5 // 模糊匹配
	EvaluatorFilterOperatorType_IsNull    EvaluatorFilterOperatorType = 6 // 为空
	EvaluatorFilterOperatorType_IsNotNull EvaluatorFilterOperatorType = 7 // 非空
)

// String 返回FilterLogicOp的字符串表示
func (f FilterLogicOp) String() string {
	switch f {
	case FilterLogicOp_And:
		return "AND"
	case FilterLogicOp_Or:
		return "OR"
	default:
		return "UNKNOWN"
	}
}

// String 返回EvaluatorFilterOperatorType的字符串表示
func (e EvaluatorFilterOperatorType) String() string {
	switch e {
	case EvaluatorFilterOperatorType_Equal:
		return "EQUAL"
	case EvaluatorFilterOperatorType_NotEqual:
		return "NOT_EQUAL"
	case EvaluatorFilterOperatorType_In:
		return "IN"
	case EvaluatorFilterOperatorType_NotIn:
		return "NOT_IN"
	case EvaluatorFilterOperatorType_Like:
		return "LIKE"
	case EvaluatorFilterOperatorType_IsNull:
		return "IS_NULL"
	case EvaluatorFilterOperatorType_IsNotNull:
		return "IS_NOT_NULL"
	default:
		return "UNKNOWN"
	}
}

// IsValid 检查FilterLogicOp是否有效
func (f FilterLogicOp) IsValid() bool {
	return f == FilterLogicOp_And || f == FilterLogicOp_Or
}

// IsValid 检查EvaluatorFilterOperatorType是否有效
func (e EvaluatorFilterOperatorType) IsValid() bool {
	return e >= EvaluatorFilterOperatorType_Equal && e <= EvaluatorFilterOperatorType_IsNotNull
}

// NewEvaluatorFilterOption 创建新的EvaluatorFilterOption
func NewEvaluatorFilterOption() *EvaluatorFilterOption {
	return &EvaluatorFilterOption{}
}

// WithSearchKeyword 设置搜索关键词
func (f *EvaluatorFilterOption) WithSearchKeyword(keyword string) *EvaluatorFilterOption {
	f.SearchKeyword = &keyword
	return f
}

// WithFilters 设置筛选条件
func (f *EvaluatorFilterOption) WithFilters(filters *EvaluatorFilters) *EvaluatorFilterOption {
	f.Filters = filters
	return f
}

// NewEvaluatorFilters 创建新的EvaluatorFilters
func NewEvaluatorFilters() *EvaluatorFilters {
	logicOp := FilterLogicOp_And
	return &EvaluatorFilters{
		LogicOp: &logicOp, // 默认为AND操作
	}
}

// WithLogicOp 设置逻辑操作符
func (f *EvaluatorFilters) WithLogicOp(logicOp FilterLogicOp) *EvaluatorFilters {
	f.LogicOp = &logicOp
	return f
}

// AddCondition 添加筛选条件
func (f *EvaluatorFilters) AddCondition(condition *EvaluatorFilterCondition) *EvaluatorFilters {
	if f.FilterConditions == nil {
		f.FilterConditions = make([]*EvaluatorFilterCondition, 0)
	}
	f.FilterConditions = append(f.FilterConditions, condition)
	return f
}

// NewEvaluatorFilterCondition 创建新的筛选条件
func NewEvaluatorFilterCondition(tagKey EvaluatorTagKey, operator EvaluatorFilterOperatorType, value string) *EvaluatorFilterCondition {
	return &EvaluatorFilterCondition{
		TagKey:   tagKey,
		Operator: operator,
		Value:    value,
	}
}
