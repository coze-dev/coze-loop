// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/coze-dev/cozeloop-go/spec/tracespec"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

func MergeStreamMsgs(msgs []*Message) *Message {
	if len(msgs) == 0 {
		return nil
	}
	allInOne := &Message{
		Role:         RoleAssistant,
		ResponseMeta: &ResponseMeta{},
	}
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		allInOne.Content += msg.Content
		allInOne.ReasoningContent += msg.ReasoningContent
		if msg.MultiModalContent != nil {
			allInOne.MultiModalContent = append(allInOne.MultiModalContent, msg.MultiModalContent...)
		}
		if msg.ResponseMeta != nil {
			if msg.ResponseMeta.Usage != nil {
				allInOne.ResponseMeta.Usage = msg.ResponseMeta.Usage
			}
			if msg.ResponseMeta.FinishReason != "" {
				allInOne.ResponseMeta.FinishReason = msg.ResponseMeta.FinishReason
			}
		}
		// 流式tool call聚合
		for _, tc := range msg.ToolCalls {
			if tc == nil {
				continue
			}
			if len(allInOne.ToolCalls) == 0 || isFirstStreamPkgToolCall(allInOne.ToolCalls[len(allInOne.ToolCalls)-1], tc) {
				allInOne.ToolCalls = append(allInOne.ToolCalls, &ToolCall{
					Index:    tc.Index,
					ID:       tc.ID,
					Type:     tc.Type,
					Function: tc.Function,
					Extra:    tc.Extra,
				})
			} else {
				allInOne.ToolCalls[len(allInOne.ToolCalls)-1].Function.Arguments += tc.Function.Arguments
				allInOne.ToolCalls[len(allInOne.ToolCalls)-1].Extra = tc.Extra
			}
		}
	}
	return allInOne
}

func isFirstStreamPkgToolCall(tc1, tc2 *ToolCall) bool {
	if tc1 == nil || tc2 == nil {
		return false
	}
	if tc2.ID == "" {
		return false
	}
	return true
}

func StreamMsgsToTraceModelChoices(msgs []*Message) *tracespec.ModelOutput {
	res := &tracespec.ModelOutput{}
	// merge messages
	allInOne := MergeStreamMsgs(msgs)
	res.Choices = append(res.Choices, MsgToTraceModelChoice(allInOne))
	return res
}

func MsgToTraceModelChoice(msg *Message) *tracespec.ModelChoice {
	if msg == nil {
		return nil
	}
	var finishReason string
	if msg.ResponseMeta != nil {
		finishReason = msg.ResponseMeta.FinishReason
	}
	return &tracespec.ModelChoice{
		FinishReason: finishReason,
		Index:        0,
		Message: &tracespec.ModelMessage{
			Role:             tracespec.VRoleAssistant,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
			Parts:            PartsToTraceMessageParts(msg.MultiModalContent),
			ToolCalls:        ToolCallsToTraceToolCalls(msg.ToolCalls),
		},
	}
}

func MsgsToTraceMsgs(ms []*Message) []*tracespec.ModelMessage {
	return slices.Transform(ms, func(m *Message, _ int) *tracespec.ModelMessage {
		return MsgToTraceMsg(m)
	})
}

func MsgToTraceMsg(m *Message) *tracespec.ModelMessage {
	if m == nil {
		return nil
	}
	return &tracespec.ModelMessage{
		Role:             string(m.Role),
		Content:          m.Content,
		ReasoningContent: m.ReasoningContent,
		Parts:            PartsToTraceMessageParts(m.MultiModalContent),
		Name:             m.Name,
		ToolCalls:        ToolCallsToTraceToolCalls(m.ToolCalls),
		ToolCallID:       m.ToolCallID,
	}
}

func ToTraceModelInput(msgs []*Message, ts []*ToolInfo, tc *ToolChoice) *tracespec.ModelInput {
	return &tracespec.ModelInput{
		Messages:        MsgsToTraceMsgs(msgs),
		Tools:           ToolsToTraceTools(ts),
		ModelToolChoice: ToolChoiceToTraceToolChoice(tc),
	}
}

func ToolChoiceToTraceToolChoice(tc *ToolChoice) *tracespec.ModelToolChoice {
	if tc == nil {
		return nil
	}
	return &tracespec.ModelToolChoice{
		Type: string(*tc),
	}
}

func ToolsToTraceTools(ts []*ToolInfo) []*tracespec.ModelTool {
	return slices.Transform(ts, func(t *ToolInfo, _ int) *tracespec.ModelTool {
		return ToolToTraceTool(t)
	})
}

func ToolToTraceTool(t *ToolInfo) *tracespec.ModelTool {
	if t == nil {
		return nil
	}
	return &tracespec.ModelTool{
		Type: tracespec.VToolChoiceFunction,
		Function: &tracespec.ModelToolFunction{
			Name:        t.Name,
			Description: t.Desc,
			Parameters:  []byte(t.Def),
		},
	}
}

func PartsToTraceMessageParts(ps []*ChatMessagePart) []*tracespec.ModelMessagePart {
	return slices.Transform(ps, func(p *ChatMessagePart, _ int) *tracespec.ModelMessagePart {
		return PartToTraceMessagePart(p)
	})
}

func PartToTraceMessagePart(p *ChatMessagePart) *tracespec.ModelMessagePart {
	if p == nil {
		return nil
	}
	switch p.Type {
	case ChatMessagePartTypeText:
		return &tracespec.ModelMessagePart{
			Type: tracespec.ModelMessagePartType(p.Type),
			Text: p.Text,
		}
	case ChatMessagePartTypeImageURL:
		return &tracespec.ModelMessagePart{
			Type: tracespec.ModelMessagePartType(p.Type),
			ImageURL: &tracespec.ModelImageURL{
				URL:    p.ImageURL.URL,
				Detail: string(p.ImageURL.Detail),
			},
		}
	case ChatMessagePartTypeVideoURL:
		//// todo @yanghoule
		//return &tracespec.ModelMessagePart{
		//	Type: tracespec.ModelMessagePartType(p.Type),
		//}
	}
	return nil
}

func ToolCallsToTraceToolCalls(ts []*ToolCall) []*tracespec.ModelToolCall {
	return slices.Transform(ts, func(t *ToolCall, _ int) *tracespec.ModelToolCall {
		return ToolCallToTraceToolCall(t)
	})
}

func ToolCallToTraceToolCall(t *ToolCall) *tracespec.ModelToolCall {
	if t == nil {
		return nil
	}
	var fc *tracespec.ModelToolCallFunction
	if t.Function != nil {
		fc = &tracespec.ModelToolCallFunction{
			Name:      t.Function.Name,
			Arguments: t.Function.Arguments,
		}
	}
	return &tracespec.ModelToolCall{
		ID:       t.ID,
		Type:     t.Type,
		Function: fc,
	}
}

func OptionsToTrace(os []Option) *tracespec.ModelCallOption {
	opts := ApplyOptions(nil, os...)
	res := &tracespec.ModelCallOption{
		Stop: opts.Stop,
	}
	if opts.Temperature != nil {
		res.Temperature = *opts.Temperature
	}
	if opts.MaxTokens != nil {
		res.MaxTokens = int64(*opts.MaxTokens)
	}
	if opts.TopP != nil {
		res.TopP = *opts.TopP
	}
	return res
}
