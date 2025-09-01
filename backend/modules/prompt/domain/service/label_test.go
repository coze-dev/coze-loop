// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0			expectedResult: nil,
package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestPromptServiceImpl_CreateLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		labelDO       *entity.PromptLabel
		presetLabels  []string
		configErr     error
		repoErr       error
		expectedError string
	}{
		{
			name: "valid label creation",
			labelDO: &entity.PromptLabel{
				SpaceID:  1,
				LabelKey: "valid_label",
			},
			presetLabels:  []string{"preset1", "preset2"},
			configErr:     nil,
			repoErr:       nil,
			expectedError: "",
		},
		{
			name: "invalid label key format - uppercase",
			labelDO: &entity.PromptLabel{
				SpaceID:  1,
				LabelKey: "Invalid_Label",
			},
			expectedError: "label key must contain only lowercase letters, digits, and underscores",
		},
		{
			name: "invalid label key format - special chars",
			labelDO: &entity.PromptLabel{
				SpaceID:  1,
				LabelKey: "label-with-dash",
			},
			expectedError: "label key must contain only lowercase letters, digits, and underscores",
		},
		{
			name: "conflict with preset label",
			labelDO: &entity.PromptLabel{
				SpaceID:  1,
				LabelKey: "preset1",
			},
			presetLabels:  []string{"preset1", "preset2"},
			configErr:     nil,
			expectedError: "label key conflicts with preset label: preset1",
		},
		{
			name: "config provider error",
			labelDO: &entity.PromptLabel{
				SpaceID:  1,
				LabelKey: "valid_label",
			},
			configErr:     assert.AnError,
			expectedError: "assert.AnError general error for testing",
		},
		{
			name: "repo creation error",
			labelDO: &entity.PromptLabel{
				SpaceID:  1,
				LabelKey: "valid_label",
			},
			presetLabels:  []string{"preset1", "preset2"},
			configErr:     nil,
			repoErr:       assert.AnError,
			expectedError: "assert.AnError general error for testing",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			// Setup mocks based on test case
			if tc.labelDO.LabelKey != "Invalid_Label" && tc.labelDO.LabelKey != "label-with-dash" {
				mockConfigProvider.EXPECT().ListPresetLabels().Return(tc.presetLabels, tc.configErr).Times(1)

				if tc.configErr == nil && tc.labelDO.LabelKey != "preset1" {
					mockLabelRepo.EXPECT().CreateLabel(ctx, tc.labelDO).Return(tc.repoErr).Times(1)
				}
			}

			err := service.CreateLabel(ctx, tc.labelDO)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPromptServiceImpl_ListLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		param                ListLabelParam
		presetLabels         []string
		configErr            error
		userLabels           []*entity.PromptLabel
		userNextToken        *int64
		repoErr              error
		checkUserLabels      []*entity.PromptLabel
		checkUserErr         error
		expectedLabels       []*entity.PromptLabel
		expectedNextToken    *int64
		expectedError        string
	}{
		{
			name: "first page - preset labels enough to fill page",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
				PageToken:    nil,
			},
			presetLabels: []string{"preset1", "preset2", "preset3"},
			configErr:    nil,
			checkUserLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
			},
			checkUserErr: nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(-3); return &token }(),
			expectedError:     "",
		},
		{
			name: "first page - preset labels exactly fill page, has user labels",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
				PageToken:    nil,
			},
			presetLabels: []string{"preset1", "preset2"},
			configErr:    nil,
			checkUserLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
			},
			checkUserErr: nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(100); return &token }(),
			expectedError:     "",
		},
		{
			name: "first page - preset labels exactly fill page, no user labels",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
				PageToken:    nil,
			},
			presetLabels:    []string{"preset1", "preset2"},
			configErr:       nil,
			checkUserLabels: []*entity.PromptLabel{},
			checkUserErr:    nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
			},
			expectedNextToken: nil,
			expectedError:     "",
		},
		{
			name: "first page - preset labels not enough, fill with user labels",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     3,
				PageToken:    nil,
			},
			presetLabels: []string{"preset1"},
			configErr:    nil,
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			userNextToken: func() *int64 { token := int64(102); return &token }(),
			repoErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(102); return &token }(),
			expectedError:     "",
		},
		{
			name: "first page - only preset labels, no user labels needed",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     3,
				PageToken:    nil,
			},
			presetLabels: []string{"preset1", "preset2"},
			configErr:    nil,
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
			},
			userNextToken: nil,
			repoErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
				{ID: 100, LabelKey: "user1", SpaceID: 1},
			},
			expectedNextToken: nil,
			expectedError:     "",
		},
		{
			name: "preset label page - middle page",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
				PageToken:    func() *int64 { token := int64(-3); return &token }(),
			},
			presetLabels: []string{"preset1", "preset2", "preset3", "preset4", "preset5"},
			configErr:    nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -3, LabelKey: "preset3", SpaceID: 1},
				{ID: -4, LabelKey: "preset4", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(-5); return &token }(),
			expectedError:     "",
		},
		{
			name: "preset label page - last page, page full, has user labels",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     1,
				PageToken:    func() *int64 { token := int64(-4); return &token }(),
			},
			presetLabels: []string{"preset1", "preset2", "preset3", "preset4"},
			configErr:    nil,
			checkUserLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
			},
			checkUserErr: nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -4, LabelKey: "preset4", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(100); return &token }(),
			expectedError:     "",
		},
		{
			name: "preset label page - last page, page not full, fill with user labels",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     3,
				PageToken:    func() *int64 { token := int64(-4); return &token }(),
			},
			presetLabels: []string{"preset1", "preset2", "preset3", "preset4"},
			configErr:    nil,
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			userNextToken: func() *int64 { token := int64(102); return &token }(),
			repoErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -4, LabelKey: "preset4", SpaceID: 1},
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(102); return &token }(),
			expectedError:     "",
		},
		{
			name: "preset label page - index out of range",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
				PageToken:    func() *int64 { token := int64(-10); return &token }(),
			},
			presetLabels:      []string{"preset1", "preset2"},
			configErr:         nil,
			expectedLabels:    []*entity.PromptLabel{},
			expectedNextToken: nil,
			expectedError:     "",
		},
		{
			name: "user label page",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
				PageToken:    func() *int64 { token := int64(100); return &token }(),
			},
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			userNextToken: func() *int64 { token := int64(102); return &token }(),
			repoErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(102); return &token }(),
			expectedError:     "",
		},
		{
			name: "with label key filter",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "test",
				PageSize:     3,
				PageToken:    nil,
			},
			presetLabels: []string{"test_preset", "other_preset", "preset_test"},
			configErr:    nil,
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "test_user", SpaceID: 1},
			},
			userNextToken: nil,
			repoErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "test_preset", SpaceID: 1},
				{ID: -2, LabelKey: "preset_test", SpaceID: 1},
				{ID: 100, LabelKey: "test_user", SpaceID: 1},
			},
			expectedNextToken: nil,
			expectedError:     "",
		},
		{
			name: "config provider error",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
				PageToken:    nil,
			},
			presetLabels:      nil,
			configErr:        assert.AnError,
			expectedLabels:    nil,
			expectedNextToken: nil,
			expectedError:     "assert.AnError general error for testing",
		},
		{
			name: "user label repo error",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
				PageToken:    func() *int64 { token := int64(100); return &token }(),
			},
			userLabels:        nil,
			userNextToken:     nil,
			repoErr:           assert.AnError,
			expectedLabels:    nil,
			expectedNextToken: nil,
			expectedError:     "assert.AnError general error for testing",
		},
		{
			name: "fill with user labels error",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     3,
				PageToken:    nil,
			},
			presetLabels:      []string{"preset1"},
			configErr:         nil,
			userLabels:        nil,
			userNextToken:     nil,
			repoErr:           assert.AnError,
			expectedLabels:    nil,
			expectedNextToken: nil,
			expectedError:     "assert.AnError general error for testing",
		},

	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			// Setup config provider mock for getting preset labels
			// ListLabel always calls getFilteredPresetLabels which calls configProvider.ListPresetLabels
			mockConfigProvider.EXPECT().ListPresetLabels().Return(tc.presetLabels, tc.configErr).Times(1)

			if tc.configErr == nil {
				// Setup mocks based on the test scenario
				switch {
								case tc.param.PageToken == nil:
					// First page scenario
					// For filtered preset labels, need to calculate actual filtered count
					filteredCount := len(tc.presetLabels)
					if tc.param.LabelKeyLike != "" {
						// Calculate filtered count
						filteredCount = 0
						for _, preset := range tc.presetLabels {
							if strings.Contains(preset, tc.param.LabelKeyLike) {
								filteredCount++
							}
						}
					}
					
					if filteredCount < tc.param.PageSize {
						// Need to fill with user labels
						mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
							SpaceID:      tc.param.SpaceID,
							LabelKeyLike: tc.param.LabelKeyLike,
							PageSize:     tc.param.PageSize - filteredCount,
							PageToken:    nil,
						}).Return(tc.userLabels, tc.userNextToken, tc.repoErr).Times(1)
					} else if filteredCount == tc.param.PageSize {
						// Check if user labels exist
						if tc.checkUserLabels != nil || tc.checkUserErr != nil {
							mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
								SpaceID:      tc.param.SpaceID,
								LabelKeyLike: tc.param.LabelKeyLike,
								PageSize:     1,
								PageToken:    nil,
							}).Return(tc.checkUserLabels, nil, tc.checkUserErr).Times(1)
						}
					}

				case *tc.param.PageToken < 0:
					// Preset label page scenario
					startIndex := int(-*tc.param.PageToken - 1)
					endIndex := startIndex + tc.param.PageSize
					if endIndex > len(tc.presetLabels) {
						endIndex = len(tc.presetLabels)
					}
					resultCount := endIndex - startIndex

					if startIndex < len(tc.presetLabels) {
						if endIndex >= len(tc.presetLabels) && resultCount < tc.param.PageSize {
							// Need to fill with user labels
							mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
								SpaceID:      tc.param.SpaceID,
								LabelKeyLike: tc.param.LabelKeyLike,
								PageSize:     tc.param.PageSize - resultCount,
								PageToken:    nil,
							}).Return(tc.userLabels, tc.userNextToken, tc.repoErr).Times(1)
						} else if endIndex >= len(tc.presetLabels) && resultCount == tc.param.PageSize {
							// Check if user labels exist
							if tc.checkUserLabels != nil || tc.checkUserErr != nil {
								mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
									SpaceID:      tc.param.SpaceID,
									LabelKeyLike: tc.param.LabelKeyLike,
									PageSize:     1,
									PageToken:    nil,
								}).Return(tc.checkUserLabels, nil, tc.checkUserErr).Times(1)
							}
						}
					}

				default:
					// User label page scenario
					mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
						SpaceID:      tc.param.SpaceID,
						LabelKeyLike: tc.param.LabelKeyLike,
						PageSize:     tc.param.PageSize,
						PageToken:    tc.param.PageToken,
					}).Return(tc.userLabels, tc.userNextToken, tc.repoErr).Times(1)
				}
			}

			labels, nextToken, err := service.ListLabel(ctx, tc.param)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, labels)
				assert.Nil(t, nextToken)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedLabels, labels)
				assert.Equal(t, tc.expectedNextToken, nextToken)
			}
		})
	}
}

func TestPromptServiceImpl_getFilteredPresetLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		labelKeyLike   string
		presetLabels   []string
		configErr      error
		expectedResult []string
		expectedError  string
	}{
		{
			name:         "no filter - return all",
			labelKeyLike: "",
			presetLabels: []string{"preset1", "preset2", "preset3"},
			configErr:    nil,
			expectedResult: []string{"preset1", "preset2", "preset3"},
			expectedError: "",
		},
		{
			name:         "with filter - partial match",
			labelKeyLike: "test",
			presetLabels: []string{"test_preset", "other_preset", "preset_test", "another"},
			configErr:    nil,
			expectedResult: []string{"test_preset", "preset_test"},
			expectedError: "",
		},
		{
			name:         "with filter - no match",
			labelKeyLike: "nonexistent",
			presetLabels: []string{"preset1", "preset2", "preset3"},
			configErr:    nil,
			expectedResult: nil,
			expectedError: "",
		},
		{
			name:         "empty preset labels",
			labelKeyLike: "",
			presetLabels: []string{},
			configErr:    nil,
			expectedResult: []string{},
			expectedError: "",
		},
		{
			name:           "config provider error",
			labelKeyLike:   "",
			presetLabels:   nil,
			configErr:      assert.AnError,
			expectedResult: nil,
			expectedError:  "assert.AnError general error for testing",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			mockConfigProvider.EXPECT().ListPresetLabels().Return(tc.presetLabels, tc.configErr).Times(1)

			result, err := service.getFilteredPresetLabels(tc.labelKeyLike)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func TestPromptServiceImpl_convertPresetLabelsToEntities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		presetLabels   []string
		spaceID        int64
		start          int
		end            int
		expectedResult []*entity.PromptLabel
	}{
		{
			name:         "normal conversion",
			presetLabels: []string{"preset1", "preset2", "preset3"},
			spaceID:      1,
			start:        0,
			end:          2,
			expectedResult: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
			},
		},
		{
			name:         "start from middle",
			presetLabels: []string{"preset1", "preset2", "preset3", "preset4"},
			spaceID:      2,
			start:        1,
			end:          3,
			expectedResult: []*entity.PromptLabel{
				{ID: -2, LabelKey: "preset2", SpaceID: 2},
				{ID: -3, LabelKey: "preset3", SpaceID: 2},
			},
		},
		{
			name:           "start >= end",
			presetLabels:   []string{"preset1", "preset2", "preset3"},
			spaceID:        1,
			start:          2,
			end:            2,
			expectedResult: nil,
		},
		{
			name:           "start >= len(presetLabels)",
			presetLabels:   []string{"preset1", "preset2"},
			spaceID:        1,
			start:          3,
			end:            5,
			expectedResult: nil,
		},
		{
			name:         "end > len(presetLabels)",
			presetLabels: []string{"preset1", "preset2"},
			spaceID:      1,
			start:        0,
			end:          5,
			expectedResult: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
			},
		},
		{
			name:           "empty preset labels",
			presetLabels:   []string{},
			spaceID:        1,
			start:          0,
			end:            2,
			expectedResult: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := &PromptServiceImpl{}

			result := service.convertPresetLabelsToEntities(tc.presetLabels, tc.spaceID, tc.start, tc.end)

			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestPromptServiceImpl_fillWithUserLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		param             ListLabelParam
		currentLabels     []*entity.PromptLabel
		userLabels        []*entity.PromptLabel
		userNextToken     *int64
		repoErr           error
		expectedLabels    []*entity.PromptLabel
		expectedNextToken *int64
		expectedError     string
	}{
		{
			name: "page already full",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
			},
			currentLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
			},
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
			},
			expectedNextToken: nil,
			expectedError:     "",
		},
		{
			name: "fill with user labels",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     3,
			},
			currentLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
			},
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			userNextToken: func() *int64 { token := int64(102); return &token }(),
			repoErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(102); return &token }(),
			expectedError:     "",
		},
		{
			name: "partial fill with user labels",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "test",
				PageSize:     4,
			},
			currentLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
			},
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
			},
			userNextToken: nil,
			repoErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: 100, LabelKey: "user1", SpaceID: 1},
			},
			expectedNextToken: nil,
			expectedError:     "",
		},
		{
			name: "repository error",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     3,
			},
			currentLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
			},
			userLabels:        nil,
			userNextToken:     nil,
			repoErr:           assert.AnError,
			expectedLabels:    nil,
			expectedNextToken: nil,
			expectedError:     "assert.AnError general error for testing",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			userLabelNeeded := tc.param.PageSize - len(tc.currentLabels)
			if userLabelNeeded > 0 {
				mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
					SpaceID:      tc.param.SpaceID,
					LabelKeyLike: tc.param.LabelKeyLike,
					PageSize:     userLabelNeeded,
					PageToken:    nil,
				}).Return(tc.userLabels, tc.userNextToken, tc.repoErr).Times(1)
			}

			labels, nextToken, err := service.fillWithUserLabels(ctx, tc.param, tc.currentLabels)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, labels)
				assert.Nil(t, nextToken)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedLabels, labels)
				assert.Equal(t, tc.expectedNextToken, nextToken)
			}
		})
	}
}

func TestPromptServiceImpl_checkUserLabelsExist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		param          ListLabelParam
		userLabels     []*entity.PromptLabel
		repoErr        error
		expectedResult *int64
		expectedError  string
	}{
		{
			name: "user labels exist",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
			},
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
			},
			repoErr:        nil,
			expectedResult: func() *int64 { id := int64(100); return &id }(),
			expectedError:  "",
		},
		{
			name: "no user labels",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "test",
			},
			userLabels:     []*entity.PromptLabel{},
			repoErr:        nil,
			expectedResult: nil,
			expectedError:  "",
		},
		{
			name: "repository error",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
			},
			userLabels:     nil,
			repoErr:        assert.AnError,
			expectedResult: nil,
			expectedError:  "assert.AnError general error for testing",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
				SpaceID:      tc.param.SpaceID,
				LabelKeyLike: tc.param.LabelKeyLike,
				PageSize:     1,
				PageToken:    nil,
			}).Return(tc.userLabels, nil, tc.repoErr).Times(1)

			result, err := service.checkUserLabelsExist(ctx, tc.param)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func TestPromptServiceImpl_handleFirstPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		param                ListLabelParam
		filteredPresetLabels []string
		userLabels           []*entity.PromptLabel
		userNextToken        *int64
		fillErr              error
		checkUserLabels      []*entity.PromptLabel
		checkErr             error
		expectedLabels       []*entity.PromptLabel
		expectedNextToken    *int64
		expectedError        string
	}{
		{
			name: "preset labels more than page size",
			param: ListLabelParam{
				SpaceID:  1,
				PageSize: 2,
			},
			filteredPresetLabels: []string{"preset1", "preset2", "preset3"},
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(-3); return &token }(),
			expectedError:     "",
		},
		{
			name: "preset labels equal page size, has user labels",
			param: ListLabelParam{
				SpaceID:  1,
				PageSize: 2,
			},
			filteredPresetLabels: []string{"preset1", "preset2"},
			checkUserLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
			},
			checkErr: nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(100); return &token }(),
			expectedError:     "",
		},
		{
			name: "preset labels equal page size, no user labels",
			param: ListLabelParam{
				SpaceID:  1,
				PageSize: 2,
			},
			filteredPresetLabels: []string{"preset1", "preset2"},
			checkUserLabels:      []*entity.PromptLabel{},
			checkErr:             nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: -2, LabelKey: "preset2", SpaceID: 1},
			},
			expectedNextToken: nil,
			expectedError:     "",
		},
		{
			name: "preset labels less than page size, fill with user labels",
			param: ListLabelParam{
				SpaceID:  1,
				PageSize: 3,
			},
			filteredPresetLabels: []string{"preset1"},
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			userNextToken: func() *int64 { token := int64(102); return &token }(),
			fillErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -1, LabelKey: "preset1", SpaceID: 1},
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(102); return &token }(),
			expectedError:     "",
		},
		{
			name: "no preset labels, fill with user labels",
			param: ListLabelParam{
				SpaceID:  1,
				PageSize: 2,
			},
			filteredPresetLabels: []string{},
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			userNextToken: nil,
			fillErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			expectedNextToken: nil,
			expectedError:     "",
		},
		{
			name: "fill with user labels error",
			param: ListLabelParam{
				SpaceID:  1,
				PageSize: 3,
			},
			filteredPresetLabels: []string{"preset1"},
			userLabels:           nil,
			userNextToken:        nil,
			fillErr:              assert.AnError,
			expectedLabels:       []*entity.PromptLabel{},
			expectedNextToken:    nil,
			expectedError:        "assert.AnError general error for testing",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			presetCount := len(tc.filteredPresetLabels)
			if presetCount >= tc.param.PageSize {
				if presetCount == tc.param.PageSize {
					// Need to check user labels exist
					mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
						SpaceID:      tc.param.SpaceID,
						LabelKeyLike: tc.param.LabelKeyLike,
						PageSize:     1,
						PageToken:    nil,
					}).Return(tc.checkUserLabels, nil, tc.checkErr).Times(1)
				}
			} else {
				// Need to fill with user labels
				userLabelNeeded := tc.param.PageSize - presetCount
				mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
					SpaceID:      tc.param.SpaceID,
					LabelKeyLike: tc.param.LabelKeyLike,
					PageSize:     userLabelNeeded,
					PageToken:    nil,
				}).Return(tc.userLabels, tc.userNextToken, tc.fillErr).Times(1)
			}

			labels, nextToken, err := service.handleFirstPage(ctx, tc.param, tc.filteredPresetLabels)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, labels)
				assert.Nil(t, nextToken)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedLabels, labels)
				assert.Equal(t, tc.expectedNextToken, nextToken)
			}
		})
	}
}

func TestPromptServiceImpl_handlePresetLabelPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		param                ListLabelParam
		filteredPresetLabels []string
		userLabels           []*entity.PromptLabel
		userNextToken        *int64
		fillErr              error
		checkUserLabels      []*entity.PromptLabel
		checkErr             error
		expectedLabels       []*entity.PromptLabel
		expectedNextToken    *int64
		expectedError        string
	}{
		{
			name: "middle page with more preset labels",
			param: ListLabelParam{
				SpaceID:   1,
				PageSize:  2,
				PageToken: func() *int64 { token := int64(-3); return &token }(),
			},
			filteredPresetLabels: []string{"preset1", "preset2", "preset3", "preset4", "preset5"},
			expectedLabels: []*entity.PromptLabel{
				{ID: -3, LabelKey: "preset3", SpaceID: 1},
				{ID: -4, LabelKey: "preset4", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(-5); return &token }(),
			expectedError:     "",
		},
		{
			name: "last preset page, page full, has user labels",
			param: ListLabelParam{
				SpaceID:   1,
				PageSize:  1,
				PageToken: func() *int64 { token := int64(-4); return &token }(),
			},
			filteredPresetLabels: []string{"preset1", "preset2", "preset3", "preset4"},
			checkUserLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
			},
			checkErr: nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -4, LabelKey: "preset4", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(100); return &token }(),
			expectedError:     "",
		},
		{
			name: "last preset page, page not full, fill with user labels",
			param: ListLabelParam{
				SpaceID:   1,
				PageSize:  3,
				PageToken: func() *int64 { token := int64(-4); return &token }(),
			},
			filteredPresetLabels: []string{"preset1", "preset2", "preset3", "preset4"},
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			userNextToken: func() *int64 { token := int64(102); return &token }(),
			fillErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: -4, LabelKey: "preset4", SpaceID: 1},
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(102); return &token }(),
			expectedError:     "",
		},
		{
			name: "page token out of range",
			param: ListLabelParam{
				SpaceID:   1,
				PageSize:  2,
				PageToken: func() *int64 { token := int64(-10); return &token }(),
			},
			filteredPresetLabels: []string{"preset1", "preset2"},
			expectedLabels:       []*entity.PromptLabel{},
			expectedNextToken:    nil,
			expectedError:        "",
		},
		{
			name: "fill with user labels error",
			param: ListLabelParam{
				SpaceID:   1,
				PageSize:  3,
				PageToken: func() *int64 { token := int64(-2); return &token }(),
			},
			filteredPresetLabels: []string{"preset1", "preset2"},
			userLabels:           nil,
			userNextToken:        nil,
			fillErr:              assert.AnError,
			expectedLabels:       []*entity.PromptLabel{},
			expectedNextToken:    nil,
			expectedError:        "assert.AnError general error for testing",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			startIndex := int(-*tc.param.PageToken - 1)
			presetCount := len(tc.filteredPresetLabels)
			endIndex := startIndex + tc.param.PageSize
			if endIndex > presetCount {
				endIndex = presetCount
			}

			if startIndex < presetCount {
				resultCount := endIndex - startIndex
				if endIndex >= presetCount && resultCount < tc.param.PageSize {
					// Need to fill with user labels
					userLabelNeeded := tc.param.PageSize - resultCount
					mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
						SpaceID:      tc.param.SpaceID,
						LabelKeyLike: tc.param.LabelKeyLike,
						PageSize:     userLabelNeeded,
						PageToken:    nil,
					}).Return(tc.userLabels, tc.userNextToken, tc.fillErr).Times(1)
				} else if endIndex >= presetCount && resultCount == tc.param.PageSize {
					// Check if user labels exist
					mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
						SpaceID:      tc.param.SpaceID,
						LabelKeyLike: tc.param.LabelKeyLike,
						PageSize:     1,
						PageToken:    nil,
					}).Return(tc.checkUserLabels, nil, tc.checkErr).Times(1)
				}
			}

			labels, nextToken, err := service.handlePresetLabelPage(ctx, tc.param, tc.filteredPresetLabels)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, labels)
				assert.Nil(t, nextToken)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedLabels, labels)
				assert.Equal(t, tc.expectedNextToken, nextToken)
			}
		})
	}
}

func TestPromptServiceImpl_handleUserLabelPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		param             ListLabelParam
		userLabels        []*entity.PromptLabel
		userNextToken     *int64
		repoErr           error
		expectedLabels    []*entity.PromptLabel
		expectedNextToken *int64
		expectedError     string
	}{
		{
			name: "successful user label page",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
				PageToken:    func() *int64 { token := int64(100); return &token }(),
			},
			userLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			userNextToken: func() *int64 { token := int64(102); return &token }(),
			repoErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: 100, LabelKey: "user1", SpaceID: 1},
				{ID: 101, LabelKey: "user2", SpaceID: 1},
			},
			expectedNextToken: func() *int64 { token := int64(102); return &token }(),
			expectedError:     "",
		},
		{
			name: "user label page with filter",
			param: ListLabelParam{
				SpaceID:      2,
				LabelKeyLike: "test",
				PageSize:     3,
				PageToken:    func() *int64 { token := int64(200); return &token }(),
			},
			userLabels: []*entity.PromptLabel{
				{ID: 200, LabelKey: "test_user", SpaceID: 2},
			},
			userNextToken: nil,
			repoErr:       nil,
			expectedLabels: []*entity.PromptLabel{
				{ID: 200, LabelKey: "test_user", SpaceID: 2},
			},
			expectedNextToken: nil,
			expectedError:     "",
		},
		{
			name: "repository error",
			param: ListLabelParam{
				SpaceID:      1,
				LabelKeyLike: "",
				PageSize:     2,
				PageToken:    func() *int64 { token := int64(100); return &token }(),
			},
			userLabels:        nil,
			userNextToken:     nil,
			repoErr:           assert.AnError,
			expectedLabels:    nil,
			expectedNextToken: nil,
			expectedError:     "assert.AnError general error for testing",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			mockLabelRepo.EXPECT().ListLabel(ctx, repo.ListLabelParam{
				SpaceID:      tc.param.SpaceID,
				LabelKeyLike: tc.param.LabelKeyLike,
				PageSize:     tc.param.PageSize,
				PageToken:    tc.param.PageToken,
			}).Return(tc.userLabels, tc.userNextToken, tc.repoErr).Times(1)

			labels, nextToken, err := service.handleUserLabelPage(ctx, tc.param)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, labels)
				assert.Nil(t, nextToken)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedLabels, labels)
				assert.Equal(t, tc.expectedNextToken, nextToken)
			}
		})
	}
}

func TestPromptServiceImpl_UpdateCommitLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		param          UpdateCommitLabelsParam
		promptDO       *entity.Prompt
		promptErr      error
		presetLabels   []string
		existingLabels []*entity.PromptLabel
		validateErr    error
		updateErr      error
		expectedError  string
	}{
		{
			name: "successful update with valid labels",
			param: UpdateCommitLabelsParam{
				PromptID:      1,
				CommitVersion: "v1.0",
				LabelKeys:     []string{"preset1", "user_label"},
				UpdatedBy:     "user1",
			},
			promptDO: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_prompt",
			},
			promptErr:      nil,
			presetLabels:   []string{"preset1", "preset2"},
			existingLabels: []*entity.PromptLabel{{LabelKey: "user_label", SpaceID: 100}},
			validateErr:    nil,
			updateErr:      nil,
			expectedError:  "",
		},
		{
			name: "prompt not found",
			param: UpdateCommitLabelsParam{
				PromptID:      1,
				CommitVersion: "v1.0",
				LabelKeys:     []string{"preset1"},
				UpdatedBy:     "user1",
			},
			promptDO:      nil,
			promptErr:     nil,
			expectedError: "prompt not found, promptID: 1",
		},
		{
			name: "prompt retrieval error",
			param: UpdateCommitLabelsParam{
				PromptID:      1,
				CommitVersion: "v1.0",
				LabelKeys:     []string{"preset1"},
				UpdatedBy:     "user1",
			},
			promptDO:      nil,
			promptErr:     assert.AnError,
			expectedError: "assert.AnError general error for testing",
		},
		{
			name: "label validation error",
			param: UpdateCommitLabelsParam{
				PromptID:      1,
				CommitVersion: "v1.0",
				LabelKeys:     []string{"nonexistent_label"},
				UpdatedBy:     "user1",
			},
			promptDO: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_prompt",
			},
			promptErr:     nil,
			presetLabels:  []string{"preset1", "preset2"},
			validateErr:   errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("label key not found: nonexistent_label")),
			expectedError: "label key not found: nonexistent_label",
		},
		{
			name: "update repository error",
			param: UpdateCommitLabelsParam{
				PromptID:      1,
				CommitVersion: "v1.0",
				LabelKeys:     []string{"preset1"},
				UpdatedBy:     "user1",
			},
			promptDO: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_prompt",
			},
			promptErr:      nil,
			presetLabels:   []string{"preset1", "preset2"},
			existingLabels: []*entity.PromptLabel{},
			validateErr:    nil,
			updateErr:      assert.AnError,
			expectedError:  "assert.AnError general error for testing",
		},
		{
			name: "empty label keys",
			param: UpdateCommitLabelsParam{
				PromptID:      1,
				CommitVersion: "v1.0",
				LabelKeys:     []string{},
				UpdatedBy:     "user1",
			},
			promptDO: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_prompt",
			},
			promptErr:     nil,
			updateErr:     nil,
			expectedError: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			// Setup prompt retrieval mock
			mockManageRepo.EXPECT().GetPrompt(ctx, repo.GetPromptParam{
				PromptID: tc.param.PromptID,
			}).Return(tc.promptDO, tc.promptErr).Times(1)

			if tc.promptErr == nil && tc.promptDO != nil {
				// Setup validation mocks if labels are provided
				if len(tc.param.LabelKeys) > 0 {
					mockConfigProvider.EXPECT().ListPresetLabels().Return(tc.presetLabels, nil).Times(1)

					// Filter user labels
					var userLabels []string
					presetMap := make(map[string]bool)
					for _, preset := range tc.presetLabels {
						presetMap[preset] = true
					}
					for _, label := range tc.param.LabelKeys {
						if !presetMap[label] {
							userLabels = append(userLabels, label)
						}
					}

					if len(userLabels) > 0 {
						mockLabelRepo.EXPECT().BatchGetLabel(ctx, tc.promptDO.SpaceID, userLabels).Return(tc.existingLabels, nil).Times(1)
					}
				}

				// Setup update mock if validation passes
				if tc.validateErr == nil {
					mockLabelRepo.EXPECT().UpdateCommitLabels(ctx, repo.UpdateCommitLabelsParam{
						SpaceID:       tc.promptDO.SpaceID,
						PromptID:      tc.param.PromptID,
						PromptKey:     tc.promptDO.PromptKey,
						LabelKeys:     tc.param.LabelKeys,
						CommitVersion: tc.param.CommitVersion,
						UpdatedBy:     tc.param.UpdatedBy,
					}).Return(tc.updateErr).Times(1)
				}
			}

			err := service.UpdateCommitLabels(ctx, tc.param)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPromptServiceImpl_BatchGetCommitLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		promptID       int64
		commitVersions []string
		repoResult     map[string][]*entity.PromptLabel
		repoErr        error
		expectedResult map[string][]string
		expectedError  string
	}{
		{
			name:           "successful batch get",
			promptID:       1,
			commitVersions: []string{"v1.0", "v2.0"},
			repoResult: map[string][]*entity.PromptLabel{
				"v1.0": {
					{LabelKey: "label1"},
					{LabelKey: "label2"},
				},
				"v2.0": {
					{LabelKey: "label3"},
				},
			},
			repoErr: nil,
			expectedResult: map[string][]string{
				"v1.0": {"label1", "label2"},
				"v2.0": {"label3"},
			},
			expectedError: "",
		},
		{
			name:           "empty result",
			promptID:       1,
			commitVersions: []string{"v1.0"},
			repoResult:     map[string][]*entity.PromptLabel{},
			repoErr:        nil,
			expectedResult: map[string][]string{},
			expectedError:  "",
		},
		{
			name:           "repository error",
			promptID:       1,
			commitVersions: []string{"v1.0"},
			repoResult:     nil,
			repoErr:        assert.AnError,
			expectedResult: nil,
			expectedError:  "assert.AnError general error for testing",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			mockLabelRepo.EXPECT().GetCommitLabels(ctx, tc.promptID, tc.commitVersions).Return(tc.repoResult, tc.repoErr).Times(1)

			result, err := service.BatchGetCommitLabels(ctx, tc.promptID, tc.commitVersions)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}

func TestPromptServiceImpl_ValidateLabelsExist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		spaceID        int64
		labelKeys      []string
		presetLabels   []string
		configErr      error
		existingLabels []*entity.PromptLabel
		repoErr        error
		expectedError  string
	}{
		{
			name:          "all labels exist - preset only",
			spaceID:       1,
			labelKeys:     []string{"preset1", "preset2"},
			presetLabels:  []string{"preset1", "preset2", "preset3"},
			configErr:     nil,
			expectedError: "",
		},
		{
			name:         "all labels exist - mixed",
			spaceID:      1,
			labelKeys:    []string{"preset1", "user_label1"},
			presetLabels: []string{"preset1", "preset2"},
			configErr:    nil,
			existingLabels: []*entity.PromptLabel{
				{LabelKey: "user_label1", SpaceID: 1},
			},
			repoErr:       nil,
			expectedError: "",
		},
		{
			name:           "user label not found",
			spaceID:        1,
			labelKeys:      []string{"preset1", "nonexistent_label"},
			presetLabels:   []string{"preset1", "preset2"},
			configErr:      nil,
			existingLabels: []*entity.PromptLabel{},
			repoErr:        nil,
			expectedError:  "label key not found: nonexistent_label",
		},
		{
			name:          "config provider error",
			spaceID:       1,
			labelKeys:     []string{"preset1"},
			presetLabels:  nil,
			configErr:     assert.AnError,
			expectedError: "assert.AnError general error for testing",
		},
		{
			name:          "repository error",
			spaceID:       1,
			labelKeys:     []string{"preset1", "user_label1"},
			presetLabels:  []string{"preset1", "preset2"},
			configErr:     nil,
			repoErr:       assert.AnError,
			expectedError: "assert.AnError general error for testing",
		},
		{
			name:          "empty label keys",
			spaceID:       1,
			labelKeys:     []string{},
			presetLabels:  []string{"preset1", "preset2"},
			configErr:     nil,
			expectedError: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			mockConfigProvider.EXPECT().ListPresetLabels().Return(tc.presetLabels, tc.configErr).Times(1)

			if tc.configErr == nil && len(tc.labelKeys) > 0 {
				// Filter user labels
				presetMap := make(map[string]bool)
				for _, preset := range tc.presetLabels {
					presetMap[preset] = true
				}
				var userLabels []string
				for _, label := range tc.labelKeys {
					if !presetMap[label] {
						userLabels = append(userLabels, label)
					}
				}

				if len(userLabels) > 0 {
					mockLabelRepo.EXPECT().BatchGetLabel(ctx, tc.spaceID, userLabels).Return(tc.existingLabels, tc.repoErr).Times(1)
				}
			}

			err := service.ValidateLabelsExist(ctx, tc.spaceID, tc.labelKeys)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPromptServiceImpl_isValidLabelKey(t *testing.T) {
	t.Parallel()

	service := &PromptServiceImpl{}

	tests := []struct {
		name     string
		labelKey string
		expected bool
	}{
		{
			name:     "valid - lowercase letters only",
			labelKey: "validlabel",
			expected: true,
		},
		{
			name:     "valid - with numbers",
			labelKey: "label123",
			expected: true,
		},
		{
			name:     "valid - with underscores",
			labelKey: "valid_label_key",
			expected: true,
		},
		{
			name:     "valid - numbers and underscores",
			labelKey: "label_123_test",
			expected: true,
		},
		{
			name:     "invalid - uppercase letters",
			labelKey: "InvalidLabel",
			expected: false,
		},
		{
			name:     "invalid - special characters",
			labelKey: "label-with-dash",
			expected: false,
		},
		{
			name:     "invalid - spaces",
			labelKey: "label with space",
			expected: false,
		},
		{
			name:     "invalid - empty string",
			labelKey: "",
			expected: false,
		},
		{
			name:     "invalid - dots",
			labelKey: "label.with.dot",
			expected: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := service.isValidLabelKey(tc.labelKey)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPromptServiceImpl_BatchGetLabelMappingPromptVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		queries        []PromptLabelQuery
		repoResult     map[repo.PromptLabelQuery]string
		repoErr        error
		expectedResult map[PromptLabelQuery]string
		expectedError  string
	}{
		{
			name: "successful batch get mapping",
			queries: []PromptLabelQuery{
				{PromptID: 1, LabelKey: "label1"},
				{PromptID: 2, LabelKey: "label2"},
			},
			repoResult: map[repo.PromptLabelQuery]string{
				{PromptID: 1, LabelKey: "label1"}: "v1.0",
				{PromptID: 2, LabelKey: "label2"}: "v2.0",
			},
			repoErr: nil,
			expectedResult: map[PromptLabelQuery]string{
				{PromptID: 1, LabelKey: "label1"}: "v1.0",
				{PromptID: 2, LabelKey: "label2"}: "v2.0",
			},
			expectedError: "",
		},
		{
			name:           "empty queries",
			queries:        []PromptLabelQuery{},
			expectedResult: map[PromptLabelQuery]string{},
			expectedError:  "",
		},
		{
			name: "repository error",
			queries: []PromptLabelQuery{
				{PromptID: 1, LabelKey: "label1"},
			},
			repoResult:     nil,
			repoErr:        assert.AnError,
			expectedResult: nil,
			expectedError:  "assert.AnError general error for testing",
		},
		{
			name: "partial results",
			queries: []PromptLabelQuery{
				{PromptID: 1, LabelKey: "label1"},
				{PromptID: 2, LabelKey: "label2"},
			},
			repoResult: map[repo.PromptLabelQuery]string{
				{PromptID: 1, LabelKey: "label1"}: "v1.0",
			},
			repoErr: nil,
			expectedResult: map[PromptLabelQuery]string{
				{PromptID: 1, LabelKey: "label1"}: "v1.0",
			},
			expectedError: "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
			mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
			mockConfigProvider := mocks.NewMockIConfigProvider(ctrl)

			service := &PromptServiceImpl{
				labelRepo:      mockLabelRepo,
				manageRepo:     mockManageRepo,
				configProvider: mockConfigProvider,
			}

			ctx := context.Background()

			if len(tc.queries) > 0 {
				var repoQueries []repo.PromptLabelQuery
				for _, query := range tc.queries {
					repoQueries = append(repoQueries, repo.PromptLabelQuery{
						PromptID: query.PromptID,
						LabelKey: query.LabelKey,
					})
				}
				mockLabelRepo.EXPECT().BatchGetPromptVersionByLabel(ctx, repoQueries).Return(tc.repoResult, tc.repoErr).Times(1)
			}

			result, err := service.BatchGetLabelMappingPromptVersion(ctx, tc.queries)

			if tc.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}
		})
	}
}