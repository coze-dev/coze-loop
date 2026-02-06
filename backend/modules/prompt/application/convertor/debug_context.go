// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func DebugContextDTO2DO(promptID int64, userID string, dto *prompt.DebugContext) *entity.DebugContext {
	if dto == nil {
		return &entity.DebugContext{
			PromptID: promptID,
			UserID:   userID,
		}
	}
	return &entity.DebugContext{
		PromptID:      promptID,
		UserID:        userID,
		DebugCore:     DebugCoreDTO2DO(dto.GetDebugCore()),
		DebugConfig:   DebugConfigDTO2DO(dto.GetDebugConfig()),
		CompareConfig: CompareConfigDTO2DO(dto.GetCompareConfig()),
	}
}

func DebugCoreDTO2DO(dto *prompt.DebugCore) *entity.DebugCore {
	if dto == nil {
		return nil
	}
	return &entity.DebugCore{
		MockContexts:  DebugMessagesDTO2DO(dto.GetMockContexts()),
		MockVariables: BatchVariableValDTO2DO(dto.GetMockVariables()),
		MockTools:     MockToolsDTO2DO(dto.GetMockTools()),
	}
}

func DebugMessagesDTO2DO(dtos []*prompt.DebugMessage) []*entity.DebugMessage {
	if len(dtos) == 0 {
		return nil
	}
	res := make([]*entity.DebugMessage, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, DebugMessageDTO2DO(dto))
	}
	return res
}

func DebugMessageDTO2DO(dto *prompt.DebugMessage) *entity.DebugMessage {
	if dto == nil {
		return nil
	}
	return &entity.DebugMessage{
		Role:             RoleDTO2DO(dto.GetRole()),
		ReasoningContent: dto.ReasoningContent,
		Content:          dto.Content,
		Parts:            BatchContentPartDTO2DO(dto.GetParts()),
		ToolCallID:       dto.ToolCallID,
		ToolCalls:        DebugToolCallsDTO2DO(dto.GetToolCalls()),
		Signature:        dto.Signature,
		DebugID:          dto.DebugID,
		InputTokens:      dto.InputTokens,
		OutputTokens:     dto.OutputTokens,
		CostMS:           dto.CostMs,
	}
}

func DebugToolCallsDTO2DO(dtos []*prompt.DebugToolCall) []*entity.DebugToolCall {
	if len(dtos) == 0 {
		return nil
	}
	res := make([]*entity.DebugToolCall, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, DebugToolCallDTO2DO(dto))
	}
	return res
}

func DebugToolCallDTO2DO(dto *prompt.DebugToolCall) *entity.DebugToolCall {
	if dto == nil {
		return nil
	}
	return &entity.DebugToolCall{
		ToolCall:      ptr.From(ToolCallDTO2DO(dto.GetToolCall())),
		MockResponse:  dto.GetMockResponse(),
		DebugTraceKey: dto.GetDebugTraceKey(),
	}
}

func MockToolsDTO2DO(dtos []*prompt.MockTool) []*entity.MockTool {
	if len(dtos) == 0 {
		return nil
	}
	res := make([]*entity.MockTool, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, MockToolDTO2DO(dto))
	}
	return res
}

func MockToolDTO2DO(dto *prompt.MockTool) *entity.MockTool {
	if dto == nil {
		return nil
	}
	return &entity.MockTool{
		Name:         dto.GetName(),
		MockResponse: dto.GetMockResponse(),
	}
}

func DebugConfigDTO2DO(dto *prompt.DebugConfig) *entity.DebugConfig {
	if dto == nil {
		return nil
	}
	return &entity.DebugConfig{
		SingleStepDebug: dto.SingleStepDebug,
	}
}

func CompareConfigDTO2DO(dto *prompt.CompareConfig) *entity.CompareConfig {
	if dto == nil {
		return nil
	}
	return &entity.CompareConfig{
		Groups: BatchCompareGroupDTO2DO(dto.GetGroups()),
	}
}

func BatchCompareGroupDTO2DO(dtos []*prompt.CompareGroup) []*entity.CompareGroup {
	if len(dtos) == 0 {
		return nil
	}
	res := make([]*entity.CompareGroup, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		res = append(res, CompareGroupDTO2DO(dto))
	}
	return res
}

func CompareGroupDTO2DO(dto *prompt.CompareGroup) *entity.CompareGroup {
	if dto == nil {
		return nil
	}
	return &entity.CompareGroup{
		PromptDetail: PromptDetailDTO2DO(dto.GetPromptDetail()),
		DebugCore:    DebugCoreDTO2DO(dto.GetDebugCore()),
	}
}

//===================================================================

func DebugContextDO2DTO(do *entity.DebugContext) *prompt.DebugContext {
	if do == nil {
		return nil
	}
	return &prompt.DebugContext{
		DebugCore:     DebugCoreDO2DTO(do.DebugCore),
		DebugConfig:   DebugConfigDO2DTO(do.DebugConfig),
		CompareConfig: CompareConfigDO2DTO(do.CompareConfig),
	}
}

func DebugCoreDO2DTO(do *entity.DebugCore) *prompt.DebugCore {
	if do == nil {
		return nil
	}
	return &prompt.DebugCore{
		MockContexts:  DebugMessagesDO2DTO(do.MockContexts),
		MockVariables: BatchVariableValDO2DTO(do.MockVariables),
		MockTools:     MockToolsDO2DTO(do.MockTools),
	}
}

func DebugMessagesDO2DTO(dos []*entity.DebugMessage) []*prompt.DebugMessage {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*prompt.DebugMessage, 0, len(dos))
	for _, dto := range dos {
		res = append(res, DebugMessageDO2DTO(dto))
	}
	return res
}

func DebugMessageDO2DTO(do *entity.DebugMessage) *prompt.DebugMessage {
	if do == nil {
		return nil
	}
	return &prompt.DebugMessage{
		Role:             ptr.Of(RoleDO2DTO(do.Role)),
		ReasoningContent: do.ReasoningContent,
		Content:          do.Content,
		Parts:            BatchContentPartDO2DTO(do.Parts),
		ToolCallID:       do.ToolCallID,
		ToolCalls:        BatchDebugToolCallDO2DTO(do.ToolCalls),
		Signature:        do.Signature,
		DebugID:          do.DebugID,
		InputTokens:      do.InputTokens,
		OutputTokens:     do.OutputTokens,
		CostMs:           do.CostMS,
	}
}

func MockToolsDO2DTO(dos []*entity.MockTool) []*prompt.MockTool {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*prompt.MockTool, 0, len(dos))
	for _, dto := range dos {
		res = append(res, MockToolDO2DTO(dto))
	}
	return res
}

func MockToolDO2DTO(do *entity.MockTool) *prompt.MockTool {
	if do == nil {
		return nil
	}
	return &prompt.MockTool{
		Name:         ptr.Of(do.Name),
		MockResponse: ptr.Of(do.MockResponse),
	}
}

func DebugConfigDO2DTO(do *entity.DebugConfig) *prompt.DebugConfig {
	if do == nil {
		return nil
	}
	return &prompt.DebugConfig{
		SingleStepDebug: do.SingleStepDebug,
	}
}

func CompareConfigDO2DTO(do *entity.CompareConfig) *prompt.CompareConfig {
	if do == nil {
		return nil
	}
	return &prompt.CompareConfig{
		Groups: BatchCompareGroupDO2DTO(do.Groups),
	}
}

func BatchCompareGroupDO2DTO(dos []*entity.CompareGroup) []*prompt.CompareGroup {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*prompt.CompareGroup, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		res = append(res, CompareGroupDO2DTO(do))
	}
	return res
}

func CompareGroupDO2DTO(do *entity.CompareGroup) *prompt.CompareGroup {
	if do == nil {
		return nil
	}
	return &prompt.CompareGroup{
		PromptDetail: PromptDetailDO2DTO(do.PromptDetail),
		DebugCore:    DebugCoreDO2DTO(do.DebugCore),
	}
}
