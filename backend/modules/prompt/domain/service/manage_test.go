// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/mem"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestPromptServiceImpl_MCompleteMultiModalFileURL(t *testing.T) {
	type fields struct {
		idgen            idgen.IIDGenerator
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		manageRepo       repo.IManageRepo
		labelRepo        repo.ILabelRepo
		configProvider   conf.IConfigProvider
		llm              rpc.ILLMProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx          context.Context
		messages     []*entity.Message
		variableVals []*entity.VariableVal
	}
	uri2URLMap := map[string]string{
		"test-image-1": "https://example.com/image1.jpg",
		"test-image-2": "https://example.com/image2.jpg",
		"test-image-3": "https://example.com/image3.jpg",
		"test-video-1": "https://example.com/video1.mp4",
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name: "message without parts",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role:    entity.RoleUser,
						Content: ptr.Of("Hello"),
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "message with nil image URL",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "single message with single image success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockFile := mocks.NewMockIFileProvider(ctrl)
				mockFile.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(uri2URLMap, nil)
				return fields{
					file: mockFile,
				}
			},
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URI: "test-image-1",
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "multiple messages with multiple images success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockFile := mocks.NewMockIFileProvider(ctrl)
				mockFile.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(uri2URLMap, nil)
				return fields{
					file: mockFile,
				}
			},
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URI: "test-image-1",
								},
							},
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URI: "test-image-2",
								},
							},
						},
					},
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URI: "test-image-3",
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "video urls filled for messages and variable values",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockFile := mocks.NewMockIFileProvider(ctrl)
				mockFile.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(uri2URLMap, nil)
				return fields{
					file: mockFile,
				}
			},
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeVideoURL,
								VideoURL: &entity.VideoURL{
									URI: "test-video-1",
								},
							},
						},
					},
				},
				variableVals: []*entity.VariableVal{
					{
						Key: "video-multi",
						MultiPartValues: []*entity.ContentPart{
							{
								Type: entity.ContentTypeVideoURL,
								VideoURL: &entity.VideoURL{
									URI: "test-video-1",
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "variableVals with nil MultiPartValues",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:      context.Background(),
				messages: nil,
				variableVals: []*entity.VariableVal{
					{
						Key:             "multivar1",
						MultiPartValues: nil,
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "variableVals with empty MultiPartValues",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:      context.Background(),
				messages: nil,
				variableVals: []*entity.VariableVal{
					{
						Key:             "multivar1",
						MultiPartValues: []*entity.ContentPart{},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "variableVals with nil values",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:          context.Background(),
				messages:     nil,
				variableVals: []*entity.VariableVal{nil},
			},
			wantErr: nil,
		},
		{
			name: "variableVals with parts containing nil ImageURL",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:      context.Background(),
				messages: nil,
				variableVals: []*entity.VariableVal{
					{
						Key: "multivar1",
						MultiPartValues: []*entity.ContentPart{
							{
								Type:     entity.ContentTypeImageURL,
								ImageURL: nil,
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "variableVals with parts containing nil parts",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:      context.Background(),
				messages: nil,
				variableVals: []*entity.VariableVal{
					{
						Key: "multivar1",
						MultiPartValues: []*entity.ContentPart{
							nil,
							{
								Type: entity.ContentTypeText,
								Text: ptr.Of("some text"),
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "empty variableVals",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:          context.Background(),
				messages:     nil,
				variableVals: []*entity.VariableVal{},
			},
			wantErr: nil,
		},
		{
			name: "nil variableVals",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:          context.Background(),
				messages:     nil,
				variableVals: nil,
			},
			wantErr: nil,
		},
		{
			name: "file.MGetFileURL error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockFile := mocks.NewMockIFileProvider(ctrl)
				mockFile.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(nil, errorx.New("file service error"))
				return fields{
					file: mockFile,
				}
			},
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URI: "test-image-1",
								},
							},
						},
					},
				},
			},
			wantErr: errorx.New("file service error"),
		},
		{
			name: "variableVals with images success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockFile := mocks.NewMockIFileProvider(ctrl)
				mockFile.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(uri2URLMap, nil)
				return fields{
					file: mockFile,
				}
			},
			args: args{
				ctx:      context.Background(),
				messages: nil,
				variableVals: []*entity.VariableVal{
					{
						Key: "multivar1",
						MultiPartValues: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URI: "test-image-1",
								},
							},
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URI: "test-image-2",
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "messages and variableVals both with images",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockFile := mocks.NewMockIFileProvider(ctrl)
				mockFile.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(uri2URLMap, nil)
				return fields{
					file: mockFile,
				}
			},
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URI: "test-image-1",
								},
							},
						},
					},
				},
				variableVals: []*entity.VariableVal{
					{
						Key: "multivar1",
						MultiPartValues: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URI: "test-image-2",
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "variableVals with empty URI",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:      context.Background(),
				messages: nil,
				variableVals: []*entity.VariableVal{
					{
						Key: "multivar1",
						MultiPartValues: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URI: "", // 空URI应该被跳过
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "nil messages and nil variableVals",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:          context.Background(),
				messages:     nil,
				variableVals: nil,
			},
			wantErr: nil,
		},
		{
			name: "messages with nil message",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					nil,
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeText,
								Text: ptr.Of("some text"),
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "messages with nil parts",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							nil,
							{
								Type: entity.ContentTypeText,
								Text: ptr.Of("some text"),
							},
						},
					},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			p := &PromptServiceImpl{
				idgen:            ttFields.idgen,
				debugLogRepo:     ttFields.debugLogRepo,
				debugContextRepo: ttFields.debugContextRepo,
				manageRepo:       ttFields.manageRepo,
				labelRepo:        ttFields.labelRepo,
				configProvider:   ttFields.configProvider,
				llm:              ttFields.llm,
				file:             ttFields.file,
			}

			var originMessages []*entity.Message
			err := mem.DeepCopy(tt.args.messages, &originMessages)
			assert.Nil(t, err)
			err = p.MCompleteMultiModalFileURL(tt.args.ctx, tt.args.messages, tt.args.variableVals)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				// 验证messages中的URL是否正确填充
				for _, message := range tt.args.messages {
					if message == nil || len(message.Parts) == 0 {
						continue
					}
					for _, part := range message.Parts {
						if part == nil {
							continue
						}
						if part.ImageURL != nil && part.ImageURL.URI != "" {
							assert.Equal(t, uri2URLMap[part.ImageURL.URI], part.ImageURL.URL)
							part.ImageURL.URL = ""
						}
						if part.VideoURL != nil && part.VideoURL.URI != "" {
							assert.Equal(t, uri2URLMap[part.VideoURL.URI], part.VideoURL.URL)
							part.VideoURL.URL = ""
						}
					}
				}
				// 验证variableVals中的URL是否正确填充
				for _, val := range tt.args.variableVals {
					if val == nil || len(val.MultiPartValues) == 0 {
						continue
					}
					for _, part := range val.MultiPartValues {
						if part == nil {
							continue
						}
						if part.ImageURL != nil && part.ImageURL.URI != "" {
							assert.Equal(t, uri2URLMap[part.ImageURL.URI], part.ImageURL.URL)
							part.ImageURL.URL = ""
						}
						if part.VideoURL != nil && part.VideoURL.URI != "" {
							assert.Equal(t, uri2URLMap[part.VideoURL.URI], part.VideoURL.URL)
							part.VideoURL.URL = ""
						}
					}
				}
				assert.Equal(t, originMessages, tt.args.messages)
			}
		})
	}
}

func TestPromptServiceImpl_MGetPromptIDs(t *testing.T) {
	type fields struct {
		idgen            idgen.IIDGenerator
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		manageRepo       repo.IManageRepo
		labelRepo        repo.ILabelRepo
		configProvider   conf.IConfigProvider
		llm              rpc.ILLMProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx        context.Context
		spaceID    int64
		promptKeys []string
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         map[string]int64
		wantErr      error
	}{
		{
			name: "empty prompt keys",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    123,
				promptKeys: []string{},
			},
			want:    map[string]int64{},
			wantErr: nil,
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt1", "test_prompt2"}),
					gomock.Any(),
				).Return([]*entity.Prompt{
					{
						ID:        1,
						PromptKey: "test_prompt1",
					},
					{
						ID:        2,
						PromptKey: "test_prompt2",
					},
				}, nil)
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    123,
				promptKeys: []string{"test_prompt1", "test_prompt2"},
			},
			want: map[string]int64{
				"test_prompt1": 1,
				"test_prompt2": 2,
			},
			wantErr: nil,
		},
		{
			name: "prompt key not found with enhanced error info",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt1", "test_prompt2"}),
					gomock.Any(),
				).Return([]*entity.Prompt{
					{
						ID:        1,
						PromptKey: "test_prompt1",
					},
				}, nil)
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    123,
				promptKeys: []string{"test_prompt1", "test_prompt2"},
			},
			want:    nil,
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("prompt key: test_prompt2 not found"), errorx.WithExtra(map[string]string{"prompt_key": "test_prompt2"})),
		},
		{
			name: "database error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt1"}),
					gomock.Any(),
				).Return(nil, errorx.New("database error"))
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    123,
				promptKeys: []string{"test_prompt1"},
			},
			want:    nil,
			wantErr: errorx.New("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			p := &PromptServiceImpl{
				idgen:            ttFields.idgen,
				debugLogRepo:     ttFields.debugLogRepo,
				debugContextRepo: ttFields.debugContextRepo,
				manageRepo:       ttFields.manageRepo,
				labelRepo:        ttFields.labelRepo,
				configProvider:   ttFields.configProvider,
				llm:              ttFields.llm,
				file:             ttFields.file,
			}

			got, err := p.MGetPromptIDs(tt.args.ctx, tt.args.spaceID, tt.args.promptKeys)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptServiceImpl_MParseCommitVersion(t *testing.T) {
	type fields struct {
		idgen            idgen.IIDGenerator
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		manageRepo       repo.IManageRepo
		labelRepo        repo.ILabelRepo
		configProvider   conf.IConfigProvider
		llm              rpc.ILLMProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx     context.Context
		spaceID int64
		params  []PromptQueryParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         map[PromptQueryParam]string
		wantErr      error
	}{
		{
			name: "empty params",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params:  []PromptQueryParam{},
			},
			want:    map[PromptQueryParam]string{},
			wantErr: nil,
		},
		{
			name: "nil params",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params:  nil,
			},
			want:    map[PromptQueryParam]string{},
			wantErr: nil,
		},
		{
			name: "pure version query with specific version",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "v1.0.0",
						Label:     "",
					},
					{
						PromptID:  2,
						PromptKey: "test_prompt2",
						Version:   "v2.0.0",
						Label:     "",
					},
				},
			},
			want: map[PromptQueryParam]string{
				{
					PromptID:  1,
					PromptKey: "test_prompt1",
					Version:   "v1.0.0",
					Label:     "",
				}: "v1.0.0",
				{
					PromptID:  2,
					PromptKey: "test_prompt2",
					Version:   "v2.0.0",
					Label:     "",
				}: "v2.0.0",
			},
			wantErr: nil,
		},
		{
			name: "pure version query with empty version (get latest)",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt1", "test_prompt2"}),
					gomock.Any(),
				).Return([]*entity.Prompt{
					{
						ID:        1,
						PromptKey: "test_prompt1",
						PromptBasic: &entity.PromptBasic{
							LatestVersion: "v1.2.0",
						},
					},
					{
						ID:        2,
						PromptKey: "test_prompt2",
						PromptBasic: &entity.PromptBasic{
							LatestVersion: "v2.1.0",
						},
					},
				}, nil)
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "",
						Label:     "",
					},
					{
						PromptID:  2,
						PromptKey: "test_prompt2",
						Version:   "",
						Label:     "",
					},
				},
			},
			want: map[PromptQueryParam]string{
				{
					PromptID:  1,
					PromptKey: "test_prompt1",
					Version:   "",
					Label:     "",
				}: "v1.2.0",
				{
					PromptID:  2,
					PromptKey: "test_prompt2",
					Version:   "",
					Label:     "",
				}: "v2.1.0",
			},
			wantErr: nil,
		},
		{
			name: "get latest version but prompt uncommitted",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt1"}),
					gomock.Any(),
				).Return([]*entity.Prompt{
					{
						ID:        1,
						PromptKey: "test_prompt1",
						PromptBasic: &entity.PromptBasic{
							LatestVersion: "", // 空版本表示未提交
						},
					},
				}, nil)
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "",
						Label:     "",
					},
				},
			},
			want:    nil,
			wantErr: errorx.NewByCode(prompterr.PromptUncommittedCode, errorx.WithExtraMsg("prompt key: test_prompt1"), errorx.WithExtra(map[string]string{"prompt_key": "test_prompt1"})),
		},
		{
			name: "get latest version with manageRepo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt1"}),
					gomock.Any(),
				).Return(nil, errorx.New("database error"))
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "",
						Label:     "",
					},
				},
			},
			want:    nil,
			wantErr: errorx.New("database error"),
		},
		{
			name: "pure label query success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
				mockLabelRepo.EXPECT().BatchGetPromptVersionByLabel(
					gomock.Any(),
					gomock.Eq([]repo.PromptLabelQuery{
						{PromptID: 1, LabelKey: "stable"},
						{PromptID: 2, LabelKey: "beta"},
					}),
					gomock.Any(),
				).Return(map[repo.PromptLabelQuery]string{
					{PromptID: 1, LabelKey: "stable"}: "v1.0.0",
					{PromptID: 2, LabelKey: "beta"}:   "v2.0.0-beta",
				}, nil)
				return fields{
					labelRepo: mockLabelRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "",
						Label:     "stable",
					},
					{
						PromptID:  2,
						PromptKey: "test_prompt2",
						Version:   "",
						Label:     "beta",
					},
				},
			},
			want: map[PromptQueryParam]string{
				{
					PromptID:  1,
					PromptKey: "test_prompt1",
					Version:   "",
					Label:     "stable",
				}: "v1.0.0",
				{
					PromptID:  2,
					PromptKey: "test_prompt2",
					Version:   "",
					Label:     "beta",
				}: "v2.0.0-beta",
			},
			wantErr: nil,
		},
		{
			name: "label query with label not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
				mockLabelRepo.EXPECT().BatchGetPromptVersionByLabel(
					gomock.Any(),
					gomock.Eq([]repo.PromptLabelQuery{
						{PromptID: 1, LabelKey: "nonexistent"},
					}),
					gomock.Any(),
				).Return(map[repo.PromptLabelQuery]string{
					{PromptID: 1, LabelKey: "nonexistent"}: "", // 空字符串表示未找到
				}, nil)
				return fields{
					labelRepo: mockLabelRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "",
						Label:     "nonexistent",
					},
				},
			},
			want:    nil,
			wantErr: errorx.NewByCode(prompterr.PromptLabelUnAssociatedCode, errorx.WithExtraMsg("prompt key: test_prompt1, label: nonexistent"), errorx.WithExtra(map[string]string{"prompt_key": "test_prompt1", "label": "nonexistent"})),
		},
		{
			name: "label query with labelRepo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)
				mockLabelRepo.EXPECT().BatchGetPromptVersionByLabel(
					gomock.Any(),
					gomock.Eq([]repo.PromptLabelQuery{
						{PromptID: 1, LabelKey: "stable"},
					}),
					gomock.Any(),
				).Return(nil, errorx.New("label repo error"))
				return fields{
					labelRepo: mockLabelRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "",
						Label:     "stable",
					},
				},
			},
			want:    nil,
			wantErr: errorx.New("label repo error"),
		},
		{
			name: "mixed query: version and label",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockLabelRepo := repomocks.NewMockILabelRepo(ctrl)

				// 对于需要获取最新版本的prompt
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt2"}),
					gomock.Any(),
				).Return([]*entity.Prompt{
					{
						ID:        2,
						PromptKey: "test_prompt2",
						PromptBasic: &entity.PromptBasic{
							LatestVersion: "v2.1.0",
						},
					},
				}, nil)

				// 对于label查询
				mockLabelRepo.EXPECT().BatchGetPromptVersionByLabel(
					gomock.Any(),
					gomock.Eq([]repo.PromptLabelQuery{
						{PromptID: 3, LabelKey: "stable"},
					}),
					gomock.Any(),
				).Return(map[repo.PromptLabelQuery]string{
					{PromptID: 3, LabelKey: "stable"}: "v3.0.0",
				}, nil)

				return fields{
					manageRepo: mockManageRepo,
					labelRepo:  mockLabelRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "v1.0.0", // 指定版本
						Label:     "",
					},
					{
						PromptID:  2,
						PromptKey: "test_prompt2",
						Version:   "", // 获取最新版本
						Label:     "",
					},
					{
						PromptID:  3,
						PromptKey: "test_prompt3",
						Version:   "", // label查询
						Label:     "stable",
					},
				},
			},
			want: map[PromptQueryParam]string{
				{
					PromptID:  1,
					PromptKey: "test_prompt1",
					Version:   "v1.0.0",
					Label:     "",
				}: "v1.0.0",
				{
					PromptID:  2,
					PromptKey: "test_prompt2",
					Version:   "",
					Label:     "",
				}: "v2.1.0",
				{
					PromptID:  3,
					PromptKey: "test_prompt3",
					Version:   "",
					Label:     "stable",
				}: "v3.0.0",
			},
			wantErr: nil,
		},
		{
			name: "version has priority over label",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "v1.0.0", // version优先于label
						Label:     "stable",
					},
				},
			},
			want: map[PromptQueryParam]string{
				{
					PromptID:  1,
					PromptKey: "test_prompt1",
					Version:   "v1.0.0",
					Label:     "stable",
				}: "v1.0.0",
			},
			wantErr: nil,
		},
		{
			name: "prompt basic is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt1"}),
					gomock.Any(),
				).Return([]*entity.Prompt{
					{
						ID:          1,
						PromptKey:   "test_prompt1",
						PromptBasic: nil, // PromptBasic为nil
					},
				}, nil)
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "",
						Label:     "",
					},
				},
			},
			want: map[PromptQueryParam]string{
				{
					PromptID:  1,
					PromptKey: "test_prompt1",
					Version:   "",
					Label:     "",
				}: "",
			},
			wantErr: nil,
		},
		{
			name: "prompt entity is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt1"}),
					gomock.Any(),
				).Return([]*entity.Prompt{
					nil, // 整个entity为nil
				}, nil)
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				params: []PromptQueryParam{
					{
						PromptID:  1,
						PromptKey: "test_prompt1",
						Version:   "",
						Label:     "",
					},
				},
			},
			want: map[PromptQueryParam]string{
				{
					PromptID:  1,
					PromptKey: "test_prompt1",
					Version:   "",
					Label:     "",
				}: "",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			p := &PromptServiceImpl{
				idgen:            ttFields.idgen,
				debugLogRepo:     ttFields.debugLogRepo,
				debugContextRepo: ttFields.debugContextRepo,
				manageRepo:       ttFields.manageRepo,
				labelRepo:        ttFields.labelRepo,
				configProvider:   ttFields.configProvider,
				llm:              ttFields.llm,
				file:             ttFields.file,
			}

			got, err := p.MParseCommitVersion(tt.args.ctx, tt.args.spaceID, tt.args.params)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestPromptServiceImpl_messageContainsBase64File(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("hello"))
	dataURL := "data:image/png;base64," + encoded

	tests := []struct {
		name     string
		messages []*entity.Message
		want     bool
	}{
		{
			name:     "nil messages returns false",
			messages: nil,
			want:     false,
		},
		{
			name: "message without parts returns false",
			messages: []*entity.Message{
				{Role: entity.RoleUser},
			},
			want: false,
		},
		{
			name: "message without data url returns false",
			messages: []*entity.Message{
				{
					Role: entity.RoleUser,
					Parts: []*entity.ContentPart{
						{
							Type: entity.ContentTypeImageURL,
							ImageURL: &entity.ImageURL{
								URL: "https://example.com/image.png",
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "contains base64 returns true",
			messages: []*entity.Message{
				{
					Role: entity.RoleUser,
					Parts: []*entity.ContentPart{
						{
							Type: entity.ContentTypeImageURL,
							ImageURL: &entity.ImageURL{
								URL: dataURL,
							},
						},
					},
				},
			},
			want: true,
		},
	}

	p := &PromptServiceImpl{}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, p.messageContainsBase64File(tt.messages))
		})
	}
}

func TestPromptServiceImpl_MConvertBase64ToFileURI(t *testing.T) {
	type args struct {
		ctx         context.Context
		messages    []*entity.Message
		workspaceID int64
	}

	decoded := []byte("hello world")
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(decoded)

	tests := []struct {
		name         string
		args         args
		setupMock    func(mock *mocks.MockIFileProvider)
		wantErr      error
		validateFunc func(t *testing.T, messages []*entity.Message)
	}{
		{
			name: "successfully converts base64 image",
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URL: dataURL,
								},
							},
						},
					},
				},
				workspaceID: 101,
			},
			setupMock: func(mock *mocks.MockIFileProvider) {
				mock.EXPECT().
					UploadFileForServer(gomock.Any(), "image/png", gomock.Eq(decoded), int64(101)).
					Return("workspace/101/file.png", nil)
			},
			wantErr: nil,
			validateFunc: func(t *testing.T, messages []*entity.Message) {
				part := messages[0].Parts[0]
				assert.Equal(t, "workspace/101/file.png", part.ImageURL.URI)
				assert.Equal(t, "", part.ImageURL.URL)
			},
		},
		{
			name: "invalid data url skipped without error",
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URL: "data:image/png;base64",
								},
							},
						},
					},
				},
				workspaceID: 1,
			},
			setupMock: func(mock *mocks.MockIFileProvider) {
				mock.EXPECT().UploadFileForServer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: nil,
			validateFunc: func(t *testing.T, messages []*entity.Message) {
				part := messages[0].Parts[0]
				assert.Equal(t, "", part.ImageURL.URI)
				assert.Equal(t, "data:image/png;base64", part.ImageURL.URL)
			},
		},
		{
			name: "upload error returns error",
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URL: dataURL,
								},
							},
						},
					},
				},
				workspaceID: 7,
			},
			setupMock: func(mock *mocks.MockIFileProvider) {
				mock.EXPECT().
					UploadFileForServer(gomock.Any(), "image/png", gomock.Eq(decoded), int64(7)).
					Return("", assert.AnError)
			},
			wantErr: assert.AnError,
		},
		{
			name: "message without parts returns nil",
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{Role: entity.RoleUser},
				},
				workspaceID: 5,
			},
			setupMock: func(mock *mocks.MockIFileProvider) {
				mock.EXPECT().UploadFileForServer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockFile := mocks.NewMockIFileProvider(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockFile)
			}
			p := &PromptServiceImpl{
				file: mockFile,
			}

			err := p.MConvertBase64DataURLToFileURI(tt.args.ctx, tt.args.messages, tt.args.workspaceID)
			unittest.AssertErrorEqual(t, tt.wantErr, err)

			if tt.validateFunc != nil {
				tt.validateFunc(t, tt.args.messages)
			}
		})
	}
}

func TestPromptServiceImpl_MConvertBase64ToFileURL(t *testing.T) {
	type args struct {
		ctx         context.Context
		messages    []*entity.Message
		workspaceID int64
	}

	decoded := []byte("image-bytes")
	dataURL := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(decoded)

	tests := []struct {
		name      string
		args      args
		setupMock func(mock *mocks.MockIFileProvider)
		wantErr   error
		validate  func(t *testing.T, messages []*entity.Message)
	}{
		{
			name: "returns quickly when no base64 data",
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type:     entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{URL: "https://example.com/image.png"},
							},
						},
					},
				},
				workspaceID: 1,
			},
			setupMock: func(mock *mocks.MockIFileProvider) {
				mock.EXPECT().UploadFileForServer(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				mock.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: nil,
		},
		{
			name: "successfully converts base64 to downloadable url",
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URL: dataURL,
								},
							},
						},
					},
				},
				workspaceID: 200,
			},
			setupMock: func(mock *mocks.MockIFileProvider) {
				gomock.InOrder(
					mock.EXPECT().
						UploadFileForServer(gomock.Any(), "image/jpeg", gomock.Eq(decoded), int64(200)).
						Return("workspace/200/file.jpg", nil),
					mock.EXPECT().
						MGetFileURL(gomock.Any(), gomock.Eq([]string{"workspace/200/file.jpg"})).
						Return(map[string]string{"workspace/200/file.jpg": "https://example.com/file.jpg"}, nil),
				)
			},
			wantErr: nil,
			validate: func(t *testing.T, messages []*entity.Message) {
				part := messages[0].Parts[0]
				assert.Equal(t, "workspace/200/file.jpg", part.ImageURL.URI)
				assert.Equal(t, "https://example.com/file.jpg", part.ImageURL.URL)
			},
		},
		{
			name: "upload error bubbles up",
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URL: dataURL,
								},
							},
						},
					},
				},
				workspaceID: 300,
			},
			setupMock: func(mock *mocks.MockIFileProvider) {
				mock.EXPECT().
					UploadFileForServer(gomock.Any(), "image/jpeg", gomock.Eq(decoded), int64(300)).
					Return("", assert.AnError)
			},
			wantErr: assert.AnError,
		},
		{
			name: "fetching url error bubbles up",
			args: args{
				ctx: context.Background(),
				messages: []*entity.Message{
					{
						Role: entity.RoleUser,
						Parts: []*entity.ContentPart{
							{
								Type: entity.ContentTypeImageURL,
								ImageURL: &entity.ImageURL{
									URL: dataURL,
								},
							},
						},
					},
				},
				workspaceID: 400,
			},
			setupMock: func(mock *mocks.MockIFileProvider) {
				gomock.InOrder(
					mock.EXPECT().
						UploadFileForServer(gomock.Any(), "image/jpeg", gomock.Eq(decoded), int64(400)).
						Return("workspace/400/file.jpg", nil),
					mock.EXPECT().
						MGetFileURL(gomock.Any(), gomock.Eq([]string{"workspace/400/file.jpg"})).
						Return(nil, assert.AnError),
				)
			},
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockFile := mocks.NewMockIFileProvider(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockFile)
			}

			p := &PromptServiceImpl{
				file: mockFile,
			}

			err := p.MConvertBase64DataURLToFileURL(tt.args.ctx, tt.args.messages, tt.args.workspaceID)
			unittest.AssertErrorEqual(t, tt.wantErr, err)

			if tt.validate != nil {
				tt.validate(t, tt.args.messages)
			}
		})
	}
}
