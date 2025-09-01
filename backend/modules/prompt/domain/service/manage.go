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

func (p *PromptServiceImpl) MCompleteMultiModalFileURL(ctx context.Context, messages []*entity.Message, variableVals []*entity.VariableVal) error {
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
	for _, val := range variableVals {
		if val == nil || len(val.MultiPartValues) == 0 {
			continue
		}
		for _, part := range val.MultiPartValues {
			if part == nil || part.ImageURL == nil || part.ImageURL.URI == "" {
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
	for _, val := range variableVals {
		if val == nil || len(val.MultiPartValues) == 0 {
			continue
		}
		for _, part := range val.MultiPartValues {
			if part == nil || part.ImageURL == nil || part.ImageURL.URI == "" {
				continue
			}
			part.ImageURL.URL = urlMap[part.ImageURL.URI]
		}
	}
	return nil
}

// MParseCommitVersion 统一解析提交版本，支持version和label两种方式
func (p *PromptServiceImpl) MParseCommitVersion(ctx context.Context, spaceID int64, params []PromptQueryParam) (promptKeyCommitVersionMap map[PromptQueryParam]string, err error) {
	promptKeyCommitVersionMap = make(map[PromptQueryParam]string)
	if len(params) == 0 {
		return promptKeyCommitVersionMap, nil
	}

	// 分类处理：分别处理version查询和label查询
	var latestVersionPromptKeys []string
	var labelParams []PromptQueryParam

	// 先为所有参数创建映射关系，并分类收集查询条件
	for _, param := range params {
		if param.Label != "" && param.Version == "" {
			// 使用label查询，优先级低于version
			labelParams = append(labelParams, param)
		} else {
			// 使用version查询，如果version为空，需要获取最新版本
			if param.Version == "" {
				latestVersionPromptKeys = append(latestVersionPromptKeys, param.PromptKey)
			}
			// 先用原始版本号占位
			promptKeyCommitVersionMap[param] = param.Version
		}
	}

	// 处理version查询中需要获取最新版本的情况
	if len(latestVersionPromptKeys) > 0 {
		basics, err := p.manageRepo.MGetPromptBasicByPromptKey(ctx, spaceID, latestVersionPromptKeys, repo.WithPromptBasicCacheEnable())
		if err != nil {
			return nil, err
		}
		for _, basic := range basics {
			if basic != nil && basic.PromptBasic != nil {
				latestCommitVersion := basic.PromptBasic.LatestVersion
				if latestCommitVersion == "" {
					return nil, errorx.NewByCode(prompterr.PromptUncommittedCode,
						errorx.WithExtraMsg(fmt.Sprintf("prompt key: %s", basic.PromptKey)),
						errorx.WithExtra(map[string]string{"prompt_key": basic.PromptKey}))
				}
				// 更新对应参数的版本号
				for _, param := range params {
					if param.PromptKey == basic.PromptKey && param.Version == "" && param.Label == "" {
						promptKeyCommitVersionMap[param] = latestCommitVersion
						break
					}
				}
			}
		}
	}

	// 处理label查询
	if len(labelParams) > 0 {
		// 构建查询参数，直接使用传入的 promptID
		promptIDLabelQueries := make([]repo.PromptLabelQuery, 0, len(labelParams))
		for _, param := range labelParams {
			promptIDLabelQueries = append(promptIDLabelQueries, repo.PromptLabelQuery{
				PromptID: param.PromptID,
				LabelKey: param.Label,
			})
		}

		if len(promptIDLabelQueries) > 0 {
			// 调用repo层获取数据，启用缓存
			mappings, err := p.labelRepo.BatchGetPromptVersionByLabel(ctx, promptIDLabelQueries, repo.WithLabelMappingCacheEnable())
			if err != nil {
				return nil, err
			}

			// 建立映射关系
			for _, param := range labelParams {
				version := mappings[repo.PromptLabelQuery{
					PromptID: param.PromptID,
					LabelKey: param.Label,
				}]
				if version == "" {
					return nil, errorx.NewByCode(prompterr.PromptLabelUnAssociatedCode,
						errorx.WithExtraMsg(fmt.Sprintf("prompt key: %s, label: %s", param.PromptKey, param.Label)),
						errorx.WithExtra(map[string]string{"prompt_key": param.PromptKey, "label": param.Label}))
				}
				promptKeyCommitVersionMap[param] = version
			}
		}
	}

	return promptKeyCommitVersionMap, nil
}
