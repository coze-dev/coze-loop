package entity

type EvaluatorTemplate struct {
	ID            int64
	SpaceID       int64
	Name          string
	Description   string
	EvaluatorType EvaluatorType

	Benchmark string
	Vendor    string
	Hot       int64

	InputSchemas       []*ArgsSchema                `json:"input_schemas"`
	OutputSchemas      []*ArgsSchema                `json:"output_schemas"`
	ReceiveChatHistory *bool                        `json:"receive_chat_history"`
	Tags               map[EvaluatorTagKey][]string `json:"tags"`

	PromptEvaluatorContent *PromptEvaluatorContent
	CodeEvaluatorContent   *CodeEvaluatorContent

	BaseInfo *BaseInfo `json:"base_info"`
}

type PromptEvaluatorContent struct {
	MessageList  []*Message   `json:"message_list"`
	ModelConfig  *ModelConfig `json:"model_config"`
	Tools        []*Tool      `json:"tools"`
	ParseType    ParseType    `json:"parse_type"`
	PromptSuffix string       `json:"prompt_suffix"`
}

type CodeEvaluatorContent struct {
	CodeContent  string       `json:"code_content"`
	LanguageType LanguageType `json:"language_type"`
}

func (do *EvaluatorTemplate) SetBaseInfo(baseInfo *BaseInfo) {
	do.BaseInfo = baseInfo
}

func (do *EvaluatorTemplate) GetBaseInfo() *BaseInfo {
	return do.BaseInfo
}
