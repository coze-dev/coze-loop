// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func ConvertToLoopPrompts(ps []*prompt.Prompt) []*rpc.LoopPrompt {
	if ps == nil {
		return nil
	}
	res := make([]*rpc.LoopPrompt, 0)
	for _, p := range ps {
		res = append(res, ConvertToLoopPrompt(p))
	}
	return res
}

func ConvertToLoopPrompt(p *prompt.Prompt) *rpc.LoopPrompt {
	if p == nil {
		return nil
	}
	res := &rpc.LoopPrompt{
		ID:        p.GetID(),
		PromptKey: p.GetPromptKey(),
		PromptBasic: &rpc.PromptBasic{
			DisplayName:   gptr.Of(p.GetPromptBasic().GetDisplayName()),
			Description:   gptr.Of(p.GetPromptBasic().GetDescription()),
			LatestVersion: gptr.Of(p.GetPromptBasic().GetLatestVersion()),
		},
		PromptCommit: &rpc.PromptCommit{
			Detail: &rpc.PromptDetail{
				PromptTemplate: &rpc.PromptTemplate{
					VariableDefs: gslice.Map(p.GetPromptCommit().GetDetail().GetPromptTemplate().GetVariableDefs(), func(p *prompt.VariableDef) *rpc.VariableDef {
						return &rpc.VariableDef{
							Key:      gptr.Of(p.GetKey()),
							Type:     gptr.Of(p.GetType()),
							TypeTags: p.TypeTags,
						}
					}),
				},
			},
			CommitInfo: &rpc.CommitInfo{
				Version:     gptr.Of(p.GetPromptCommit().GetCommitInfo().GetVersion()),
				BaseVersion: gptr.Of(p.GetPromptCommit().GetCommitInfo().GetBaseVersion()),
				Description: gptr.Of(p.GetPromptCommit().GetCommitInfo().GetDescription()),
				CommittedAt: gptr.Of(p.GetPromptCommit().GetCommitInfo().GetCommittedAt()),
				CommittedBy: gptr.Of(p.GetPromptCommit().GetCommitInfo().GetCommittedBy()),
			},
		},
	}
	return res
}

func ConvertVariables2Prompt(fromVals []*entity.VariableVal) (toVals []*prompt.VariableVal) {
	if len(fromVals) == 0 {
		return toVals
	}
	toVals = make([]*prompt.VariableVal, 0)
	for _, v := range fromVals {
		toVals = append(toVals, &prompt.VariableVal{
			Key:                 v.Key,
			Value:               v.Value,
			PlaceholderMessages: ConvertMessages2Prompt(v.PlaceholderMessages),
			MultiPartValues:     ConvertContent(v.Content),
		})
	}
	return toVals
}

func ConvertMessages2Prompt(fromMsg []*entity.Message) (toMsg []*prompt.Message) {
	if len(fromMsg) == 0 {
		return toMsg
	}
	toMsg = make([]*prompt.Message, 0)
	for _, m := range fromMsg {
		if m == nil || m.Content == nil {
			continue
		}
		if m.Content.GetContentType() == entity.ContentTypeText {
			toMsg = append(toMsg, &prompt.Message{
				Role:    gptr.Of(Role2PromptRole(m.Role)),
				Content: m.Content.Text,
			})
		} else {
			toMsg = append(toMsg, &prompt.Message{
				Role:  gptr.Of(Role2PromptRole(m.Role)),
				Parts: ConvertContent(m.Content),
			})
		}
	}
	return toMsg
}

func ConvertPromptToolCalls2Eval(promptToolCalls []*prompt.ToolCall) []*entity.ToolCall {
	if len(promptToolCalls) == 0 {
		return nil
	}
	res := make([]*entity.ToolCall, 0)
	for _, t := range promptToolCalls {
		res = append(res, &entity.ToolCall{
			Index: gptr.Indirect(t.Index),
			ID:    gptr.Indirect(t.ID),
			Type:  entity.ToolTypeFunction,
			FunctionCall: &entity.FunctionCall{
				Name:      gptr.Indirect(gptr.Indirect(t.FunctionCall).Name),
				Arguments: t.FunctionCall.Arguments,
			},
		})
	}
	return res
}

func Role2PromptRole(role entity.Role) prompt.Role {
	switch role {
	case entity.RoleSystem:
		return prompt.RoleSystem
	case entity.RoleUser:
		return prompt.RoleUser
	case entity.RoleAssistant:
		return prompt.RoleAssistant
	case entity.RoleTool:
		return prompt.RoleTool
	default:
		// follow prompt's logic
		return prompt.RoleUser
	}
}

func ConvertContent(content *entity.Content) []*prompt.ContentPart {
	if content == nil {
		return nil
	}
	switch content.GetContentType() {
	case entity.ContentTypeText:
		return []*prompt.ContentPart{
			{
				Type: gptr.Of(prompt.ContentTypeText),
				Text: gptr.Of(content.GetText()),
			},
		}
	case entity.ContentTypeImage:
		return []*prompt.ContentPart{
			{
				Type: gptr.Of(prompt.ContentTypeImageURL),
				ImageURL: &prompt.ImageURL{
					URL: content.Image.URL,
					URI: content.Image.URI,
				},
			},
		}
	case entity.ContentTypeMultipart:
		cps := make([]*prompt.ContentPart, 0, len(content.MultiPart))
		for _, sub := range content.MultiPart {
			cps = append(cps, ConvertContent(sub)...)
		}
		return cps
	default:
		return []*prompt.ContentPart{}
	}
}

func ConvertFromContent(parts []*prompt.ContentPart) *entity.Content {
	if len(parts) == 0 {
		return nil
	}

	content := &entity.Content{ContentType: gptr.Of(entity.ContentTypeMultipart)}
	for _, part := range parts {
		if part == nil {
			continue
		}
		switch part.GetType() {
		case prompt.ContentTypeText:
			content.MultiPart = append(content.MultiPart, &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        part.Text,
			})
		case prompt.ContentTypeImageURL:
			content.MultiPart = append(content.MultiPart, &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeImage),
				Image: &entity.Image{
					URL: part.ImageURL.URL,
					URI: part.ImageURL.URI,
				},
			})
		default:
		}
	}
	return content
}
