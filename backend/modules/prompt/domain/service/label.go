// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func (p *PromptServiceImpl) CreateLabel(ctx context.Context, labelDO *entity.PromptLabel) error {
	// 校验label格式：只包含英文、下划线、数字，必须为小写
	if !p.isValidLabelKey(labelDO.LabelKey) {
		return errorx.NewByCode(prompterr.CommonInvalidParamCode,
			errorx.WithExtraMsg("label key must contain only lowercase letters, digits, and underscores"))
	}

	// 检查是否与预置标签重复
	presetLabels, err := p.configProvider.ListPresetLabels()
	if err != nil {
		return err
	}
	for _, preset := range presetLabels {
		if preset == labelDO.LabelKey {
			return errorx.NewByCode(prompterr.CommonResourceDuplicatedCode,
				errorx.WithExtraMsg(fmt.Sprintf("label key conflicts with preset label: %s", labelDO.LabelKey)))
		}
	}

	err = p.labelRepo.CreateLabel(ctx, labelDO)
	if err != nil {
		return err
	}
	return nil
}

func (p *PromptServiceImpl) ListLabel(ctx context.Context, param ListLabelParam) ([]*entity.PromptLabel, *int64, error) {
	// 获取并过滤预置标签
	filteredPresetLabels, err := p.getFilteredPresetLabels(param.LabelKeyLike)
	if err != nil {
		return nil, nil, err
	}

	// 根据PageToken类型分发处理
	switch {
	case param.PageToken == nil:
		return p.handleFirstPage(ctx, param, filteredPresetLabels)
	case *param.PageToken < 0:
		return p.handlePresetLabelPage(ctx, param, filteredPresetLabels)
	default:
		return p.handleUserLabelPage(ctx, param)
	}
}

// getFilteredPresetLabels 获取并过滤预置标签
func (p *PromptServiceImpl) getFilteredPresetLabels(labelKeyLike string) ([]string, error) {
	presetLabels, err := p.configProvider.ListPresetLabels()
	if err != nil {
		return nil, err
	}

	if labelKeyLike == "" {
		return presetLabels, nil
	}

	var filtered []string
	for _, preset := range presetLabels {
		if strings.Contains(preset, labelKeyLike) {
			filtered = append(filtered, preset)
		}
	}
	return filtered, nil
}

// 将预置标签字符串转换为实体对象，为预置标签分配虚拟ID
func (p *PromptServiceImpl) convertPresetLabelsToEntities(presetLabels []string, spaceID int64, start, end int) []*entity.PromptLabel {
	var result []*entity.PromptLabel
	for i := start; i < end && i < len(presetLabels); i++ {
		result = append(result, &entity.PromptLabel{
			ID:       int64(-(i + 1)), // 使用负数作为预置标签的虚拟ID，索引i对应ID为-(i+1)
			LabelKey: presetLabels[i],
			SpaceID:  spaceID,
		})
	}
	return result
}

// fillWithUserLabels 用用户标签填充剩余的页面空间
func (p *PromptServiceImpl) fillWithUserLabels(ctx context.Context, param ListLabelParam, currentLabels []*entity.PromptLabel) ([]*entity.PromptLabel, *int64, error) {
	userLabelNeeded := param.PageSize - len(currentLabels)
	if userLabelNeeded <= 0 {
		return currentLabels, nil, nil
	}

	userLabels, userNextToken, err := p.labelRepo.ListLabel(ctx, repo.ListLabelParam{
		SpaceID:      param.SpaceID,
		LabelKeyLike: param.LabelKeyLike,
		PageSize:     userLabelNeeded,
		PageToken:    nil,
	})
	if err != nil {
		return nil, nil, err
	}

	result := append(currentLabels, userLabels...)
	return result, userNextToken, nil
}

// checkUserLabelsExist 检查是否存在用户标签（用于判断下一页token）
func (p *PromptServiceImpl) checkUserLabelsExist(ctx context.Context, param ListLabelParam) (*int64, error) {
	userLabels, _, err := p.labelRepo.ListLabel(ctx, repo.ListLabelParam{
		SpaceID:      param.SpaceID,
		LabelKeyLike: param.LabelKeyLike,
		PageSize:     1,
		PageToken:    nil,
	})
	if err != nil {
		return nil, err
	}
	if len(userLabels) > 0 {
		return &userLabels[0].ID, nil
	}
	return nil, nil
}

// handleFirstPage 处理第一页请求（PageToken为nil）
func (p *PromptServiceImpl) handleFirstPage(ctx context.Context, param ListLabelParam, filteredPresetLabels []string) ([]*entity.PromptLabel, *int64, error) {
	presetCount := len(filteredPresetLabels)

	if presetCount >= param.PageSize {
		// 预置标签足够填满一页
		resultLabels := p.convertPresetLabelsToEntities(filteredPresetLabels, param.SpaceID, 0, param.PageSize)

		var nextPageToken *int64
		if presetCount > param.PageSize {
			// 还有更多预置标签，下一页从第param.PageSize个预置标签开始
			token := int64(-(param.PageSize + 1)) // 下一页起始位置的虚拟ID
			nextPageToken = &token
		} else {
			// 预置标签刚好填满一页，检查是否有用户标签
			nextPageToken, _ = p.checkUserLabelsExist(ctx, param)
		}

		return resultLabels, nextPageToken, nil
	}

	// 预置标签不够一页，添加所有预置标签并补充用户标签
	presetLabels := p.convertPresetLabelsToEntities(filteredPresetLabels, param.SpaceID, 0, presetCount)
	return p.fillWithUserLabels(ctx, param, presetLabels)
}

// handlePresetLabelPage 处理预置标签分页请求（PageToken < 0）
func (p *PromptServiceImpl) handlePresetLabelPage(ctx context.Context, param ListLabelParam, filteredPresetLabels []string) ([]*entity.PromptLabel, *int64, error) {
	// PageToken为负数时，表示预置标签的虚拟ID，转换为实际的数组索引
	startIndex := int(-*param.PageToken - 1) // -1对应索引0，-2对应索引1，以此类推
	presetCount := len(filteredPresetLabels)

	if startIndex >= presetCount {
		// 预置标签已用完，返回空结果
		return []*entity.PromptLabel{}, nil, nil
	}

	// 计算当前页的结束索引
	endIndex := startIndex + param.PageSize
	if endIndex > presetCount {
		endIndex = presetCount
	}

	resultLabels := p.convertPresetLabelsToEntities(filteredPresetLabels, param.SpaceID, startIndex, endIndex)

	var nextPageToken *int64
	if endIndex < presetCount {
		// 还有更多预置标签，下一页从endIndex位置开始
		token := int64(-(endIndex + 1)) // 下一页起始位置的虚拟ID
		nextPageToken = &token
	} else if len(resultLabels) < param.PageSize {
		// 预置标签用完且当前页未满，补充用户标签
		return p.fillWithUserLabels(ctx, param, resultLabels)
	} else {
		// 预置标签用完且当前页已满，检查是否有用户标签
		nextPageToken, _ = p.checkUserLabelsExist(ctx, param)
	}

	return resultLabels, nextPageToken, nil
}

// handleUserLabelPage 处理用户标签分页请求（PageToken > 0）
func (p *PromptServiceImpl) handleUserLabelPage(ctx context.Context, param ListLabelParam) ([]*entity.PromptLabel, *int64, error) {
	userLabels, userNextToken, err := p.labelRepo.ListLabel(ctx, repo.ListLabelParam{
		SpaceID:      param.SpaceID,
		LabelKeyLike: param.LabelKeyLike,
		PageSize:     param.PageSize,
		PageToken:    param.PageToken,
	})
	if err != nil {
		return nil, nil, err
	}

	return userLabels, userNextToken, nil
}

func (p *PromptServiceImpl) UpdateCommitLabels(ctx context.Context, param UpdateCommitLabelsParam) error {
	// 先获取prompt信息，以获得SpaceID和PromptKey
	promptDO, err := p.manageRepo.GetPrompt(ctx, repo.GetPromptParam{
		PromptID: param.PromptID,
	})
	if err != nil {
		return err
	}
	if promptDO == nil {
		return errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg(fmt.Sprintf("prompt not found, promptID: %d", param.PromptID)))
	}

	spaceID := promptDO.SpaceID
	promptKey := promptDO.PromptKey

	// 验证所有标签是否存在
	if len(param.LabelKeys) > 0 {
		err := p.ValidateLabelsExist(ctx, spaceID, param.LabelKeys)
		if err != nil {
			return err
		}
	}

	// 调用repo层的UpdateCommitLabels方法，在事务中完成所有操作
	err = p.labelRepo.UpdateCommitLabels(ctx, repo.UpdateCommitLabelsParam{
		SpaceID:       spaceID,
		PromptID:      param.PromptID,
		PromptKey:     promptKey,
		LabelKeys:     param.LabelKeys,
		CommitVersion: param.CommitVersion,
		UpdatedBy:     param.UpdatedBy,
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *PromptServiceImpl) BatchGetCommitLabels(ctx context.Context, promptID int64, commitVersions []string) (map[string][]string, error) {
	versionLabelsMap, err := p.labelRepo.GetCommitLabels(ctx, promptID, commitVersions)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string)
	for version, labels := range versionLabelsMap {
		labelKeys := make([]string, len(labels))
		for i, label := range labels {
			labelKeys[i] = label.LabelKey
		}
		result[version] = labelKeys
	}

	return result, nil
}

func (p *PromptServiceImpl) ValidateLabelsExist(ctx context.Context, spaceID int64, labelKeys []string) error {
	// 获取预置标签
	presetLabels, err := p.configProvider.ListPresetLabels()
	if err != nil {
		return err
	}
	presetLabelMap := make(map[string]bool)
	for _, preset := range presetLabels {
		presetLabelMap[preset] = true
	}

	// 分离预置标签和用户自定义标签
	var userLabelKeys []string
	for _, labelKey := range labelKeys {
		if !presetLabelMap[labelKey] {
			userLabelKeys = append(userLabelKeys, labelKey)
		}
	}

	// 验证用户自定义标签是否存在
	if len(userLabelKeys) > 0 {
		existingLabels, err := p.labelRepo.BatchGetLabel(ctx, spaceID, userLabelKeys)
		if err != nil {
			return err
		}

		existingLabelMap := make(map[string]bool)
		for _, label := range existingLabels {
			existingLabelMap[label.LabelKey] = true
		}

		for _, labelKey := range userLabelKeys {
			if !existingLabelMap[labelKey] {
				return errorx.NewByCode(prompterr.ResourceNotFoundCode,
					errorx.WithExtraMsg(fmt.Sprintf("label key not found: %s", labelKey)))
			}
		}
	}

	return nil
}

func (p *PromptServiceImpl) isValidLabelKey(key string) bool {
	// 校验规则：只包含小写字母、数字和下划线
	matched, _ := regexp.MatchString("^[a-z0-9_]+$", key)
	return matched
}

func (p *PromptServiceImpl) BatchGetLabelMappingPromptVersion(ctx context.Context, queries []PromptLabelQuery) (map[PromptLabelQuery]string, error) {
	if len(queries) == 0 {
		return make(map[PromptLabelQuery]string), nil
	}

	// 构建查询参数，查询每个label对应该prompt的版本
	var repoQueries []repo.PromptLabelQuery
	for _, query := range queries {
		repoQueries = append(repoQueries, repo.PromptLabelQuery{
			PromptID: query.PromptID,
			LabelKey: query.LabelKey,
		})
	}

	// 查询label和prompt的版本映射关系
	mappings, err := p.labelRepo.BatchGetPromptVersionByLabel(ctx, repoQueries)
	if err != nil {
		return nil, err
	}

	// 构建label key到prompt version的映射
	promptVersionMapping := make(map[PromptLabelQuery]string)
	for query, version := range mappings {
		promptVersionMapping[PromptLabelQuery{PromptID: query.PromptID, LabelKey: query.LabelKey}] = version
	}

	return promptVersionMapping, nil
}
