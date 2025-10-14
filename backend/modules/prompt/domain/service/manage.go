// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/bytedance/gg/gmap"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
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

// CreatePrompt creates a prompt with optional snippet validation
func (p *PromptServiceImpl) CreatePrompt(ctx context.Context, promptDO *entity.Prompt) (promptID int64, err error) {
	if promptDO == nil {
		return 0, errorx.New("promptDO is empty")
	}

	// Validate basic prompt information
	if promptDO.SpaceID <= 0 {
		return 0, errorx.New("spaceID is invalid: %d", promptDO.SpaceID)
	}
	if promptDO.PromptKey == "" {
		return 0, errorx.New("promptKey is empty")
	}
	if promptDO.PromptBasic == nil {
		return 0, errorx.New("promptBasic is empty")
	}

	err = p.parseAndValidateSnippets(ctx, promptDO)
	if err != nil {
		return 0, err
	}

	// Create the prompt through repository
	promptID, err = p.manageRepo.CreatePrompt(ctx, promptDO)
	if err != nil {
		return 0, err
	}

	return promptID, nil
}

func (p *PromptServiceImpl) parseAndValidateSnippets(ctx context.Context, promptDO *entity.Prompt) error {
	// Check if prompt has snippets based on the flag
	hasSnippets := false
	if promptDetail := promptDO.GetPromptDetail(); promptDetail != nil && promptDetail.PromptTemplate != nil {
		hasSnippets = promptDetail.PromptTemplate.HasSnippets
	}
	if !hasSnippets {
		return nil
	}

	// Only parse and validate snippets if hasSnippets is true
	if promptDetail := promptDO.GetPromptDetail(); promptDetail != nil && promptDetail.PromptTemplate != nil {
		var allContent string
		for _, message := range promptDetail.PromptTemplate.Messages {
			if ptr.From(message.Content) != "" {
				allContent += ptr.From(message.Content)
			}
			for _, part := range message.Parts {
				if ptr.From(part.Text) != "" {
					allContent += ptr.From(part.Text)
				}
			}
		}

		var snippetRefs []*SnippetReference
		var err error
		if allContent != "" {
			snippetRefs, err = p.snippetParser.ParseReferences(allContent)
			if err != nil {
				return errorx.WrapByCode(err, prompterr.CommonInvalidParamCode,
					errorx.WithExtraMsg("failed to parse snippet references"))
			}
		}

		// Validate that snippets were actually found
		if len(snippetRefs) == 0 {
			return errorx.NewByCode(prompterr.CommonInvalidParamCode,
				errorx.WithExtraMsg("has_snippets is true but no snippet references found in content"))
		}

		// Validate snippet references exist and are of correct type with valid versions
		queriesMap := make(map[repo.GetPromptParam]bool)
		for _, ref := range snippetRefs {
			queriesMap[repo.GetPromptParam{
				PromptID:      ref.PromptID,
				WithCommit:    true,
				CommitVersion: ref.CommitVersion,
			}] = true
		}

		snippetPrompts, err := p.manageRepo.MGetPrompt(ctx, gmap.Keys(queriesMap), repo.WithPromptCacheEnable())
		if err != nil {
			return errorx.WrapByCode(err, prompterr.CommonInvalidParamCode,
				errorx.WithExtraMsg("failed to get snippet prompts"))
		}
		// fill snippets
		promptDetail.PromptTemplate.Snippets = gmap.Values(snippetPrompts)

		// Validate each snippet reference using map access
		for _, ref := range snippetRefs {
			key := repo.GetPromptParam{
				PromptID:      ref.PromptID,
				WithCommit:    true,
				CommitVersion: ref.CommitVersion,
			}

			prompt, exists := snippetPrompts[key]
			if !exists || prompt == nil {
				return errorx.NewByCode(prompterr.ResourceNotFoundCode,
					errorx.WithExtraMsg(fmt.Sprintf("snippet prompt %d with version %s not found", ref.PromptID, ref.CommitVersion)))
			}

			// Check if prompt is a snippet type
			if prompt.PromptBasic == nil || prompt.PromptBasic.PromptType != entity.PromptTypeSnippet {
				return errorx.NewByCode(prompterr.CommonInvalidParamCode,
					errorx.WithExtraMsg(fmt.Sprintf("prompt %d is not a snippet type", ref.PromptID)))
			}
		}
	}
	return nil
}

// SaveDraft saves a draft with snippet validation and relationship management
func (p *PromptServiceImpl) SaveDraft(ctx context.Context, promptDO *entity.Prompt) (*entity.DraftInfo, error) {
	if promptDO == nil || promptDO.PromptDraft == nil {
		return nil, errorx.New("promptDO or promptDO.PromptDraft is empty")
	}

	// Parse and validate snippets in the draft content
	err := p.parseAndValidateSnippets(ctx, promptDO)
	if err != nil {
		return nil, err
	}

	// Save the draft through repository (which will handle snippet relationships)
	draftInfo, err := p.manageRepo.SaveDraft(ctx, promptDO)
	if err != nil {
		return nil, err
	}

	return draftInfo, nil
}

// ExpandSnippets expands all snippet references in the prompt's messages
func (p *PromptServiceImpl) ExpandSnippets(ctx context.Context, promptDO *entity.Prompt) error {
	maxDepth := 2
	return p.doExpandSnippets(ctx, promptDO, maxDepth)
}

func (p *PromptServiceImpl) doExpandSnippets(ctx context.Context, promptDO *entity.Prompt, maxDepth int) error {
	if promptDO == nil {
		return errorx.New("promptDO is empty")
	}

	// Get the prompt detail
	promptDetail := promptDO.GetPromptDetail()
	if promptDetail == nil || promptDetail.PromptTemplate == nil {
		return nil // No template to expand
	}

	// Check if prompt has snippets
	if !promptDetail.PromptTemplate.HasSnippets {
		return nil // No snippets to expand
	}

	// Validate max depth to prevent infinite recursion
	if maxDepth <= 0 {
		return errorx.New("max recursion depth reached")
	}
	// First, parse and validate snippets to populate the Snippets field
	// This will call MGetPrompt once to get all snippet data
	err := p.parseAndValidateSnippets(ctx, promptDO)
	if err != nil {
		return err
	}

	for _, snippet := range promptDetail.PromptTemplate.Snippets {
		if snippet.GetPromptDetail().PromptTemplate == nil || !snippet.GetPromptDetail().PromptTemplate.HasSnippets {
			continue
		}
		err = p.doExpandSnippets(ctx, snippet, maxDepth-1)
		if err != nil {
			return err
		}
	}

	// Build maps for quick lookup of snippet content and variable definitions
	snippetContentMap := make(map[string]string)
	snippetVariableMap := make(map[string][]*entity.VariableDef)
	for _, snippet := range promptDetail.PromptTemplate.Snippets {
		if snippet == nil || snippet.PromptBasic == nil {
			continue
		}

		snippetDetail := snippet.GetPromptDetail()
		if snippetDetail == nil || snippetDetail.PromptTemplate == nil {
			continue
		}

		// Build lookup key: "promptID_version"
		key := fmt.Sprintf("%d_%s", snippet.ID, snippet.GetVersion())
		// Get snippet content from the first message
		if len(snippetDetail.PromptTemplate.Messages) > 0 {
			snippetContent := ptr.From(snippetDetail.PromptTemplate.Messages[0].Content)
			snippetContentMap[key] = snippetContent
		}

		// Collect variable definitions from snippet
		if len(snippetDetail.PromptTemplate.VariableDefs) > 0 {
			snippetVariableMap[key] = snippetDetail.PromptTemplate.VariableDefs
		}
	}

	// Now expand all snippet references in messages using the pre-built content map and merge variable definitions
	var allMergedVariableDefs []*entity.VariableDef
	existingVarKeys := make(map[string]bool) // Track existing variable keys to avoid duplicates

	// First, collect existing variable definitions from the main prompt
	for _, varDef := range promptDetail.PromptTemplate.VariableDefs {
		if varDef != nil {
			allMergedVariableDefs = append(allMergedVariableDefs, varDef)
			existingVarKeys[varDef.Key] = true
		}
	}

	for _, message := range promptDetail.PromptTemplate.Messages {
		if message == nil {
			continue
		}

		// Expand content if it exists
		if message.Content != nil && *message.Content != "" {
			expandedContent, mergedVars, err := p.expandWithSnippetMap(ctx, *message.Content, snippetContentMap, snippetVariableMap)
			if err != nil {
				return err
			}
			message.Content = &expandedContent
			// Add merged variable definitions (avoid duplicates)
			for _, varDef := range mergedVars {
				if varDef != nil && !existingVarKeys[varDef.Key] {
					allMergedVariableDefs = append(allMergedVariableDefs, varDef)
					existingVarKeys[varDef.Key] = true
				}
			}
		}

		// Expand text in parts
		for _, part := range message.Parts {
			if part == nil || part.Text == nil || *part.Text == "" {
				continue
			}
			expandedText, mergedVars, err := p.expandWithSnippetMap(ctx, *part.Text, snippetContentMap, snippetVariableMap)
			if err != nil {
				return err
			}
			part.Text = &expandedText
			// Add merged variable definitions (avoid duplicates)
			for _, varDef := range mergedVars {
				if varDef != nil && !existingVarKeys[varDef.Key] {
					allMergedVariableDefs = append(allMergedVariableDefs, varDef)
					existingVarKeys[varDef.Key] = true
				}
			}
		}
	}

	// Update the prompt template with merged variable definitions
	promptDetail.PromptTemplate.VariableDefs = allMergedVariableDefs

	return nil
}

// expandWithSnippetMap expands snippet references using a pre-built content map and returns expanded content with merged variable definitions
func (p *PromptServiceImpl) expandWithSnippetMap(ctx context.Context, content string, snippetContentMap map[string]string, snippetVariableMap map[string][]*entity.VariableDef) (string, []*entity.VariableDef, error) {
	// Parse snippet references from content
	snippetRefs, err := p.snippetParser.ParseReferences(content)
	if err != nil {
		return "", nil, err
	}

	// If no references found, return original content and empty variable definitions
	if len(snippetRefs) == 0 {
		return content, nil, nil
	}

	// Replace each reference with expanded content from the map and collect variable definitions
	expandedContent := content
	var mergedVariableDefs []*entity.VariableDef
	processedSnippets := make(map[string]bool) // Track processed snippets to avoid duplicates

	for _, ref := range snippetRefs {
		// Build lookup key: "promptID_version"
		key := fmt.Sprintf("%d_%s", ref.PromptID, ref.CommitVersion)
		expandedSnippetContent, exists := snippetContentMap[key]
		if !exists {
			return "", nil, errorx.NewByCode(prompterr.ResourceNotFoundCode,
				errorx.WithExtraMsg(fmt.Sprintf("snippet content for prompt %d with version %s not found in cache", ref.PromptID, ref.CommitVersion)))
		}

		// Collect variable definitions from this snippet if not already processed
		if !processedSnippets[key] {
			if variableDefs, varExists := snippetVariableMap[key]; varExists && len(variableDefs) > 0 {
				mergedVariableDefs = append(mergedVariableDefs, variableDefs...)
			}
			processedSnippets[key] = true
		}

		// Replace the reference with expanded content
		refString := p.snippetParser.SerializeReference(ref)
		expandedContent = strings.ReplaceAll(expandedContent, refString, expandedSnippetContent)
	}

	return expandedContent, mergedVariableDefs, nil
}
