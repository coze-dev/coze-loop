// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/mem"
)

type Prompt struct {
	ID           int64         `json:"id"`
	SpaceID      int64         `json:"space_id"`
	PromptKey    string        `json:"prompt_key"`
	PromptBasic  *PromptBasic  `json:"prompt_basic,omitempty"`
	PromptDraft  *PromptDraft  `json:"prompt_draft,omitempty"`
	PromptCommit *PromptCommit `json:"prompt_commit,omitempty"`
}

type PromptDraft struct {
	PromptDetail *PromptDetail `json:"prompt_detail,omitempty"`
	DraftInfo    *DraftInfo    `json:"draft_info,omitempty"`
}

type PromptCommit struct {
	PromptDetail *PromptDetail `json:"prompt_detail,omitempty"`
	CommitInfo   *CommitInfo   `json:"commit_info,omitempty"`
}

type DraftInfo struct {
	UserID      string    `json:"user_id"`
	BaseVersion string    `json:"base_version"`
	IsModified  bool      `json:"is_modified"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CommitInfo struct {
	Version     string    `json:"version"`
	BaseVersion string    `json:"base_version"`
	Description string    `json:"description"`
	CommittedBy string    `json:"committed_by"`
	CommittedAt time.Time `json:"committed_at"`
}

func (p *Prompt) Clone() *Prompt {
	if p == nil {
		return nil
	}
	copiedPrompt := &Prompt{}
	_ = mem.DeepCopy(p, copiedPrompt)
	return copiedPrompt
}

func (p *Prompt) CloneDetail() *Prompt {
	if p == nil {
		return nil
	}
	copiedPrompt := p.Clone()
	if copiedPrompt == nil {
		return nil
	}

	copiedPrompt.ID = 0
	copiedPrompt.PromptKey = ""
	copiedPrompt.PromptBasic = nil
	if copiedPrompt.PromptCommit != nil {
		copiedPrompt.PromptCommit.CommitInfo = nil
	}
	if copiedPrompt.PromptDraft != nil {
		copiedPrompt.PromptDraft.DraftInfo = nil
	}

	return copiedPrompt
}

func (p *Prompt) GetVersion() string {
	if p == nil {
		return ""
	}
	var version string
	if p.PromptCommit != nil && p.PromptCommit.CommitInfo != nil {
		version = p.PromptCommit.CommitInfo.Version
	}
	return version
}

func (p *Prompt) GetPromptDetail() *PromptDetail {
	if p == nil {
		return nil
	}
	if p.PromptDraft != nil && p.PromptDraft.PromptDetail != nil {
		return p.PromptDraft.PromptDetail
	}
	if p.PromptCommit != nil && p.PromptCommit.PromptDetail != nil {
		return p.PromptCommit.PromptDetail
	}
	return nil
}

func (p *Prompt) FormatMessages(messages []*Message, variableVals []*VariableVal) (formattedMessages []*Message, err error) {
	if p == nil {
		return nil, nil
	}
	var promptTemplate *PromptTemplate
	if promptDetail := p.GetPromptDetail(); promptDetail != nil {
		promptTemplate = promptDetail.PromptTemplate
	}
	if promptTemplate == nil {
		return nil, nil
	}
	return promptTemplate.formatMessages(messages, variableVals)
}

func (p *Prompt) GetTemplateMessages(messages []*Message) []*Message {
	if p == nil {
		return nil
	}
	var promptTemplate *PromptTemplate
	if promptDetail := p.GetPromptDetail(); promptDetail != nil {
		promptTemplate = promptDetail.PromptTemplate
	}
	if promptTemplate == nil {
		return nil
	}
	return promptTemplate.getTemplateMessages(messages)
}
