// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
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

func (p *PromptServiceImpl) MCompleteMultiModalFileURL(ctx context.Context, messages []*entity.Message, variableVals []*entity.VariableVal) error {
	var fileKeys []string
	for _, message := range messages {
		if message == nil || len(message.Parts) == 0 {
			continue
		}
		for _, part := range message.Parts {
			if part == nil {
				continue
			}
			if part.ImageURL != nil && part.ImageURL.URI != "" {
				fileKeys = append(fileKeys, part.ImageURL.URI)
			}
			if part.VideoURL != nil && part.VideoURL.URI != "" {
				fileKeys = append(fileKeys, part.VideoURL.URI)
			}
		}
	}
	for _, val := range variableVals {
		if val == nil || len(val.MultiPartValues) == 0 {
			continue
		}
		for _, part := range val.MultiPartValues {
			if part == nil {
				continue
			}
			if part.ImageURL != nil && part.ImageURL.URI != "" {
				fileKeys = append(fileKeys, part.ImageURL.URI)
			}
			if part.VideoURL != nil && part.VideoURL.URI != "" {
				fileKeys = append(fileKeys, part.VideoURL.URI)
			}
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
			if part == nil {
				continue
			}
			if part.ImageURL != nil {
				part.ImageURL.URL = urlMap[part.ImageURL.URI]
			}
			if part.VideoURL != nil {
				part.VideoURL.URL = urlMap[part.VideoURL.URI]
			}
		}
	}
	for _, val := range variableVals {
		if val == nil || len(val.MultiPartValues) == 0 {
			continue
		}
		for _, part := range val.MultiPartValues {
			if part == nil {
				continue
			}
			if part.ImageURL != nil && part.ImageURL.URI != "" {
				part.ImageURL.URL = urlMap[part.ImageURL.URI]
			}
			if part.VideoURL != nil && part.VideoURL.URI != "" {
				part.VideoURL.URL = urlMap[part.VideoURL.URI]
			}
		}
	}
	return nil
}

// MConvertBase64DataURLToFileURI converts base64 files to file URIs by uploading them
func (p *PromptServiceImpl) MConvertBase64DataURLToFileURI(ctx context.Context, messages []*entity.Message, workspaceID int64) error {
	for _, message := range messages {
		if message == nil || len(message.Parts) == 0 {
			continue
		}

		for _, part := range message.Parts {
			if part == nil || part.ImageURL == nil {
				continue
			}
			// Check if the URL is a base64 data URL
			url := part.ImageURL.URL
			if url == "" || !strings.HasPrefix(url, "data:") {
				continue
			}

			// Parse the data URL to extract mime type and base64 data
			// Format: data:<mime_type>;base64,<base64_data>
			parts := strings.SplitN(url, ",", 2)
			if len(parts) != 2 {
				logs.CtxWarn(ctx, "invalid data URL format: %s", url)
				continue
			}

			// Extract mime type from the first part
			headerParts := strings.SplitN(parts[0], ";", 2)
			if len(headerParts) != 2 {
				logs.CtxWarn(ctx, "invalid data URL header: %s", parts[0])
				continue
			}
			mimeType := strings.TrimPrefix(headerParts[0], "data:")
			if mimeType == "" {
				logs.CtxWarn(ctx, "missing mime type in data URL")
				continue
			}

			// Decode base64 data
			decodedData, err := base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				logs.CtxError(ctx, "failed to decode base64 file: %v", err)
				continue
			}

			// Upload the file
			fileKey, err := p.file.UploadFileForServer(ctx, mimeType, decodedData, workspaceID)
			if err != nil {
				logs.CtxError(ctx, "failed to upload file: %v", err)
				return err
			}

			// Replace the base64 URL with the file URI
			part.ImageURL.URI = fileKey
			part.ImageURL.URL = "" // Clear the URL, it will be filled later by MGetFileURL if needed
		}
	}

	return nil
}

// messageContainsBase64File checks if messages contain base64 files
func (p *PromptServiceImpl) messageContainsBase64File(messages []*entity.Message) bool {
	for _, message := range messages {
		if message == nil || len(message.Parts) == 0 {
			continue
		}
		for _, part := range message.Parts {
			if part == nil || part.ImageURL == nil {
				continue
			}
			// Check if the URL is a base64 data URL (format: data:<mime_type>;base64,<data>)
			url := part.ImageURL.URL
			if url != "" && strings.HasPrefix(url, "data:") {
				return true
			}
		}
	}
	return false
}

// MConvertBase64DataURLToFileURL converts base64 files to download URLs
func (p *PromptServiceImpl) MConvertBase64DataURLToFileURL(ctx context.Context, messages []*entity.Message, workspaceID int64) error {
	// Fast path: skip processing if no base64 files present
	if !p.messageContainsBase64File(messages) {
		return nil
	}

	// Convert base64 files to file URIs
	if err := p.MConvertBase64DataURLToFileURI(ctx, messages, workspaceID); err != nil {
		return err
	}

	// Convert file URIs to download URLs
	if err := p.MCompleteMultiModalFileURL(ctx, messages, nil); err != nil {
		return err
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
