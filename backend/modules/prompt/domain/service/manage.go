// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func (p *PromptServiceImpl) MGetPromptIDs(ctx context.Context, spaceID int64, promptKeys []string) (PromptKeyIDMap map[string]int64, err error) {
	promptKeyIDMap := make(map[string]int64)
	if len(promptKeys) == 0 {
		return promptKeyIDMap, nil
	}
	basics, err := p.manageRepo.MGetPromptBasicByPromptKey(ctx, spaceID, promptKeys, repo.WithPromptBasicCacheEnable())
	if err != nil {
		return nil, err
	}
	for _, basic := range basics {
		promptKeyIDMap[basic.PromptKey] = basic.ID
	}
	for _, promptKey := range promptKeys {
		if _, ok := promptKeyIDMap[promptKey]; !ok {
			return nil, errorx.NewByCode(prompterr.ResourceNotFoundCode,
				errorx.WithExtraMsg(fmt.Sprintf("prompt key: %s not found", promptKey)),
				errorx.WithExtra(map[string]string{"prompt_key": promptKey}))
		}
	}
	return promptKeyIDMap, nil
}

func (p *PromptServiceImpl) MParseCommitVersionByPromptKey(ctx context.Context, spaceID int64, pairs []PromptKeyVersionPair) (promptKeyCommitVersionMap map[PromptKeyVersionPair]string, err error) {
	promptKeyCommitVersionMap = make(map[PromptKeyVersionPair]string)
	var emptyVersionPromptKeys []string
	for _, pair := range pairs {
		if pair.Version == "" {
			emptyVersionPromptKeys = append(emptyVersionPromptKeys, pair.PromptKey)
		}
		// 不管原始版本号是否为空，都先用原始版本号占位
		promptKeyCommitVersionMap[pair] = pair.Version
	}
	if len(emptyVersionPromptKeys) == 0 {
		return promptKeyCommitVersionMap, nil
	}
	basics, err := p.manageRepo.MGetPromptBasicByPromptKey(ctx, spaceID, emptyVersionPromptKeys, repo.WithPromptBasicCacheEnable())
	if err != nil {
		return nil, err
	}
	for _, basic := range basics {
		if basic != nil && basic.PromptBasic != nil {
			lastestCommitVersion := basic.PromptBasic.LatestVersion
			if lastestCommitVersion == "" {
				return nil, errorx.NewByCode(prompterr.PromptUncommittedCode, errorx.WithExtraMsg(fmt.Sprintf("prompt key: %s", basic.PromptKey)), errorx.WithExtra(map[string]string{"prompt_key": basic.PromptKey}))
			}
			promptKeyCommitVersionMap[PromptKeyVersionPair{PromptKey: basic.PromptKey}] = lastestCommitVersion
		}
	}
	return promptKeyCommitVersionMap, nil
}

func (p *PromptServiceImpl) MCompleteMultiModalFileURL(ctx context.Context, messages []*entity.Message) error {
	var fileKeys []string
	for _, message := range messages {
		if message == nil || len(message.Parts) == 0 {
			continue
		}
		for _, part := range message.Parts {
			if part == nil || part.ImageURL == nil {
				continue
			}
			fileKeys = append(fileKeys, part.ImageURL.URI)
		}
	}
	if len(fileKeys) == 0 {
		return nil
	}
	urlMap, err := p.file.MGetFileURL(ctx, fileKeys)
	if err != nil {
		return err
	}
	// 回填url
	for _, message := range messages {
		if message == nil || len(message.Parts) == 0 {
			continue
		}
		for _, part := range message.Parts {
			if part == nil || part.ImageURL == nil {
				continue
			}
			part.ImageURL.URL = urlMap[part.ImageURL.URI]
		}
	}
	return nil
}
