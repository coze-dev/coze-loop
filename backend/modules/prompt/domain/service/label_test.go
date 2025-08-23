package service

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
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
