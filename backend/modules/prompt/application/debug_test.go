// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/debug"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/loop_gen/infra/kitex/localstream"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service/mocks"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestPromptDebugApplicationImpl_DebugStreaming(t *testing.T) {
	type fields struct {
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		promptService    service.IPromptService
		benefitService   benefit.IBenefitService
		auth             rpc.IAuthProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx    context.Context
		req    *debug.DebugStreamingRequest
		stream debug.PromptDebugService_DebugStreamingServer
	}
	mockContent := "Hello!"
	mockUser := &session.User{
		AppID: 111,
		ID:    "111222333",
		Name:  "test_user",
		Email: "test_user@mock.com",
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
	}{
		{
			name: "success: debug",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugLogRepo := repomocks.NewMockIDebugLogRepo(ctrl)
				mockDebugLogRepo.EXPECT().SaveDebugLog(gomock.Any(), gomock.Any()).Return(nil)
				mockPromptSvc := servicemocks.NewMockIPromptService(ctrl)
				mockPromptSvc.EXPECT().MCompleteMultiModalFileURL(gomock.Any(), gomock.Any(), nil).Return(nil)
				mockPromptSvc.EXPECT().MConvertBase64ToFileURL(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockPromptSvc.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
					for _, v := range mockContent {
						param.ResultStream <- &entity.Reply{
							Item: &entity.ReplyItem{
								Message: &entity.Message{
									Role:    entity.RoleAssistant,
									Content: ptr.Of(string(v)),
								},
							},
						}
					}
					return &entity.Reply{
						Item: &entity.ReplyItem{
							Message: &entity.Message{
								Role:    entity.RoleAssistant,
								Content: ptr.Of(mockContent),
							},
						},
					}, nil
				})
				mockBenefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
				mockBenefitSvc.EXPECT().CheckPromptBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckPromptBenefitResult{}, nil)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					debugLogRepo:     mockDebugLogRepo,
					debugContextRepo: nil,
					promptService:    mockPromptSvc,
					benefitService:   mockBenefitSvc,
					auth:             mockAuth,
					file:             nil,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.DebugStreamingRequest{
					Prompt: &prompt.Prompt{
						ID:          ptr.Of(int64(123456)),
						WorkspaceID: ptr.Of(int64(123456)),
						PromptDraft: &prompt.PromptDraft{
							Detail: &prompt.PromptDetail{
								PromptTemplate: &prompt.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								},
								ModelConfig: &prompt.ModelConfig{},
							},
						},
					},
					SingleStepDebug: ptr.Of(true),
				},
				stream: localstream.NewInMemStream(context.Background(), make(chan *debug.DebugStreamingResponse), make(chan error)),
			},
			wantErr: nil,
		},
		{
			name: "success: playground",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugLogRepo := repomocks.NewMockIDebugLogRepo(ctrl)
				mockDebugLogRepo.EXPECT().SaveDebugLog(gomock.Any(), gomock.Any()).Return(nil)
				mockPromptSvc := servicemocks.NewMockIPromptService(ctrl)
				mockPromptSvc.EXPECT().MCompleteMultiModalFileURL(gomock.Any(), gomock.Any(), nil).Return(nil)
				mockPromptSvc.EXPECT().MConvertBase64ToFileURL(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockPromptSvc.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
					for _, v := range mockContent {
						param.ResultStream <- &entity.Reply{
							Item: &entity.ReplyItem{
								Message: &entity.Message{
									Role:    entity.RoleAssistant,
									Content: ptr.Of(string(v)),
								},
							},
						}
					}
					return &entity.Reply{
						Item: &entity.ReplyItem{
							Message: &entity.Message{
								Role:    entity.RoleAssistant,
								Content: ptr.Of(mockContent),
							},
						},
					}, nil
				})
				mockBenefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
				mockBenefitSvc.EXPECT().CheckPromptBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckPromptBenefitResult{}, nil)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckSpacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					debugLogRepo:     mockDebugLogRepo,
					debugContextRepo: nil,
					promptService:    mockPromptSvc,
					benefitService:   mockBenefitSvc,
					auth:             mockAuth,
					file:             nil,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.DebugStreamingRequest{
					Prompt: &prompt.Prompt{
						ID:          nil,
						WorkspaceID: ptr.Of(int64(123456)),
						PromptDraft: &prompt.PromptDraft{
							Detail: &prompt.PromptDetail{
								PromptTemplate: &prompt.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								},
								ModelConfig: &prompt.ModelConfig{},
							},
						},
					},
					SingleStepDebug: ptr.Of(true),
				},
				stream: localstream.NewInMemStream(context.Background(), make(chan *debug.DebugStreamingResponse), make(chan error)),
			},
			wantErr: nil,
		},
		{
			name:         "invalid param: prompt is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields { return fields{} },
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.DebugStreamingRequest{
					Prompt:          nil,
					SingleStepDebug: ptr.Of(true),
				},
				stream: localstream.NewInMemStream(context.Background(), make(chan *debug.DebugStreamingResponse), make(chan error)),
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode),
		},
		{
			name:         "invalid param: single_step_debug is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields { return fields{} },
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.DebugStreamingRequest{
					Prompt: &prompt.Prompt{
						ID: ptr.Of(int64(123456)),
						PromptDraft: &prompt.PromptDraft{
							Detail: &prompt.PromptDetail{
								PromptTemplate: &prompt.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								},
								ModelConfig: &prompt.ModelConfig{},
							},
						},
					},
					SingleStepDebug: nil,
				},
				stream: localstream.NewInMemStream(context.Background(), make(chan *debug.DebugStreamingResponse), make(chan error)),
			},
			wantErr: errorx.NewByCode(prompterr.CommonInvalidParamCode),
		},
		{
			name: "base64 convert error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugLogRepo := repomocks.NewMockIDebugLogRepo(ctrl)
				mockDebugLogRepo.EXPECT().SaveDebugLog(gomock.Any(), gomock.Any()).Return(nil)
				mockPromptSvc := servicemocks.NewMockIPromptService(ctrl)
				mockPromptSvc.EXPECT().MCompleteMultiModalFileURL(gomock.Any(), gomock.Any(), nil).Return(nil)
				mockPromptSvc.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
					param.ResultStream <- &entity.Reply{
						Item: &entity.ReplyItem{
							Message: &entity.Message{
								Role: entity.RoleAssistant,
								Parts: []*entity.ContentPart{
									{
										Type: entity.ContentTypeImageURL,
										ImageURL: &entity.ImageURL{
											URL: "data:image/png;base64,abc",
										},
									},
								},
							},
						},
					}
					return &entity.Reply{
						Item: &entity.ReplyItem{
							Message: &entity.Message{
								Role: entity.RoleAssistant,
							},
						},
					}, nil
				})
				convertErr := errors.New("convert error")
				mockPromptSvc.EXPECT().MConvertBase64ToFileURL(gomock.Any(), gomock.Any(), gomock.Any()).Return(convertErr)
				mockBenefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
				mockBenefitSvc.EXPECT().CheckPromptBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckPromptBenefitResult{}, nil)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					debugLogRepo:     mockDebugLogRepo,
					debugContextRepo: nil,
					promptService:    mockPromptSvc,
					benefitService:   mockBenefitSvc,
					auth:             mockAuth,
					file:             nil,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.DebugStreamingRequest{
					Prompt: &prompt.Prompt{
						ID:          ptr.Of(int64(123456)),
						WorkspaceID: ptr.Of(int64(123456)),
						PromptDraft: &prompt.PromptDraft{
							Detail: &prompt.PromptDetail{
								PromptTemplate: &prompt.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								},
								ModelConfig: &prompt.ModelConfig{},
							},
						},
					},
					SingleStepDebug: ptr.Of(true),
				},
				stream: localstream.NewInMemStream(context.Background(), make(chan *debug.DebugStreamingResponse), make(chan error)),
			},
			wantErr: errorx.WrapByCode(errors.New("convert error"), prompterr.CommonInternalErrorCode),
		},
		{
			name: "goroutine panic",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugLogRepo := repomocks.NewMockIDebugLogRepo(ctrl)
				mockDebugLogRepo.EXPECT().SaveDebugLog(gomock.Any(), gomock.Any()).Return(nil)
				mockPromptSvc := servicemocks.NewMockIPromptService(ctrl)
				mockPromptSvc.EXPECT().MCompleteMultiModalFileURL(gomock.Any(), gomock.Any(), nil).Return(nil)
				mockPromptSvc.EXPECT().MConvertBase64ToFileURL(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				mockPromptSvc.EXPECT().ExecuteStreaming(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, param service.ExecuteStreamingParam) (*entity.Reply, error) {
					panic("mock panic")
				})
				mockBenefitSvc := benefitmocks.NewMockIBenefitService(ctrl)
				mockBenefitSvc.EXPECT().CheckPromptBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckPromptBenefitResult{}, nil)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					debugLogRepo:     mockDebugLogRepo,
					debugContextRepo: nil,
					promptService:    mockPromptSvc,
					benefitService:   mockBenefitSvc,
					auth:             mockAuth,
					file:             nil,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.DebugStreamingRequest{
					Prompt: &prompt.Prompt{
						ID:          ptr.Of(int64(123456)),
						WorkspaceID: ptr.Of(int64(123456)),
						PromptDraft: &prompt.PromptDraft{
							Detail: &prompt.PromptDetail{
								PromptTemplate: &prompt.PromptTemplate{
									TemplateType: ptr.Of(prompt.TemplateTypeNormal),
								},
								ModelConfig: &prompt.ModelConfig{},
							},
						},
					},
					SingleStepDebug: ptr.Of(true),
				},
				stream: localstream.NewInMemStream(context.Background(), make(chan *debug.DebugStreamingResponse), make(chan error)),
			},
			wantErr: errorx.NewByCode(prompterr.CommonInternalErrorCode),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptDebugApplicationImpl{
				debugLogRepo:     ttFields.debugLogRepo,
				debugContextRepo: ttFields.debugContextRepo,
				promptService:    ttFields.promptService,
				benefitService:   ttFields.benefitService,
				auth:             ttFields.auth,
				file:             ttFields.file,
			}
			stream, ok := tt.args.stream.(*localstream.InMemStream[*debug.DebugStreamingResponse])
			assert.True(t, ok)
			errCh := make(chan error, 1)
			go func() {
				defer close(errCh)
				defer func() {
					_ = stream.CloseSend(tt.args.ctx)
				}()
				err := p.DebugStreaming(tt.args.ctx, tt.args.req, tt.args.stream)
				errCh <- err
			}()
			if tt.wantErr == nil {
				var content string
				for {
					resp, err := stream.Recv(tt.args.ctx)
					if err != nil {
						break
					}
					assert.NotNil(t, resp)
					assert.NotNil(t, resp.Delta)
					content += ptr.From(resp.Delta.Content)
				}
				assert.Equal(t, mockContent, content)
			}
			select { //nolint:staticcheck
			case err := <-errCh:
				unittest.AssertErrorEqual(t, tt.wantErr, err)
			}
		})
	}
}

func TestPromptDebugApplicationImpl_SaveDebugContext(t *testing.T) {
	type fields struct {
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		promptService    service.IPromptService
		benefitService   benefit.IBenefitService
		auth             rpc.IAuthProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx context.Context
		req *debug.SaveDebugContextRequest
	}
	mockUser := &session.User{
		AppID: 111,
		ID:    "111222333",
		Name:  "test_user",
		Email: "test_user@mock.com",
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantR        *debug.SaveDebugContextResponse
		wantErr      error
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugContextRepo := repomocks.NewMockIDebugContextRepo(ctrl)
				mockDebugContextRepo.EXPECT().SaveDebugContext(gomock.Any(), gomock.Any()).Return(nil)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					debugContextRepo: mockDebugContextRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.SaveDebugContextRequest{
					PromptID:     ptr.Of(int64(123)),
					WorkspaceID:  ptr.Of(int64(123456)),
					DebugContext: nil,
				},
			},
			wantR:   &debug.SaveDebugContextResponse{},
			wantErr: nil,
		},
		{
			name: "permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))
				return fields{
					auth: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.SaveDebugContextRequest{
					PromptID:     ptr.Of(int64(123)),
					WorkspaceID:  ptr.Of(int64(123456)),
					DebugContext: nil,
				},
			},
			wantR:   nil,
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "debug context repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugContextRepo := repomocks.NewMockIDebugContextRepo(ctrl)
				mockDebugContextRepo.EXPECT().SaveDebugContext(gomock.Any(), gomock.Any()).Return(errors.New("database error"))
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					debugContextRepo: mockDebugContextRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.SaveDebugContextRequest{
					PromptID:     ptr.Of(int64(123)),
					WorkspaceID:  ptr.Of(int64(123456)),
					DebugContext: nil,
				},
			},
			wantR:   &debug.SaveDebugContextResponse{},
			wantErr: errors.New("database error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptDebugApplicationImpl{
				debugLogRepo:     ttFields.debugLogRepo,
				debugContextRepo: ttFields.debugContextRepo,
				promptService:    ttFields.promptService,
				benefitService:   ttFields.benefitService,
				auth:             ttFields.auth,
				file:             ttFields.file,
			}
			gotR, err := p.SaveDebugContext(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			assert.Equal(t, tt.wantR, gotR)
		})
	}
}

func TestPromptDebugApplicationImpl_GetDebugContext(t *testing.T) {
	type fields struct {
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		promptService    service.IPromptService
		benefitService   benefit.IBenefitService
		auth             rpc.IAuthProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx context.Context
		req *debug.GetDebugContextRequest
	}
	uri2URLMap := map[string]string{
		"test-key1": "test-url1",
		"test-key2": "test-url2",
	}
	mockUser := &session.User{
		AppID: 111,
		ID:    "111222333",
		Name:  "test_user",
		Email: "test_user@mock.com",
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantR        *debug.GetDebugContextResponse
		wantErr      error
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugContextRepo := repomocks.NewMockIDebugContextRepo(ctrl)
				mockDebugContextRepo.EXPECT().GetDebugContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.DebugContext{
					PromptID: 123,
					UserID:   "123",
					DebugCore: &entity.DebugCore{
						MockContexts: []*entity.DebugMessage{
							{
								Role: entity.RoleUser,
								Parts: []*entity.ContentPart{
									{
										Type: entity.ContentTypeImageURL,
										ImageURL: &entity.ImageURL{
											URI: "test-key1",
										},
									},
								},
							},
						},
						MockVariables: nil,
						MockTools:     nil,
					},
					DebugConfig: &entity.DebugConfig{
						SingleStepDebug: ptr.Of(true),
					},
					CompareConfig: &entity.CompareConfig{
						Groups: []*entity.CompareGroup{
							{
								DebugCore: &entity.DebugCore{
									MockContexts: []*entity.DebugMessage{
										{
											Role: entity.RoleUser,
											Parts: []*entity.ContentPart{
												{
													Type: entity.ContentTypeImageURL,
													ImageURL: &entity.ImageURL{
														URI: "test-key2",
													},
												},
											},
										},
									},
									MockVariables: nil,
									MockTools:     nil,
								},
							},
						},
					},
				}, nil)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockFile := rpcmocks.NewMockIFileProvider(ctrl)
				mockFile.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(uri2URLMap, nil)
				return fields{
					debugContextRepo: mockDebugContextRepo,
					auth:             mockAuth,
					file:             mockFile,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.GetDebugContextRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(123456)),
				},
			},
			wantR: &debug.GetDebugContextResponse{
				DebugContext: &prompt.DebugContext{
					DebugCore: &prompt.DebugCore{
						MockContexts: []*prompt.DebugMessage{
							{
								Role: ptr.Of(prompt.RoleUser),
								Parts: []*prompt.ContentPart{
									{
										Type: ptr.Of(prompt.ContentTypeImageURL),
										ImageURL: &prompt.ImageURL{
											URI: ptr.Of("test-key1"),
											URL: ptr.Of("test-url1"),
										},
									},
								},
							},
						},
						MockVariables: nil,
						MockTools:     nil,
					},
					DebugConfig: &prompt.DebugConfig{
						SingleStepDebug: ptr.Of(true),
					},
					CompareConfig: &prompt.CompareConfig{
						Groups: []*prompt.CompareGroup{
							{
								DebugCore: &prompt.DebugCore{
									MockContexts: []*prompt.DebugMessage{
										{
											Role: ptr.Of(prompt.RoleUser),
											Parts: []*prompt.ContentPart{
												{
													Type: ptr.Of(prompt.ContentTypeImageURL),
													ImageURL: &prompt.ImageURL{
														URI: ptr.Of("test-key2"),
														URL: ptr.Of("test-url2"),
													},
												},
											},
										},
									},
									MockVariables: nil,
									MockTools:     nil,
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))
				return fields{
					auth: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.GetDebugContextRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(123456)),
				},
			},
			wantR:   nil,
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "debug context repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugContextRepo := repomocks.NewMockIDebugContextRepo(ctrl)
				mockDebugContextRepo.EXPECT().GetDebugContext(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("database error"))
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					debugContextRepo: mockDebugContextRepo,
					auth:             mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.GetDebugContextRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(123456)),
				},
			},
			wantR:   debug.NewGetDebugContextResponse(),
			wantErr: errors.New("database error"),
		},
		{
			name: "multimodal file URL completion error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugContextRepo := repomocks.NewMockIDebugContextRepo(ctrl)
				debugContext := &entity.DebugContext{
					PromptID: 123,
					UserID:   "123",
					DebugCore: &entity.DebugCore{
						MockContexts: []*entity.DebugMessage{
							{
								Parts: []*entity.ContentPart{
									{
										ImageURL: &entity.ImageURL{
											URI: "test-key",
										},
									},
								},
							},
						},
					},
				}
				mockDebugContextRepo.EXPECT().GetDebugContext(gomock.Any(), gomock.Any(), gomock.Any()).Return(debugContext, nil)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockFile := rpcmocks.NewMockIFileProvider(ctrl)
				mockFile.EXPECT().MGetFileURL(gomock.Any(), gomock.Any()).Return(nil, errors.New("file service error"))
				return fields{
					debugContextRepo: mockDebugContextRepo,
					auth:             mockAuth,
					file:             mockFile,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.GetDebugContextRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(123456)),
				},
			},
			wantR:   debug.NewGetDebugContextResponse(),
			wantErr: errors.New("file service error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptDebugApplicationImpl{
				debugLogRepo:     ttFields.debugLogRepo,
				debugContextRepo: ttFields.debugContextRepo,
				promptService:    ttFields.promptService,
				benefitService:   ttFields.benefitService,
				auth:             ttFields.auth,
				file:             ttFields.file,
			}
			gotR, err := p.GetDebugContext(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			assert.Equal(t, tt.wantR, gotR)
		})
	}
}

func TestPromptDebugApplicationImpl_ListDebugHistory(t *testing.T) {
	type fields struct {
		debugLogRepo     repo.IDebugLogRepo
		debugContextRepo repo.IDebugContextRepo
		promptService    service.IPromptService
		benefitService   benefit.IBenefitService
		auth             rpc.IAuthProvider
		file             rpc.IFileProvider
	}
	type args struct {
		ctx context.Context
		req *debug.ListDebugHistoryRequest
	}
	startTime := time.Now()
	endTime := startTime.Add(1 * time.Second)
	mockUser := &session.User{
		AppID: 111,
		ID:    "111222333",
		Name:  "test_user",
		Email: "test_user@mock.com",
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantR        *debug.ListDebugHistoryResponse
		wantErr      error
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugLogRepo := repomocks.NewMockIDebugLogRepo(ctrl)
				mockDebugLogRepo.EXPECT().ListDebugHistory(gomock.Any(), gomock.Any()).Return(&repo.ListDebugHistoryResult{
					DebugHistory: []*entity.DebugLog{
						{
							ID:           1,
							PromptID:     123,
							SpaceID:      123456,
							PromptKey:    "test_key",
							Version:      "1.0.0",
							InputTokens:  100,
							OutputTokens: 50,
							StartedAt:    startTime,
							EndedAt:      endTime,
							CostMS:       1000,
							StatusCode:   0,
							DebuggedBy:   "123",
							DebugID:      10001,
							DebugStep:    1,
						},
					},
					NextPageToken: 1000,
					HasMore:       true,
				}, nil)
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					debugLogRepo: mockDebugLogRepo,
					auth:         mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.ListDebugHistoryRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(123456)),
					PageToken:   ptr.Of("0"),
					PageSize:    ptr.Of(int32(10)),
					DaysLimit:   ptr.Of(int32(30)),
				},
			},
			wantR: &debug.ListDebugHistoryResponse{
				DebugHistory: []*prompt.DebugLog{
					{
						ID:           ptr.Of(int64(1)),
						PromptID:     ptr.Of(int64(123)),
						WorkspaceID:  ptr.Of(int64(123456)),
						PromptKey:    ptr.Of("test_key"),
						Version:      ptr.Of("1.0.0"),
						InputTokens:  ptr.Of(int64(100)),
						OutputTokens: ptr.Of(int64(50)),
						CostMs:       ptr.Of(int64(1000)),
						StatusCode:   ptr.Of(int32(0)),
						DebuggedBy:   ptr.Of("123"),
						DebugID:      ptr.Of(int64(10001)),
						DebugStep:    ptr.Of(int32(1)),
						StartedAt:    ptr.Of(startTime.UnixMilli()),
						EndedAt:      ptr.Of(endTime.UnixMilli()),
					},
				},
				HasMore:       ptr.Of(true),
				NextPageToken: ptr.Of("1000"),
			},
			wantErr: nil,
		},
		{
			name: "permission check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(prompterr.CommonNoPermissionCode))
				return fields{
					auth: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.ListDebugHistoryRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(123456)),
					PageToken:   ptr.Of("0"),
				},
			},
			wantR:   nil,
			wantErr: errorx.NewByCode(prompterr.CommonNoPermissionCode),
		},
		{
			name: "debug log repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockDebugLogRepo := repomocks.NewMockIDebugLogRepo(ctrl)
				mockDebugLogRepo.EXPECT().ListDebugHistory(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("database error"))
				mockAuth := rpcmocks.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().MCheckPromptPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					debugLogRepo: mockDebugLogRepo,
					auth:         mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), mockUser),
				req: &debug.ListDebugHistoryRequest{
					PromptID:    ptr.Of(int64(123)),
					WorkspaceID: ptr.Of(int64(123456)),
					PageToken:   ptr.Of("0"),
				},
			},
			wantR:   debug.NewListDebugHistoryResponse(),
			wantErr: errors.New("database error"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)
			p := &PromptDebugApplicationImpl{
				debugLogRepo:     ttFields.debugLogRepo,
				debugContextRepo: ttFields.debugContextRepo,
				promptService:    ttFields.promptService,
				benefitService:   ttFields.benefitService,
				auth:             ttFields.auth,
				file:             ttFields.file,
			}
			gotR, err := p.ListDebugHistory(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			assert.Equal(t, tt.wantR, gotR)
		})
	}
}
