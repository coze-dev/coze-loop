// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
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
		configProvider   conf.IConfigProvider
		llm              rpc.ILLMProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx      context.Context
		messages []*entity.Message
	}
	uri2URLMap := map[string]string{
		"test-image-1": "https://example.com/image1.jpg",
		"test-image-2": "https://example.com/image2.jpg",
		"test-image-3": "https://example.com/image3.jpg",
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
				configProvider:   ttFields.configProvider,
				llm:              ttFields.llm,
				file:             ttFields.file,
			}

			var originMessages []*entity.Message
			err := mem.DeepCopy(tt.args.messages, &originMessages)
			assert.Nil(t, err)
			err = p.MCompleteMultiModalFileURL(tt.args.ctx, tt.args.messages)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			for _, message := range tt.args.messages {
				if message == nil || len(message.Parts) == 0 {
					continue
				}
				for _, part := range message.Parts {
					if part == nil || part.ImageURL == nil {
						continue
					}
					assert.Equal(t, uri2URLMap[part.ImageURL.URI], part.ImageURL.URL)
					part.ImageURL.URL = ""
				}
			}
			assert.Equal(t, originMessages, tt.args.messages)
		})
	}
}

func TestPromptServiceImpl_MGetPromptIDs(t *testing.T) {
	type fields struct {
		idgen            idgen.IIDGenerator
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		manageRepo       repo.IManageRepo
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

func TestPromptServiceImpl_MParseCommitVersionByPromptKey(t *testing.T) {
	type fields struct {
		idgen            idgen.IIDGenerator
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		manageRepo       repo.IManageRepo
		configProvider   conf.IConfigProvider
		llm              rpc.ILLMProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx     context.Context
		spaceID int64
		pairs   []PromptKeyVersionPair
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         map[PromptKeyVersionPair]string
		wantErr      error
	}{
		{
			name: "all prompt keys have version",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				pairs: []PromptKeyVersionPair{
					{
						PromptKey: "test_prompt1",
						Version:   "1.0.0",
					},
					{
						PromptKey: "test_prompt2",
						Version:   "2.0.0",
					},
				},
			},
			want: map[PromptKeyVersionPair]string{
				{PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
				{PromptKey: "test_prompt2", Version: "2.0.0"}: "2.0.0",
			},
			wantErr: nil,
		},
		{
			name: "some prompt keys need latest version",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt2"}),
					gomock.Any(),
				).Return([]*entity.Prompt{
					{
						PromptKey: "test_prompt2",
						PromptBasic: &entity.PromptBasic{
							LatestVersion: "2.0.0",
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
				pairs: []PromptKeyVersionPair{
					{
						PromptKey: "test_prompt1",
						Version:   "1.0.0",
					},
					{
						PromptKey: "test_prompt2",
						Version:   "",
					},
				},
			},
			want: map[PromptKeyVersionPair]string{
				{PromptKey: "test_prompt1", Version: "1.0.0"}: "1.0.0",
				{PromptKey: "test_prompt2", Version: ""}:      "2.0.0",
			},
			wantErr: nil,
		},
		{
			name: "prompt not committed with enhanced error info",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt2"}),
					gomock.Any(),
				).Return([]*entity.Prompt{
					{
						PromptKey: "test_prompt2",
						PromptBasic: &entity.PromptBasic{
							LatestVersion: "",
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
				pairs: []PromptKeyVersionPair{
					{
						PromptKey: "test_prompt1",
						Version:   "1.0.0",
					},
					{
						PromptKey: "test_prompt2",
						Version:   "",
					},
				},
			},
			want:    nil,
			wantErr: errorx.NewByCode(prompterr.PromptUncommittedCode, errorx.WithExtraMsg("prompt key: test_prompt2"), errorx.WithExtra(map[string]string{"prompt_key": "test_prompt2"})),
		},
		{
			name: "database error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().MGetPromptBasicByPromptKey(
					gomock.Any(),
					gomock.Eq(int64(123)),
					gomock.Eq([]string{"test_prompt2"}),
					gomock.Any(),
				).Return(nil, errors.New("database error"))
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: 123,
				pairs: []PromptKeyVersionPair{
					{
						PromptKey: "test_prompt1",
						Version:   "1.0.0",
					},
					{
						PromptKey: "test_prompt2",
						Version:   "",
					},
				},
			},
			want:    nil,
			wantErr: errors.New("database error"),
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
				configProvider:   ttFields.configProvider,
				llm:              ttFields.llm,
				file:             ttFields.file,
			}

			got, err := p.MParseCommitVersionByPromptKey(tt.args.ctx, tt.args.spaceID, tt.args.pairs)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
