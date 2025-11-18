// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
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

type fakeSnippetParser struct {
	parseFunc     func(string) ([]*SnippetReference, error)
	serializeFunc func(*SnippetReference) string
}

func (f fakeSnippetParser) ParseReferences(content string) ([]*SnippetReference, error) {
	if f.parseFunc != nil {
		return f.parseFunc(content)
	}
	return nil, nil
}

func (f fakeSnippetParser) SerializeReference(ref *SnippetReference) string {
	if f.serializeFunc != nil {
		return f.serializeFunc(ref)
	}
	return fmt.Sprintf("<cozeloop_snippet>id=%d&version=%s</cozeloop_snippet>", ref.PromptID, ref.CommitVersion)
}

func TestPromptServiceImpl_SaveDraft(t *testing.T) {
	t.Parallel()
	type fields struct {
		idgen            idgen.IIDGenerator
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		manageRepo       repo.IManageRepo
		labelRepo        repo.ILabelRepo
		configProvider   conf.IConfigProvider
		llm              rpc.ILLMProvider
		file             rpc.IFileProvider
		snippetParser    SnippetParser
	}
	type args struct {
		ctx      context.Context
		promptDO *entity.Prompt
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *entity.DraftInfo
		wantErr      error
		assertFunc   func(t *testing.T, prompt *entity.Prompt)
	}{
		{
			name: "正常保存草稿 - 无片段",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().
					SaveDraft(gomock.Any(), gomock.Any()).
					Return(&entity.DraftInfo{
						UserID:     "user123",
						IsModified: true,
					}, nil)
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID:      123,
					SpaceID: 456,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "user123",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: false,
							},
						},
					},
				},
			},
			want: &entity.DraftInfo{
				UserID:     "user123",
				IsModified: true,
			},
		},
		{
			name: "参数错误 - promptDO为空",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:      context.Background(),
				promptDO: nil,
			},
			wantErr: errorx.New("promptDO or promptDO.PromptDraft is empty"),
		},
		{
			name: "参数错误 - PromptDraft为空",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID:          123,
					PromptDraft: nil,
				},
			},
			wantErr: errorx.New("promptDO or promptDO.PromptDraft is empty"),
		},
		{
			name: "保存失败 - repository错误",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().
					SaveDraft(gomock.Any(), gomock.Any()).
					Return(nil, errorx.New("repository error"))
				return fields{
					manageRepo: mockManageRepo,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					ID:      123,
					SpaceID: 456,
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{
							UserID: "user123",
						},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: false,
							},
						},
					},
				},
			},
			wantErr: errorx.New("repository error"),
		},
		{
			name: "片段解析失败",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					snippetParser: fakeSnippetParser{
						parseFunc: func(string) ([]*SnippetReference, error) {
							return nil, errors.New("parse error")
						},
					},
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages: []*entity.Message{
									{Content: ptr.Of("content")},
								},
							},
						},
					},
				},
			},
			wantErr: errorx.WrapByCode(errors.New("parse error"), prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("failed to parse snippet references")),
		},
		{
			name: "片段引用不存在",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().
					MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(map[repo.GetPromptParam]*entity.Prompt{}, nil)
				return fields{
					manageRepo:    mockManageRepo,
					snippetParser: NewCozeLoopSnippetParser(),
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages: []*entity.Message{
									{Content: ptr.Of("<cozeloop_snippet>id=2&version=v1</cozeloop_snippet>")},
								},
							},
						},
					},
				},
			},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("snippet prompt 2 with version v1 not found")),
		},
		{
			name: "片段校验成功并保存",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManageRepo := repomocks.NewMockIManageRepo(ctrl)
				mockManageRepo.EXPECT().
					MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, params []repo.GetPromptParam, _ ...repo.GetPromptOptionFunc) (map[repo.GetPromptParam]*entity.Prompt, error) {
						assert.Len(t, params, 1)
						query := params[0]
						snippetPrompt := &entity.Prompt{
							ID:      query.PromptID,
							SpaceID: 1,
							PromptBasic: &entity.PromptBasic{
								PromptType: entity.PromptTypeSnippet,
							},
							PromptCommit: &entity.PromptCommit{
								CommitInfo: &entity.CommitInfo{Version: query.CommitVersion},
								PromptDetail: &entity.PromptDetail{
									PromptTemplate: &entity.PromptTemplate{
										Messages: []*entity.Message{{Content: ptr.Of("snippet content")}},
									},
								},
							},
						}
						return map[repo.GetPromptParam]*entity.Prompt{query: snippetPrompt}, nil
					})
				mockManageRepo.EXPECT().
					SaveDraft(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, prompt *entity.Prompt) (*entity.DraftInfo, error) {
						if detail := prompt.GetPromptDetail(); detail != nil && detail.PromptTemplate != nil {
							assert.NotEmpty(t, detail.PromptTemplate.Snippets)
						}
						return &entity.DraftInfo{UserID: "user123", IsModified: true}, nil
					})
				return fields{
					manageRepo:    mockManageRepo,
					snippetParser: NewCozeLoopSnippetParser(),
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptDraft: &entity.PromptDraft{
						DraftInfo: &entity.DraftInfo{UserID: "user123"},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages: []*entity.Message{
									{Content: ptr.Of("<cozeloop_snippet>id=2&version=v1</cozeloop_snippet>")},
								},
							},
						},
					},
				},
			},
			want: &entity.DraftInfo{UserID: "user123", IsModified: true},
			assertFunc: func(t *testing.T, prompt *entity.Prompt) {
				as := assert.New(t)
				detail := prompt.GetPromptDetail()
				as.NotNil(detail)
				if detail == nil || detail.PromptTemplate == nil {
					return
				}
				as.NotEmpty(detail.PromptTemplate.Snippets)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := tt.fieldsGetter(ctrl)

			p := &PromptServiceImpl{
				idgen:            tFields.idgen,
				debugLogRepo:     tFields.debugLogRepo,
				debugContextRepo: tFields.debugContextRepo,
				manageRepo:       tFields.manageRepo,
				labelRepo:        tFields.labelRepo,
				configProvider:   tFields.configProvider,
				llm:              tFields.llm,
				file:             tFields.file,
				snippetParser:    tFields.snippetParser,
			}

			got, err := p.SaveDraft(tt.args.ctx, tt.args.promptDO)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr != nil {
				assert.Nil(t, got)
				return
			}
			assert.NotNil(t, got)
			if tt.want != nil {
				assert.Equal(t, tt.want.UserID, got.UserID)
				assert.Equal(t, tt.want.IsModified, got.IsModified)
			}
			if tt.assertFunc != nil {
				tt.assertFunc(t, tt.args.promptDO)
			}
		})
	}
}

func TestPromptServiceImpl_CreatePrompt(t *testing.T) {
	t.Parallel()
	type fields struct {
		manageRepo    repo.IManageRepo
		snippetParser SnippetParser
	}
	type args struct {
		ctx      context.Context
		promptDO *entity.Prompt
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantID       int64
		wantErr      error
	}{
		{
			name: "prompt do is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:      context.Background(),
				promptDO: nil,
			},
			wantErr: errorx.New("promptDO is empty"),
		},
		{
			name: "prompt key empty",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   1,
					PromptKey: "",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			wantErr: errorx.New("promptKey is empty"),
		},
		{
			name: "prompt basic nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
				},
			},
			wantErr: errorx.New("promptBasic is empty"),
		},
		{
			name: "space id invalid",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   0,
					PromptKey: "key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			wantErr: errorx.New("spaceID is invalid: %d", 0),
		},
		{
			name: "has snippets parse error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					snippetParser: fakeSnippetParser{
						parseFunc: func(string) ([]*SnippetReference, error) {
							return nil, errors.New("parse error")
						},
					},
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages: []*entity.Message{
									{Content: ptr.Of("test content")},
								},
							},
						},
					},
				},
			},
			wantErr: errorx.WrapByCode(errors.New("parse error"), prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("failed to parse snippet references")),
		},
		{
			name: "has snippets but no references",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					snippetParser: fakeSnippetParser{
						parseFunc: func(string) ([]*SnippetReference, error) {
							return []*SnippetReference{}, nil
						},
					},
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages: []*entity.Message{
									{Content: ptr.Of("<cozeloop_snippet>id=2&version=v1</cozeloop_snippet>")},
								},
							},
						},
					},
				},
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("has_snippets is true but no snippet references found in content")),
		},
		{
			name: "snippet prompt type invalid",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, params []repo.GetPromptParam, _ ...repo.GetPromptOptionFunc) (map[repo.GetPromptParam]*entity.Prompt, error) {
					assert.Len(t, params, 1)
					query := params[0]
					snippetPrompt := &entity.Prompt{
						ID:        query.PromptID,
						SpaceID:   1,
						PromptKey: "snippet",
						PromptBasic: &entity.PromptBasic{
							PromptType: entity.PromptTypeNormal,
						},
						PromptCommit: &entity.PromptCommit{
							CommitInfo: &entity.CommitInfo{Version: query.CommitVersion},
							PromptDetail: &entity.PromptDetail{
								PromptTemplate: &entity.PromptTemplate{
									Messages: []*entity.Message{
										{Content: ptr.Of("snippet content")},
									},
								},
							},
						},
					}
					return map[repo.GetPromptParam]*entity.Prompt{query: snippetPrompt}, nil
				})
				return fields{
					manageRepo:    mockRepo,
					snippetParser: NewCozeLoopSnippetParser(),
				}
			},
			args: func() args {
				promptDO := &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages: []*entity.Message{
									{Content: ptr.Of("<cozeloop_snippet>id=2&version=v1</cozeloop_snippet>")},
								},
							},
						},
					},
				}
				return args{
					ctx:      context.Background(),
					promptDO: promptDO,
				}
			}(),
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg(fmt.Sprintf("prompt %d is not a snippet type", 2))),
		},
		{
			name: "create prompt repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).Return(int64(0), errorx.New("create failed"))
				return fields{
					manageRepo: mockRepo,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			wantErr: errorx.New("create failed"),
		},
		{
			name: "snippet repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errorx.New("mget error"))
				return fields{
					manageRepo: mockRepo,
					snippetParser: fakeSnippetParser{
						parseFunc: func(string) ([]*SnippetReference, error) {
							return []*SnippetReference{{PromptID: 2, CommitVersion: "v1"}}, nil
						},
					},
				}
			},
			args: func() args {
				promptDO := &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages: []*entity.Message{
									{Content: ptr.Of("<cozeloop_snippet>id=2&version=v1</cozeloop_snippet>")},
								},
							},
						},
					},
				}
				return args{
					ctx:      context.Background(),
					promptDO: promptDO,
				}
			}(),
			wantErr: errorx.WrapByCode(errorx.New("mget error"), prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("failed to get snippet prompts")),
		},
		{
			name: "success without snippets",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, prompt *entity.Prompt) (int64, error) {
					return 11, nil
				})
				return fields{
					manageRepo: mockRepo,
				}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
			wantID: 11,
		},
		{
			name: "success with snippets",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, params []repo.GetPromptParam, _ ...repo.GetPromptOptionFunc) (map[repo.GetPromptParam]*entity.Prompt, error) {
					assert.Len(t, params, 1)
					query := params[0]
					snippetPrompt := &entity.Prompt{
						ID:      query.PromptID,
						SpaceID: 1,
						PromptBasic: &entity.PromptBasic{
							PromptType: entity.PromptTypeSnippet,
						},
						PromptCommit: &entity.PromptCommit{
							CommitInfo: &entity.CommitInfo{Version: query.CommitVersion},
							PromptDetail: &entity.PromptDetail{
								PromptTemplate: &entity.PromptTemplate{
									Messages: []*entity.Message{
										{Content: ptr.Of("snippet content")},
									},
									VariableDefs: []*entity.VariableDef{{Key: "snippet_var"}},
								},
							},
						},
					}
					return map[repo.GetPromptParam]*entity.Prompt{query: snippetPrompt}, nil
				})
				mockRepo.EXPECT().CreatePrompt(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, prompt *entity.Prompt) (int64, error) {
					if prompt.PromptDraft != nil && prompt.PromptDraft.PromptDetail != nil && prompt.PromptDraft.PromptDetail.PromptTemplate != nil {
						assert.NotEmpty(t, prompt.PromptDraft.PromptDetail.PromptTemplate.Snippets)
					}
					return 101, nil
				})
				return fields{
					manageRepo:    mockRepo,
					snippetParser: NewCozeLoopSnippetParser(),
				}
			},
			args: func() args {
				promptDO := &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages: []*entity.Message{
									{Content: ptr.Of("<cozeloop_snippet>id=2&version=v1</cozeloop_snippet>")},
								},
								VariableDefs: []*entity.VariableDef{{Key: "base_var"}},
							},
						},
					},
				}
				return args{
					ctx:      context.Background(),
					promptDO: promptDO,
				}
			}(),
			wantID: 101,
		},
	}

	for _, tt := range tests {
		ttt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := ttt.fieldsGetter(ctrl)
			service := &PromptServiceImpl{
				manageRepo:    tFields.manageRepo,
				snippetParser: tFields.snippetParser,
			}

			got, err := service.CreatePrompt(ttt.args.ctx, ttt.args.promptDO)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.Equal(t, tt.wantID, got)
			}
		})
	}
}

func TestPromptServiceImpl_ExpandSnippets(t *testing.T) {
	t.Parallel()
	type fields struct {
		manageRepo    repo.IManageRepo
		snippetParser SnippetParser
	}
	type args struct {
		ctx      context.Context
		promptDO *entity.Prompt
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		assertFunc   func(t *testing.T, prompt *entity.Prompt)
		wantErr      error
	}{
		{
			name: "prompt do is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx:      context.Background(),
				promptDO: nil,
			},
			wantErr: errorx.New("promptDO is empty"),
		},
		{
			name: "prompt detail missing",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:   1,
					PromptKey: "key",
					PromptBasic: &entity.PromptBasic{
						PromptType: entity.PromptTypeNormal,
					},
				},
			},
		},
		{
			name: "no snippets flag",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				promptDO: &entity.Prompt{
					SpaceID:     1,
					PromptKey:   "key",
					PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: false,
							},
						},
					},
				},
			},
		},
		{
			name: "snippet not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{}, nil)
				return fields{
					manageRepo:    mockRepo,
					snippetParser: NewCozeLoopSnippetParser(),
				}
			},
			args: func() args {
				promptDO := &entity.Prompt{
					SpaceID:     1,
					PromptKey:   "key",
					PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages:    []*entity.Message{{Content: ptr.Of("<cozeloop_snippet>id=2&version=v1</cozeloop_snippet>")}},
							},
						},
					},
				}
				return args{ctx: context.Background(), promptDO: promptDO}
			}(),
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("snippet prompt 2 with version v1 not found")),
		},
		{
			name: "exceed max depth",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, params []repo.GetPromptParam, _ ...repo.GetPromptOptionFunc) (map[repo.GetPromptParam]*entity.Prompt, error) {
					assert.Len(t, params, 1)
					query := params[0]
					switch query.PromptID {
					case 2:
						snippetPrompt := &entity.Prompt{
							ID:      query.PromptID,
							SpaceID: 1,
							PromptBasic: &entity.PromptBasic{
								PromptType: entity.PromptTypeSnippet,
							},
							PromptCommit: &entity.PromptCommit{
								CommitInfo: &entity.CommitInfo{Version: query.CommitVersion},
								PromptDetail: &entity.PromptDetail{
									PromptTemplate: &entity.PromptTemplate{
										HasSnippets: true,
										Messages:    []*entity.Message{{Content: ptr.Of("<cozeloop_snippet>id=3&version=v2</cozeloop_snippet>")}},
									},
								},
							},
						}
						return map[repo.GetPromptParam]*entity.Prompt{query: snippetPrompt}, nil
					case 3:
						nestedPrompt := &entity.Prompt{
							ID:      query.PromptID,
							SpaceID: 1,
							PromptBasic: &entity.PromptBasic{
								PromptType: entity.PromptTypeSnippet,
							},
							PromptCommit: &entity.PromptCommit{
								CommitInfo: &entity.CommitInfo{Version: query.CommitVersion},
								PromptDetail: &entity.PromptDetail{
									PromptTemplate: &entity.PromptTemplate{
										HasSnippets: true,
									},
								},
							},
						}
						return map[repo.GetPromptParam]*entity.Prompt{query: nestedPrompt}, nil
					default:
						return map[repo.GetPromptParam]*entity.Prompt{}, nil
					}
				}).AnyTimes()
				return fields{
					manageRepo:    mockRepo,
					snippetParser: NewCozeLoopSnippetParser(),
				}
			},
			args: func() args {
				prompt := &entity.Prompt{
					ID:          10,
					SpaceID:     1,
					PromptKey:   "main",
					PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages:    []*entity.Message{{Content: ptr.Of("<cozeloop_snippet>id=2&version=v1</cozeloop_snippet>")}},
							},
						},
					},
				}
				return args{ctx: context.Background(), promptDO: prompt}
			}(),
			wantErr: errorx.New("max recursion depth reached"),
		},
		{
			name: "expand snippets success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomocks.NewMockIManageRepo(ctrl)
				mockRepo.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, params []repo.GetPromptParam, _ ...repo.GetPromptOptionFunc) (map[repo.GetPromptParam]*entity.Prompt, error) {
					assert.Len(t, params, 1)
					query := params[0]
					snippetPrompt := &entity.Prompt{
						ID:          query.PromptID,
						SpaceID:     1,
						PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeSnippet},
						PromptCommit: &entity.PromptCommit{
							CommitInfo: &entity.CommitInfo{Version: query.CommitVersion},
							PromptDetail: &entity.PromptDetail{
								PromptTemplate: &entity.PromptTemplate{
									Messages:     []*entity.Message{{Content: ptr.Of("snippet body")}},
									VariableDefs: []*entity.VariableDef{{Key: "snippet_var"}},
								},
							},
						},
					}
					return map[repo.GetPromptParam]*entity.Prompt{query: snippetPrompt}, nil
				})
				return fields{
					manageRepo:    mockRepo,
					snippetParser: NewCozeLoopSnippetParser(),
				}
			},
			args: func() args {
				prompt := &entity.Prompt{
					ID:          10,
					SpaceID:     1,
					PromptKey:   "main",
					PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeNormal},
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets:  true,
								Messages:     []*entity.Message{{Content: ptr.Of("hello <cozeloop_snippet>id=2&version=v1</cozeloop_snippet>")}},
								VariableDefs: []*entity.VariableDef{{Key: "main_var"}},
							},
						},
					},
				}
				return args{ctx: context.Background(), promptDO: prompt}
			}(),
			assertFunc: func(t *testing.T, prompt *entity.Prompt) {
				detail := prompt.GetPromptDetail()
				as := assert.New(t)
				as.NotNil(detail)
				if detail == nil {
					return
				}
				as.NotNil(detail.PromptTemplate)
				if detail.PromptTemplate == nil {
					return
				}
				as.NotEmpty(detail.PromptTemplate.Snippets)
				as.Equal("hello snippet body", ptr.From(detail.PromptTemplate.Messages[0].Content))
				as.Len(detail.PromptTemplate.VariableDefs, 1)
				if len(detail.PromptTemplate.VariableDefs) > 0 {
					as.Equal("main_var", detail.PromptTemplate.VariableDefs[0].Key)
				}
			},
		},
	}

	for _, tt := range tests {
		ttt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := ttt.fieldsGetter(ctrl)
			service := &PromptServiceImpl{
				manageRepo:    tFields.manageRepo,
				snippetParser: tFields.snippetParser,
			}

			err := service.ExpandSnippets(ttt.args.ctx, ttt.args.promptDO)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil && ttt.assertFunc != nil {
				tt.assertFunc(t, tt.args.promptDO)
			}
		})
	}
}

func TestPromptServiceImpl_expandWithSnippetMap(t *testing.T) {
	t.Parallel()
	type fields struct {
		snippetParser SnippetParser
	}
	type args struct {
		content           string
		snippetContentMap map[string]string
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantContent string
		wantErr     error
	}{
		{
			name: "parse error",
			fields: fields{
				snippetParser: fakeSnippetParser{
					parseFunc: func(string) ([]*SnippetReference, error) {
						return nil, errors.New("parse fail")
					},
				},
			},
			args: args{
				content:           "test",
				snippetContentMap: map[string]string{},
			},
			wantErr: errors.New("parse fail"),
		},
		{
			name: "snippet content missing",
			fields: fields{
				snippetParser: fakeSnippetParser{
					parseFunc: func(string) ([]*SnippetReference, error) {
						return []*SnippetReference{{PromptID: 2, CommitVersion: "v1"}}, nil
					},
				},
			},
			args: args{
				content:           "<cozeloop_snippet>id=2&version=v1</cozeloop_snippet>",
				snippetContentMap: map[string]string{},
			},
			wantErr: errorx.NewByCode(prompterr.ResourceNotFoundCode, errorx.WithExtraMsg("snippet content for prompt 2 with version v1 not found in cache")),
		},
		{
			name: "success expands duplicated snippets",
			fields: fields{
				snippetParser: NewCozeLoopSnippetParser(),
			},
			args: args{
				content: "hello <cozeloop_snippet>id=2&version=v1</cozeloop_snippet> and again <cozeloop_snippet>id=2&version=v1</cozeloop_snippet>",
				snippetContentMap: map[string]string{
					"2_v1": "snippet",
				},
			},
			wantContent: "hello snippet and again snippet",
		},
	}

	for _, tt := range tests {
		ttt := tt
		t.Run(ttt.name, func(t *testing.T) {
			t.Parallel()
			svc := &PromptServiceImpl{
				snippetParser: ttt.fields.snippetParser,
			}
			if svc.snippetParser == nil {
				svc.snippetParser = NewCozeLoopSnippetParser()
			}

			gotContent, err := svc.expandWithSnippetMap(context.Background(), ttt.args.content, ttt.args.snippetContentMap)
			unittest.AssertErrorEqual(t, ttt.wantErr, err)
			if ttt.wantErr != nil {
				return
			}
			assert.Equal(t, ttt.wantContent, gotContent)
		})
	}
}

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

func TestPromptServiceImpl_GetPrompt(t *testing.T) {
	type fields struct {
		manageRepo    repo.IManageRepo
		snippetParser SnippetParser
	}
	type args struct {
		ctx   context.Context
		param GetPromptParam
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name: "repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 1, WithCommit: true, CommitVersion: "1.0.0", WithDraft: false, UserID: ""}).Return(nil, errorx.New("repo error"))
				return fields{manageRepo: repoMock}
			},
			args: args{
				ctx:   context.Background(),
				param: GetPromptParam{PromptID: 1, WithCommit: true, CommitVersion: "1.0.0"},
			},
			wantErr: errorx.New("repo error"),
		},
		{
			name: "parse snippet error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 2, WithCommit: true, CommitVersion: "1.0.0", WithDraft: false, UserID: ""}).Return(&entity.Prompt{
					ID: 2,
					PromptCommit: &entity.PromptCommit{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages:    []*entity.Message{{Content: ptr.Of("content")}},
							},
						},
					},
				}, nil)
				parser := fakeSnippetParser{
					parseFunc: func(string) ([]*SnippetReference, error) { return nil, errors.New("parse fail") },
				}
				return fields{manageRepo: repoMock, snippetParser: parser}
			},
			args: args{
				ctx:   context.Background(),
				param: GetPromptParam{PromptID: 2, WithCommit: true, CommitVersion: "1.0.0"},
			},
			wantErr: errorx.WrapByCode(errors.New("parse fail"), prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("failed to parse snippet references")),
		},
		{
			name: "success expand snippet",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIManageRepo(ctrl)
				promptDO := &entity.Prompt{
					ID: 3,
					PromptDraft: &entity.PromptDraft{
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages:    []*entity.Message{{Content: ptr.Of("hello <cozeloop_snippet>id=4&version=v1</cozeloop_snippet>")}},
							},
						},
					},
				}
				repoMock.EXPECT().GetPrompt(gomock.Any(), repo.GetPromptParam{PromptID: 3, WithCommit: false, CommitVersion: "", WithDraft: false, UserID: "user"}).Return(promptDO, nil)
				repoMock.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[repo.GetPromptParam]*entity.Prompt{
					{PromptID: 4, WithCommit: true, CommitVersion: "v1"}: {
						ID:          4,
						PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeSnippet},
						PromptCommit: &entity.PromptCommit{
							CommitInfo:   &entity.CommitInfo{Version: "v1"},
							PromptDetail: &entity.PromptDetail{PromptTemplate: &entity.PromptTemplate{Messages: []*entity.Message{{Content: ptr.Of("snippet content")}}}},
						},
					},
				}, nil).AnyTimes()
				return fields{manageRepo: repoMock, snippetParser: NewCozeLoopSnippetParser()}
			},
			args: args{
				ctx:   context.Background(),
				param: GetPromptParam{PromptID: 3, UserID: "user", ExpandSnippet: true},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		caseData := tt
		t.Run(caseData.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tFields := caseData.fieldsGetter(ctrl)
			service := &PromptServiceImpl{
				manageRepo:    tFields.manageRepo,
				snippetParser: tFields.snippetParser,
			}

			got, err := service.GetPrompt(caseData.args.ctx, caseData.args.param)
			unittest.AssertErrorEqual(t, caseData.wantErr, err)
			if err == nil {
				assert.NotNil(t, got)
				if caseData.name == "success expand snippet" {
					assert.Contains(t, ptr.From(got.PromptDraft.PromptDetail.PromptTemplate.Messages[0].Content), "snippet content")
				}
			}
		})
	}
}

func TestPromptServiceImpl_doExpandSnippets_MaxDepth(t *testing.T) {
	t.Parallel()
	service := &PromptServiceImpl{snippetParser: NewCozeLoopSnippetParser()}
	err := service.doExpandSnippets(context.Background(), &entity.Prompt{
		PromptDraft: &entity.PromptDraft{
			PromptDetail: &entity.PromptDetail{
				PromptTemplate: &entity.PromptTemplate{HasSnippets: true},
			},
		},
	}, 0)
	unittest.AssertErrorEqual(t, errorx.New("max recursion depth reached"), err)
}

func TestPromptServiceImpl_doExpandSnippets_NoTemplate(t *testing.T) {
	t.Parallel()
	service := &PromptServiceImpl{snippetParser: NewCozeLoopSnippetParser()}
	err := service.doExpandSnippets(context.Background(), &entity.Prompt{}, 2)
	unittest.AssertErrorEqual(t, nil, err)
}

func TestPromptServiceImpl_doExpandSnippets_Nested(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockIManageRepo(ctrl)
	repoMock.EXPECT().MGetPrompt(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, params []repo.GetPromptParam, _ ...repo.GetPromptOptionFunc) (map[repo.GetPromptParam]*entity.Prompt, error) {
		assert.Len(t, params, 1)
		switch params[0].PromptID {
		case 4:
			return map[repo.GetPromptParam]*entity.Prompt{
				params[0]: {
					ID:          4,
					PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeSnippet},
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{Version: params[0].CommitVersion},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								HasSnippets: true,
								Messages:    []*entity.Message{{Content: ptr.Of("<cozeloop_snippet>id=5&version=v2</cozeloop_snippet>")}},
							},
						},
					},
				},
				{PromptID: 0}: {
					ID:           0,
					PromptCommit: &entity.PromptCommit{PromptDetail: &entity.PromptDetail{}},
				},
				{PromptID: 99}: {
					ID:           99,
					PromptCommit: &entity.PromptCommit{PromptDetail: &entity.PromptDetail{}},
				},
			}, nil
		case 5:
			return map[repo.GetPromptParam]*entity.Prompt{
				params[0]: {
					ID:          5,
					PromptBasic: &entity.PromptBasic{PromptType: entity.PromptTypeSnippet},
					PromptCommit: &entity.PromptCommit{
						CommitInfo: &entity.CommitInfo{Version: params[0].CommitVersion},
						PromptDetail: &entity.PromptDetail{
							PromptTemplate: &entity.PromptTemplate{
								Messages: []*entity.Message{{Content: ptr.Of("deep snippet")}},
							},
						},
					},
				},
				{PromptID: 101}: {
					ID:           101,
					PromptCommit: &entity.PromptCommit{PromptDetail: &entity.PromptDetail{}},
				},
			}, nil
		default:
			return map[repo.GetPromptParam]*entity.Prompt{}, nil
		}
	}).AnyTimes()

	service := &PromptServiceImpl{manageRepo: repoMock, snippetParser: NewCozeLoopSnippetParser()}
	prompt := &entity.Prompt{
		PromptDraft: &entity.PromptDraft{
			PromptDetail: &entity.PromptDetail{
				PromptTemplate: &entity.PromptTemplate{
					HasSnippets: true,
					Messages:    []*entity.Message{{Content: ptr.Of("outer <cozeloop_snippet>id=4&version=v1</cozeloop_snippet>")}},
				},
			},
		},
	}

	err := service.doExpandSnippets(context.Background(), prompt, 2)
	unittest.AssertErrorEqual(t, nil, err)
	assert.Equal(t, "outer deep snippet", ptr.From(prompt.PromptDraft.PromptDetail.PromptTemplate.Messages[0].Content))
}
