// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func PromptDTO2DO(dto *prompt.Prompt) *entity.Prompt {
	if dto == nil {
		return nil
	}
	return &entity.Prompt{
		ID:           dto.GetID(),
		SpaceID:      dto.GetWorkspaceID(),
		PromptKey:    dto.GetPromptKey(),
		PromptBasic:  PromptBasicDTO2DO(dto.GetPromptBasic()),
		PromptDraft:  PromptDraftDTO2DO(dto.GetPromptDraft()),
		PromptCommit: PromptCommitDTO2DO(dto.GetPromptCommit()),
	}
}

func BatchPromptDTO2DO(dtos []*prompt.Prompt) []*entity.Prompt {
	if len(dtos) == 0 {
		return nil
	}
	prompts := make([]*entity.Prompt, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		prompts = append(prompts, PromptDTO2DO(dto))
	}
	if len(prompts) == 0 {
		return nil
	}
	return prompts
}

func PromptDraftDTO2DO(dto *prompt.PromptDraft) *entity.PromptDraft {
	if dto == nil {
		return nil
	}
	return &entity.PromptDraft{
		PromptDetail: PromptDetailDTO2DO(dto.GetDetail()),
		DraftInfo:    DraftInfoDTO2DO(dto.GetDraftInfo()),
	}
}

func DraftInfoDTO2DO(dto *prompt.DraftInfo) *entity.DraftInfo {
	if dto == nil {
		return nil
	}
	return &entity.DraftInfo{
		UserID:      dto.GetUserID(),
		BaseVersion: dto.GetBaseVersion(),
		IsModified:  dto.GetIsModified(),
		CreatedAt:   time.UnixMilli(dto.GetCreatedAt()),
		UpdatedAt:   time.UnixMilli(dto.GetUpdatedAt()),
	}
}

func PromptCommitDTO2DO(dto *prompt.PromptCommit) *entity.PromptCommit {
	if dto == nil {
		return nil
	}
	return &entity.PromptCommit{
		CommitInfo:   PromptCommitInfoDTO2DO(dto.GetCommitInfo()),
		PromptDetail: PromptDetailDTO2DO(dto.GetDetail()),
	}
}

func PromptCommitInfoDTO2DO(dto *prompt.CommitInfo) *entity.CommitInfo {
	if dto == nil {
		return nil
	}
	return &entity.CommitInfo{
		Version:     dto.GetVersion(),
		BaseVersion: dto.GetBaseVersion(),
		Description: dto.GetDescription(),
		CommittedBy: dto.GetCommittedBy(),
		CommittedAt: time.UnixMilli(dto.GetCommittedAt()),
	}
}

func PromptBasicDTO2DO(dto *prompt.PromptBasic) *entity.PromptBasic {
	if dto == nil {
		return nil
	}
	return &entity.PromptBasic{
		PromptType:    PromptTypeDTO2DO(dto.GetPromptType()),
		DisplayName:   dto.GetDisplayName(),
		Description:   dto.GetDescription(),
		LatestVersion: dto.GetLatestVersion(),
		CreatedBy:     dto.GetCreatedBy(),
		UpdatedBy:     dto.GetUpdatedBy(),
		CreatedAt:     time.UnixMilli(dto.GetCreatedAt()),
		UpdatedAt:     time.UnixMilli(dto.GetUpdatedAt()),
	}
}

func PromptDetailDTO2DO(dto *prompt.PromptDetail) *entity.PromptDetail {
	if dto == nil {
		return nil
	}

	return &entity.PromptDetail{
		PromptTemplate: PromptTemplateDTO2DO(dto.PromptTemplate),
		Tools:          BatchToolDTO2DO(dto.Tools),
		ToolCallConfig: ToolCallConfigDTO2DO(dto.ToolCallConfig),
		ModelConfig:    ModelConfigDTO2DO(dto.ModelConfig),
		ExtInfos:       dto.ExtInfos,
	}
}

func PromptTemplateDTO2DO(dto *prompt.PromptTemplate) *entity.PromptTemplate {
	if dto == nil {
		return nil
	}

	return &entity.PromptTemplate{
		TemplateType: TemplateTypeDTO2DO(dto.GetTemplateType()),
		Messages:     BatchMessageDTO2DO(dto.Messages),
		VariableDefs: BatchVariableDefDTO2DO(dto.VariableDefs),
		HasSnippets:  dto.GetHasSnippet(),
		Snippets:     BatchPromptDTO2DO(dto.Snippets),
		Metadata:     dto.Metadata,
	}
}

func TemplateTypeDTO2DO(dto prompt.TemplateType) entity.TemplateType {
	switch dto {
	case prompt.TemplateTypeNormal:
		return entity.TemplateTypeNormal
	case prompt.TemplateTypeJinja2:
		return entity.TemplateTypeJinja2
	case prompt.TemplateTypeGoTemplate:
		return entity.TemplateTypeGoTemplate
	case prompt.TemplateTypeCustomTemplateM:
		return entity.TemplateTYpeCustomTemplateM
	default:
		return entity.TemplateTypeNormal
	}
}

func PromptTypeDTO2DO(dto prompt.PromptType) entity.PromptType {
	switch dto {
	case prompt.PromptTypeNormal:
		return entity.PromptTypeNormal
	case prompt.PromptTypeSnippet:
		return entity.PromptTypeSnippet
	default:
		return entity.PromptTypeNormal
	}
}

func BatchMessageDTO2DO(dtos []*prompt.Message) []*entity.Message {
	if dtos == nil {
		return nil
	}
	messages := make([]*entity.Message, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		messages = append(messages, MessageDTO2DO(dto))
	}
	return messages
}

func MessageDTO2DO(dto *prompt.Message) *entity.Message {
	if dto == nil {
		return nil
	}

	return &entity.Message{
		Role:             RoleDTO2DO(dto.GetRole()),
		ReasoningContent: dto.ReasoningContent,
		Content:          dto.Content,
		Parts:            BatchContentPartDTO2DO(dto.Parts),
		ToolCallID:       dto.ToolCallID,
		ToolCalls:        BatchToolCallDTO2DO(dto.ToolCalls),
		Metadata:         dto.Metadata,
	}
}

func RoleDTO2DO(role prompt.Role) entity.Role {
	switch role {
	case prompt.RoleSystem:
		return entity.RoleSystem
	case prompt.RoleUser:
		return entity.RoleUser
	case prompt.RoleAssistant:
		return entity.RoleAssistant
	case prompt.RoleTool:
		return entity.RoleTool
	case prompt.RolePlaceholder:
		return entity.RolePlaceholder
	default:
		return entity.RoleUser
	}
}

func BatchContentPartDTO2DO(dtos []*prompt.ContentPart) []*entity.ContentPart {
	if dtos == nil {
		return nil
	}
	parts := make([]*entity.ContentPart, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		parts = append(parts, ContentPartDTO2DO(dto))
	}
	return parts
}

func ContentPartDTO2DO(dto *prompt.ContentPart) *entity.ContentPart {
	if dto == nil {
		return nil
	}

	return &entity.ContentPart{
		Type:        ContentTypeDTO2DO(dto.GetType()),
		Text:        dto.Text,
		ImageURL:    ImageURLDTO2DO(dto.ImageURL),
		VideoURL:    VideoURLDTO2DO(dto.VideoURL),
		MediaConfig: MediaConfigDTO2DO(dto.MediaConfig),
	}
}

func ContentTypeDTO2DO(dto prompt.ContentType) entity.ContentType {
	switch dto {
	case prompt.ContentTypeText:
		return entity.ContentTypeText
	case prompt.ContentTypeImageURL:
		return entity.ContentTypeImageURL
	case prompt.ContentTypeVideoURL:
		return entity.ContentTypeVideoURL
	case prompt.ContentTypeMultiPartVariable:
		return entity.ContentTypeMultiPartVariable
	default:
		return entity.ContentTypeText
	}
}

func ImageURLDTO2DO(dto *prompt.ImageURL) *entity.ImageURL {
	if dto == nil {
		return nil
	}

	return &entity.ImageURL{
		URI: dto.GetURI(),
		URL: dto.GetURL(),
	}
}

func VideoURLDTO2DO(dto *prompt.VideoURL) *entity.VideoURL {
	if dto == nil {
		return nil
	}

	return &entity.VideoURL{
		URI: dto.GetURI(),
		URL: dto.GetURL(),
	}
}

func MediaConfigDTO2DO(dto *prompt.MediaConfig) *entity.MediaConfig {
	if dto == nil {
		return nil
	}

	return &entity.MediaConfig{
		Fps: dto.Fps,
	}
}

func BatchVariableDefDTO2DO(dtos []*prompt.VariableDef) []*entity.VariableDef {
	if dtos == nil {
		return nil
	}
	variableDefs := make([]*entity.VariableDef, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		variableDefs = append(variableDefs, VariableDefDTO2DO(dto))
	}
	return variableDefs
}

func VariableDefDTO2DO(dto *prompt.VariableDef) *entity.VariableDef {
	if dto == nil {
		return nil
	}

	return &entity.VariableDef{
		Key:      dto.GetKey(),
		Desc:     dto.GetDesc(),
		Type:     VariableTypeDTO2DO(dto.GetType()),
		TypeTags: dto.TypeTags,
	}
}

func VariableTypeDTO2DO(dto prompt.VariableType) entity.VariableType {
	switch dto {
	case prompt.VariableTypeString:
		return entity.VariableTypeString
	case prompt.VariableTypePlaceholder:
		return entity.VariableTypePlaceholder
	case prompt.VariableTypeBoolean:
		return entity.VariableTypeBoolean
	case prompt.VariableTypeFloat:
		return entity.VariableTypeFloat
	case prompt.VariableTypeInteger:
		return entity.VariableTypeInteger
	case prompt.VariableTypeObject:
		return entity.VariableTypeObject
	case prompt.VariableTypeArrayString:
		return entity.VariableTypeArrayString
	case prompt.VariableTypeArrayInteger:
		return entity.VariableTypeArrayInteger
	case prompt.VariableTypeArrayFloat:
		return entity.VariableTypeArrayFloat
	case prompt.VariableTypeArrayBoolean:
		return entity.VariableTypeArrayBoolean
	case prompt.VariableTypeArrayObject:
		return entity.VariableTypeArrayObject
	case prompt.VariableTypeMultiPart:
		return entity.VariableTypeMultiPart
	default:
		return entity.VariableTypeString
	}
}

func BatchToolDTO2DO(dtos []*prompt.Tool) []*entity.Tool {
	if dtos == nil {
		return nil
	}
	tools := make([]*entity.Tool, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		tools = append(tools, ToolDTO2DO(dto))
	}
	return tools
}

func ToolDTO2DO(dto *prompt.Tool) *entity.Tool {
	if dto == nil {
		return nil
	}

	return &entity.Tool{
		Type:     ToolTypeDTO2DO(dto.GetType()),
		Function: FunctionDTO2DO(dto.Function),
	}
}

func FunctionDTO2DO(dto *prompt.Function) *entity.Function {
	if dto == nil {
		return nil
	}

	return &entity.Function{
		Name:        dto.GetName(),
		Description: dto.GetDescription(),
		Parameters:  dto.GetParameters(),
	}
}

func BatchToolCallDTO2DO(dtos []*prompt.ToolCall) []*entity.ToolCall {
	if dtos == nil {
		return nil
	}
	toolCalls := make([]*entity.ToolCall, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		toolCalls = append(toolCalls, ToolCallDTO2DO(dto))
	}
	return toolCalls
}

func ToolCallDTO2DO(dto *prompt.ToolCall) *entity.ToolCall {
	if dto == nil {
		return nil
	}

	return &entity.ToolCall{
		Index:        dto.GetIndex(),
		ID:           dto.GetID(),
		Type:         ToolTypeDTO2DO(dto.GetType()),
		FunctionCall: FunctionCallDTO2DO(dto.FunctionCall),
	}
}

func ToolTypeDTO2DO(dto prompt.ToolType) entity.ToolType {
	switch dto {
	case prompt.ToolTypeFunction:
		return entity.ToolTypeFunction
	case prompt.ToolTypeGoogleSearch:
		return entity.ToolTypeGoogleSearch
	default:
		return entity.ToolTypeFunction
	}
}

func FunctionCallDTO2DO(dto *prompt.FunctionCall) *entity.FunctionCall {
	if dto == nil {
		return nil
	}

	return &entity.FunctionCall{
		Name:      dto.GetName(),
		Arguments: dto.Arguments,
	}
}

func ToolCallConfigDTO2DO(dto *prompt.ToolCallConfig) *entity.ToolCallConfig {
	if dto == nil {
		return nil
	}

	return &entity.ToolCallConfig{
		ToolChoice:              ToolChoiceTypeDTO2DO(dto.GetToolChoice()),
		ToolChoiceSpecification: ToolChoiceSpecificationDTO2DO(dto.ToolChoiceSpecification),
	}
}

func ToolChoiceSpecificationDTO2DO(dto *prompt.ToolChoiceSpecification) *entity.ToolChoiceSpecification {
	if dto == nil {
		return nil
	}

	return &entity.ToolChoiceSpecification{
		Type: ToolTypeDTO2DO(dto.GetType()),
		Name: dto.GetName(),
	}
}

func ToolChoiceTypeDTO2DO(dto prompt.ToolChoiceType) entity.ToolChoiceType {
	switch dto {
	case prompt.ToolChoiceTypeNone:
		return entity.ToolChoiceTypeNone
	case prompt.ToolChoiceTypeAuto:
		return entity.ToolChoiceTypeAuto
	case prompt.ToolChoiceTypeSpecific:
		return entity.ToolChoiceTypeSpecific
	default:
		return entity.ToolChoiceTypeAuto
	}
}

func ModelConfigDTO2DO(dto *prompt.ModelConfig) *entity.ModelConfig {
	if dto == nil {
		return nil
	}

	return &entity.ModelConfig{
		ModelID:           dto.GetModelID(),
		MaxTokens:         dto.MaxTokens,
		Temperature:       dto.Temperature,
		TopK:              dto.TopK,
		TopP:              dto.TopP,
		PresencePenalty:   dto.PresencePenalty,
		FrequencyPenalty:  dto.FrequencyPenalty,
		JSONMode:          dto.JSONMode,
		Extra:            dto.Extra,
		ParamConfigValues: BatchParamConfigValueDTO2DO(dto.ParamConfigValues),
	}
}

func BatchParamConfigValueDTO2DO(dtos []*prompt.ParamConfigValue) []*entity.ParamConfigValue {
	if dtos == nil {
		return nil
	}
	result := make([]*entity.ParamConfigValue, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		result = append(result, ParamConfigValueDTO2DO(dto))
	}
	return result
}

func ParamConfigValueDTO2DO(dto *prompt.ParamConfigValue) *entity.ParamConfigValue {
	if dto == nil {
		return nil
	}
	return &entity.ParamConfigValue{
		Name:  ptr.From(dto.Name),
		Label: ptr.From(dto.Label),
		Value: ParamOptionDTO2DO(dto.Value),
	}
}

func ParamOptionDTO2DO(dto *prompt.ParamOption) *entity.ParamOption {
	if dto == nil {
		return nil
	}
	return &entity.ParamOption{
		Value: ptr.From(dto.Value),
		Label: ptr.From(dto.Label),
	}
}

func BatchVariableValDTO2DO(dtos []*prompt.VariableVal) []*entity.VariableVal {
	if dtos == nil {
		return nil
	}
	variableVals := make([]*entity.VariableVal, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		variableVals = append(variableVals, VariableValDTO2DO(dto))
	}
	return variableVals
}

func VariableValDTO2DO(dto *prompt.VariableVal) *entity.VariableVal {
	if dto == nil {
		return nil
	}
	return &entity.VariableVal{
		Key:                 dto.GetKey(),
		Value:               dto.Value,
		PlaceholderMessages: BatchMessageDTO2DO(dto.PlaceholderMessages),
		MultiPartValues:     BatchContentPartDTO2DO(dto.MultiPartValues),
	}
}

func ScenarioDTO2DO(dto prompt.Scenario) entity.Scenario {
	switch dto {
	case prompt.ScenarioEvalTarget:
		return entity.ScenarioEvalTarget
	default:
		return entity.ScenarioDefault
	}
}

// ====================================================================

func RoleDO2DTO(do entity.Role) prompt.Role {
	switch do {
	case entity.RoleSystem:
		return prompt.RoleSystem
	case entity.RoleUser:
		return prompt.RoleUser
	case entity.RoleAssistant:
		return prompt.RoleAssistant
	case entity.RoleTool:
		return prompt.RoleTool
	case entity.RolePlaceholder:
		return prompt.RolePlaceholder
	default:
		return prompt.RoleUser
	}
}

func BatchToolCallDO2DTO(dos []*entity.ToolCall) []*prompt.ToolCall {
	if dos == nil {
		return nil
	}
	toolCalls := make([]*prompt.ToolCall, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		toolCalls = append(toolCalls, ToolCallDO2DTO(do))
	}
	return toolCalls
}

func ToolCallDO2DTO(do *entity.ToolCall) *prompt.ToolCall {
	if do == nil {
		return nil
	}
	return &prompt.ToolCall{
		Index:        ptr.Of(do.Index),
		ID:           ptr.Of(do.ID),
		Type:         ptr.Of(ToolTypeDO2DTO(do.Type)),
		FunctionCall: FunctionCallDO2DTO(do.FunctionCall),
	}
}

func ToolTypeDO2DTO(do entity.ToolType) prompt.ToolType {
	switch do {
	case entity.ToolTypeFunction:
		return prompt.ToolTypeFunction
	case entity.ToolTypeGoogleSearch:
		return prompt.ToolTypeGoogleSearch
	default:
		return prompt.ToolTypeFunction
	}
}

func FunctionCallDO2DTO(do *entity.FunctionCall) *prompt.FunctionCall {
	if do == nil {
		return nil
	}
	return &prompt.FunctionCall{
		Name:      ptr.Of(do.Name),
		Arguments: do.Arguments,
	}
}

func TokenUsageDO2DTO(do *entity.TokenUsage) *prompt.TokenUsage {
	if do == nil {
		return nil
	}
	return &prompt.TokenUsage{
		InputTokens:  ptr.Of(do.InputTokens),
		OutputTokens: ptr.Of(do.OutputTokens),
	}
}

func BatchContentPartDO2DTO(dos []*entity.ContentPart) []*prompt.ContentPart {
	if dos == nil {
		return nil
	}
	parts := make([]*prompt.ContentPart, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		parts = append(parts, ContentPartDO2DTO(do))
	}
	return parts
}

func ContentPartDO2DTO(do *entity.ContentPart) *prompt.ContentPart {
	if do == nil {
		return nil
	}
	return &prompt.ContentPart{
		Type:        ptr.Of(ContentTypeDO2DTO(do.Type)),
		Text:        do.Text,
		ImageURL:    ImageURLDO2DTO(do.ImageURL),
		VideoURL:    VideoURLDO2DTO(do.VideoURL),
		MediaConfig: MediaConfigDO2DTO(do.MediaConfig),
	}
}

func ContentTypeDO2DTO(do entity.ContentType) prompt.ContentType {
	switch do {
	case entity.ContentTypeText:
		return prompt.ContentTypeText
	case entity.ContentTypeImageURL:
		return prompt.ContentTypeImageURL
	case entity.ContentTypeVideoURL:
		return prompt.ContentType("video_url")
	case entity.ContentTypeMultiPartVariable:
		return prompt.ContentTypeMultiPartVariable
	default:
		return prompt.ContentTypeText
	}
}

func ImageURLDO2DTO(do *entity.ImageURL) *prompt.ImageURL {
	if do == nil {
		return nil
	}
	return &prompt.ImageURL{
		URI: ptr.Of(do.URI),
		URL: ptr.Of(do.URL),
	}
}

func VideoURLDO2DTO(do *entity.VideoURL) *prompt.VideoURL {
	if do == nil {
		return nil
	}
	return &prompt.VideoURL{
		URI: ptr.Of(do.URI),
		URL: ptr.Of(do.URL),
	}
}

func MediaConfigDO2DTO(do *entity.MediaConfig) *prompt.MediaConfig {
	if do == nil {
		return nil
	}
	return &prompt.MediaConfig{
		Fps: do.Fps,
	}
}

func BatchDebugToolCallDO2DTO(dos []*entity.DebugToolCall) []*prompt.DebugToolCall {
	if dos == nil {
		return nil
	}
	toolCalls := make([]*prompt.DebugToolCall, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		toolCalls = append(toolCalls, DebugToolCallDO2DTO(do))
	}
	return toolCalls
}

func DebugToolCallDO2DTO(do *entity.DebugToolCall) *prompt.DebugToolCall {
	if do == nil {
		return nil
	}
	return &prompt.DebugToolCall{
		ToolCall:      ToolCallDO2DTO(&do.ToolCall),
		MockResponse:  ptr.Of(do.MockResponse),
		DebugTraceKey: ptr.Of(do.DebugTraceKey),
	}
}

func BatchVariableValDO2DTO(dos []*entity.VariableVal) []*prompt.VariableVal {
	if dos == nil {
		return nil
	}
	variableVals := make([]*prompt.VariableVal, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		variableVals = append(variableVals, VariableValDO2DTO(do))
	}
	return variableVals
}

func VariableValDO2DTO(do *entity.VariableVal) *prompt.VariableVal {
	if do == nil {
		return nil
	}
	return &prompt.VariableVal{
		Key:                 ptr.Of(do.Key),
		Value:               do.Value,
		PlaceholderMessages: BatchMessageDO2DTO(do.PlaceholderMessages),
		MultiPartValues:     BatchContentPartDO2DTO(do.MultiPartValues),
	}
}

func BatchMessageDO2DTO(dos []*entity.Message) []*prompt.Message {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*prompt.Message, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, MessageDO2DTO(do))
	}
	return dtos
}

func MessageDO2DTO(do *entity.Message) *prompt.Message {
	if do == nil {
		return nil
	}
	return &prompt.Message{
		Role:             ptr.Of(RoleDO2DTO(do.Role)),
		ReasoningContent: do.ReasoningContent,
		Content:          do.Content,
		Parts:            BatchContentPartDO2DTO(do.Parts),
		ToolCallID:       do.ToolCallID,
		ToolCalls:        BatchToolCallDO2DTO(do.ToolCalls),
		Metadata:         do.Metadata,
	}
}

func BatchPromptDO2DTO(dos []*entity.Prompt) []*prompt.Prompt {
	if len(dos) == 0 {
		return nil
	}
	prompts := make([]*prompt.Prompt, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		prompts = append(prompts, PromptDO2DTO(do))
	}
	if len(prompts) <= 0 {
		return nil
	}
	return prompts
}

func PromptDO2DTO(do *entity.Prompt) *prompt.Prompt {
	if do == nil {
		return nil
	}
	return &prompt.Prompt{
		ID:           ptr.Of(do.ID),
		WorkspaceID:  ptr.Of(do.SpaceID),
		PromptKey:    ptr.Of(do.PromptKey),
		PromptBasic:  PromptBasicDO2DTO(do.PromptBasic),
		PromptCommit: PromptCommitDO2DTO(do.PromptCommit),
		PromptDraft:  PromptDraftDO2DTO(do.PromptDraft),
	}
}

func PromptDraftDO2DTO(do *entity.PromptDraft) *prompt.PromptDraft {
	if do == nil {
		return nil
	}
	return &prompt.PromptDraft{
		DraftInfo: DraftInfoDO2DTO(do.DraftInfo),
		Detail:    PromptDetailDO2DTO(do.PromptDetail),
	}
}

func DraftInfoDO2DTO(do *entity.DraftInfo) *prompt.DraftInfo {
	if do == nil {
		return nil
	}
	return &prompt.DraftInfo{
		UserID:      ptr.Of(do.UserID),
		BaseVersion: ptr.Of(do.BaseVersion),
		IsModified:  ptr.Of(do.IsModified),

		CreatedAt: ptr.Of(do.CreatedAt.UnixMilli()),
		UpdatedAt: ptr.Of(do.UpdatedAt.UnixMilli()),
	}
}

func PromptBasicDO2DTO(do *entity.PromptBasic) *prompt.PromptBasic {
	if do == nil {
		return nil
	}
	return &prompt.PromptBasic{
		DisplayName:   ptr.Of(do.DisplayName),
		Description:   ptr.Of(do.Description),
		LatestVersion: ptr.Of(do.LatestVersion),
		CreatedBy:     ptr.Of(do.CreatedBy),
		UpdatedBy:     ptr.Of(do.UpdatedBy),
		CreatedAt:     ptr.Of(do.CreatedAt.UnixMilli()),
		UpdatedAt:     ptr.Of(do.UpdatedAt.UnixMilli()),
		LatestCommittedAt: func() *int64 {
			if do.LatestCommittedAt == nil {
				return nil
			}
			return ptr.Of(do.LatestCommittedAt.UnixMilli())
		}(),
		PromptType: ptr.Of(PromptTypeDO2DTO(do.PromptType)),
	}
}

func PromptTypeDO2DTO(do entity.PromptType) prompt.PromptType {
	switch do {
	case entity.PromptTypeNormal:
		return prompt.PromptTypeNormal
	case entity.PromptTypeSnippet:
		return prompt.PromptTypeSnippet
	default:
		return prompt.PromptTypeNormal
	}
}

func BatchPromptCommitDO2DTO(dos []*entity.PromptCommit) []*prompt.PromptCommit {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*prompt.PromptCommit, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, PromptCommitDO2DTO(do))
	}
	return dtos
}

func PromptCommitDO2DTO(do *entity.PromptCommit) *prompt.PromptCommit {
	if do == nil {
		return nil
	}
	return &prompt.PromptCommit{
		CommitInfo: CommitInfoDO2DTO(do.CommitInfo),
		Detail:     PromptDetailDO2DTO(do.PromptDetail),
	}
}

func BatchCommitInfoDO2DTO(dos []*entity.CommitInfo) []*prompt.CommitInfo {
	if len(dos) <= 0 {
		return nil
	}
	dtos := make([]*prompt.CommitInfo, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, CommitInfoDO2DTO(do))
	}
	if len(dtos) <= 0 {
		return nil
	}
	return dtos
}

func CommitInfoDO2DTO(do *entity.CommitInfo) *prompt.CommitInfo {
	if do == nil {
		return nil
	}
	return &prompt.CommitInfo{
		Version:     ptr.Of(do.Version),
		BaseVersion: ptr.Of(do.BaseVersion),
		Description: ptr.Of(do.Description),
		CommittedBy: ptr.Of(do.CommittedBy),
		CommittedAt: ptr.Of(do.CommittedAt.UnixMilli()),
	}
}

func PromptDetailDO2DTO(do *entity.PromptDetail) *prompt.PromptDetail {
	if do == nil {
		return nil
	}
	return &prompt.PromptDetail{
		PromptTemplate: PromptTemplateDO2DTO(do.PromptTemplate),
		Tools:          BatchToolDO2DTO(do.Tools),
		ToolCallConfig: ToolCallConfigDO2DTO(do.ToolCallConfig),
		ModelConfig:    ModelConfigDO2DTO(do.ModelConfig),
		ExtInfos:       do.ExtInfos,
	}
}

func ModelConfigDO2DTO(do *entity.ModelConfig) *prompt.ModelConfig {
	if do == nil {
		return nil
	}
	return &prompt.ModelConfig{
		ModelID:           ptr.Of(do.ModelID),
		MaxTokens:         do.MaxTokens,
		Temperature:       do.Temperature,
		TopK:              do.TopK,
		TopP:              do.TopP,
		PresencePenalty:   do.PresencePenalty,
		FrequencyPenalty:  do.FrequencyPenalty,
		JSONMode:          do.JSONMode,
		Extra:             do.Extra,
		ParamConfigValues: BatchParamConfigValueDO2DTO(do.ParamConfigValues),
	}
}

func BatchParamConfigValueDO2DTO(dos []*entity.ParamConfigValue) []*prompt.ParamConfigValue {
	if dos == nil {
		return nil
	}
	result := make([]*prompt.ParamConfigValue, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		result = append(result, ParamConfigValueDO2DTO(do))
	}
	return result
}

func ParamConfigValueDO2DTO(do *entity.ParamConfigValue) *prompt.ParamConfigValue {
	if do == nil {
		return nil
	}
	return &prompt.ParamConfigValue{
		Name:  ptr.Of(do.Name),
		Label: ptr.Of(do.Label),
		Value: ParamOptionDO2DTO(do.Value),
	}
}

func ParamOptionDO2DTO(do *entity.ParamOption) *prompt.ParamOption {
	if do == nil {
		return nil
	}
	return &prompt.ParamOption{
		Value: ptr.Of(do.Value),
		Label: ptr.Of(do.Label),
	}
}

func ToolCallConfigDO2DTO(do *entity.ToolCallConfig) *prompt.ToolCallConfig {
	if do == nil {
		return nil
	}
	return &prompt.ToolCallConfig{
		ToolChoice:              ptr.Of(prompt.ToolChoiceType(do.ToolChoice)),
		ToolChoiceSpecification: ToolChoiceSpecificationDO2DTO(do.ToolChoiceSpecification),
	}
}

func ToolChoiceSpecificationDO2DTO(do *entity.ToolChoiceSpecification) *prompt.ToolChoiceSpecification {
	if do == nil {
		return nil
	}
	return &prompt.ToolChoiceSpecification{
		Type: ptr.Of(prompt.ToolType(do.Type)),
		Name: ptr.Of(do.Name),
	}
}

func BatchToolDO2DTO(dos []*entity.Tool) []*prompt.Tool {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*prompt.Tool, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, ToolDO2DTO(do))
	}
	return dtos
}

func ToolDO2DTO(do *entity.Tool) *prompt.Tool {
	if do == nil {
		return nil
	}
	return &prompt.Tool{
		Type:     ptr.Of(prompt.ToolType(do.Type)),
		Function: FunctionDO2DTO(do.Function),
	}
}

func FunctionDO2DTO(do *entity.Function) *prompt.Function {
	if do == nil {
		return nil
	}
	return &prompt.Function{
		Name:        ptr.Of(do.Name),
		Description: ptr.Of(do.Description),
		Parameters:  ptr.Of(do.Parameters),
	}
}

func PromptTemplateDO2DTO(do *entity.PromptTemplate) *prompt.PromptTemplate {
	if do == nil {
		return nil
	}
	return &prompt.PromptTemplate{
		TemplateType: ptr.Of(prompt.TemplateType(do.TemplateType)),
		Messages:     BatchMessageDO2DTO(do.Messages),
		VariableDefs: BatchVariableDefDO2DTO(do.VariableDefs),
		HasSnippet:   ptr.Of(do.HasSnippets),
		Snippets:     BatchPromptDO2DTO(do.Snippets),
		Metadata:     do.Metadata,
	}
}

func BatchVariableDefDO2DTO(dos []*entity.VariableDef) []*prompt.VariableDef {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*prompt.VariableDef, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, VariableDefDO2DTO(do))
	}
	return dtos
}

func VariableDefDO2DTO(do *entity.VariableDef) *prompt.VariableDef {
	if do == nil {
		return nil
	}
	return &prompt.VariableDef{
		Key:      ptr.Of(do.Key),
		Desc:     ptr.Of(do.Desc),
		Type:     ptr.Of(prompt.VariableType(do.Type)),
		TypeTags: do.TypeTags,
	}
}
