// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"io"
	"testing"

	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	limitermocks "github.com/coze-dev/coze-loop/backend/infra/limiter/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/common"
	druntime "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime"
	"github.com/coze-dev/coze-loop/backend/modules/llm/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/service"
	llmservicemocks "github.com/coze-dev/coze-loop/backend/modules/llm/domain/service/mocks"
	llm_errorx "github.com/coze-dev/coze-loop/backend/modules/llm/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
	entitymocks "github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity/mocks"
	"github.com/pkg/errors"
)

func Test_runtimeApp_Chat(t *testing.T) {
	req := &runtime.ChatRequest{
		ModelConfig: &druntime.ModelConfig{
			ModelID:     1,
			Temperature: ptr.Of(float64(1.0)),
			MaxTokens:   ptr.Of(int64(100)),
			TopP:        ptr.Of(float64(0.7)),
			Stop:        []string{"stop words"},
			ToolChoice:  ptr.Of(druntime.ToolChoiceAuto),
		},
		Messages: []*druntime.Message{
			{
				Role:    druntime.RoleUser,
				Content: ptr.Of("your content"),
				MultimodalContents: []*druntime.ChatMessagePart{
					{
						Type: ptr.Of(druntime.ChatMessagePartTypeImageURL),
						Text: nil,
						ImageURL: &druntime.ChatMessageImageURL{
							URL:      ptr.Of("your url"),
							Detail:   ptr.Of(druntime.ImageURLDetailHigh),
							MimeType: ptr.Of("image/png"),
						},
					},
				},
				ToolCalls: []*druntime.ToolCall{
					{
						Index: nil,
						ID:    ptr.Of("toolcall id"),
						Type:  ptr.Of(druntime.ToolTypeFunction),
						FunctionCall: &druntime.FunctionCall{
							Name:      ptr.Of("function name"),
							Arguments: ptr.Of("function arg"),
						},
					},
				},
				ToolCallID: ptr.Of("toolcall id"),
				ResponseMeta: &druntime.ResponseMeta{
					FinishReason: ptr.Of("stop"),
					Usage: &druntime.TokenUsage{
						PromptTokens:     ptr.Of(int64(100)),
						CompletionTokens: ptr.Of(int64(10)),
						TotalTokens:      ptr.Of(int64(110)),
					},
				},
				ReasoningContent: ptr.Of("your reasoning content"),
			},
		},
		Tools: []*druntime.Tool{
			{
				Name:    ptr.Of("tool name"),
				Desc:    ptr.Of("tool desc"),
				DefType: ptr.Of(druntime.ToolDefTypeOpenAPIV3),
				Def:     ptr.Of("{}"),
			},
		},
		BizParam: &druntime.BizParam{
			WorkspaceID:           ptr.Of(int64(1)),
			UserID:                nil,
			Scenario:              ptr.Of(common.ScenarioPromptDebug),
			ScenarioEntityID:      ptr.Of("prompt key"),
			ScenarioEntityVersion: ptr.Of("prompt version"),
		},
		Base: nil,
	}
	type fields struct {
		manageSrv   service.IManage
		runtimeSrv  service.IRuntime
		redis       redis.Cmdable
		rateLimiter limiter.IRateLimiter
	}
	type args struct {
		ctx context.Context
		req *runtime.ChatRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantResp     *runtime.ChatResponse
		wantErr      error
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManage := llmservicemocks.NewMockIManage(ctrl)
				mockRuntime := llmservicemocks.NewMockIRuntime(ctrl)
				mockLimiter := limitermocks.NewMockIRateLimiter(ctrl)

				model := &entity.Model{
					ID:          1,
					WorkspaceID: 0,
					Name:        "your model name",
					Desc:        "your model desc",
					Ability: &entity.Ability{
						MaxContextTokens: ptr.Of(int64(10000)),
						MaxInputTokens:   ptr.Of(int64(6000)),
						MaxOutputTokens:  ptr.Of(int64(4000)),
						FunctionCall:     true,
						JsonMode:         true,
						MultiModal:       true,
						AbilityMultiModal: &entity.AbilityMultiModal{
							Image: true,
							AbilityImage: &entity.AbilityImage{
								URLEnabled:    true,
								BinaryEnabled: true,
								MaxImageSize:  20 * 1024,
								MaxImageCount: 20,
							},
						},
					},
					Frame:          entity.FrameEino,
					Protocol:       entity.ProtocolArk,
					ProtocolConfig: &entity.ProtocolConfig{},
					ScenarioConfigs: map[entity.Scenario]*entity.ScenarioConfig{
						entity.ScenarioDefault: {
							Scenario: entity.ScenarioDefault,
							Quota: &entity.Quota{
								Qpm: 10,
								Tpm: 1000,
							},
							Unavailable: false,
						},
					},
					ParamConfig: nil,
				}
				mockManage.EXPECT().GetModelByID(gomock.Any(), gomock.Any()).Return(model, nil)
				mockLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{
					Allowed:   true,
					OriginKey: "",
					LimitKey:  "",
				}, nil).AnyTimes()
				mockRuntime.EXPECT().HandleMsgsPreCallModel(gomock.Any(), gomock.Any(), gomock.Any()).Return(convertor.MessagesDTO2DO(req.GetMessages()), nil)
				mockRuntime.EXPECT().Generate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(convertor.MessagesDTO2DO(req.GetMessages())[0], nil)
				mockRuntime.EXPECT().CreateModelRequestRecord(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				return fields{
					manageSrv:   mockManage,
					runtimeSrv:  mockRuntime,
					rateLimiter: mockLimiter,
				}
			},
			args: args{
				ctx: context.Background(),
				req: req,
			},
			wantResp: &runtime.ChatResponse{
				Message: req.GetMessages()[0],
			},
			wantErr: nil,
		},
		{
			name: "validate_fail",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &runtime.ChatRequest{},
			},
			wantErr: errorx.NewByCode(llm_errorx.RequestNotValidCode),
		},
		{
			name: "get_model_fail",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManage := llmservicemocks.NewMockIManage(ctrl)
				mockManage.EXPECT().GetModelByID(gomock.Any(), gomock.Any()).Return(nil, errors.New("err"))
				return fields{manageSrv: mockManage}
			},
			args: args{
				ctx: context.Background(),
				req: req,
			},
			wantErr: errors.New("err"),
		},
		{
			name: "model_invalid",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManage := llmservicemocks.NewMockIManage(ctrl)
				mockManage.EXPECT().GetModelByID(gomock.Any(), gomock.Any()).Return(&entity.Model{}, nil)
				return fields{manageSrv: mockManage}
			},
			args: args{
				ctx: context.Background(),
				req: req,
			},
			wantErr: errorx.NewByCode(llm_errorx.ModelInvalidCode),
		},
		{
			name: "rate_limit_blocked",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockManage := llmservicemocks.NewMockIManage(ctrl)
				mockLimiter := limitermocks.NewMockIRateLimiter(ctrl)
				model := &entity.Model{
					ID: 1, Name: "model", Ability: &entity.Ability{},
					Protocol: "ark",
					ProtocolConfig: &entity.ProtocolConfig{
						BaseURL: "http://test.com",
					},
					ScenarioConfigs: map[entity.Scenario]*entity.ScenarioConfig{
						entity.ScenarioDefault: {Scenario: entity.ScenarioDefault, Quota: &entity.Quota{Qpm: 10}},
					},
				}
				mockManage.EXPECT().GetModelByID(gomock.Any(), gomock.Any()).Return(model, nil)
				mockLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: false}, nil)
				return fields{manageSrv: mockManage, rateLimiter: mockLimiter}
			},
			args: args{
				ctx: context.Background(),
				req: req,
			},
			wantErr: errorx.NewByCode(llm_errorx.ModelQPMLimitCode),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ttFields := tt.fieldsGetter(ctrl)
			r := &runtimeApp{
				manageSrv:   ttFields.manageSrv,
				runtimeSrv:  ttFields.runtimeSrv,
				redis:       ttFields.redis,
				rateLimiter: ttFields.rateLimiter,
			}
			gotResp, err := r.Chat(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err != nil {
				return
			}
			assert.Equal(t, tt.wantResp.Message.GetContent(), gotResp.Message.GetContent())
		})
	}
}

func TestNewRuntimeApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockFactory := limitermocks.NewMockIRateLimiterFactory(ctrl)
	mockFactory.EXPECT().NewRateLimiter().Return(nil)
	got := NewRuntimeApplication(nil, nil, nil, mockFactory)
	assert.NotNil(t, got)
}

func Test_runtimeApp_validateChatReq(t *testing.T) {
	r := &runtimeApp{}
	tests := []struct {
		name string
		req  *runtime.ChatRequest
	}{
		{"nil model config", &runtime.ChatRequest{}},
		{"nil biz param", &runtime.ChatRequest{ModelConfig: &druntime.ModelConfig{ModelID: 1}, Messages: []*druntime.Message{{}}}},
		{"missing workspace id", &runtime.ChatRequest{
			ModelConfig: &druntime.ModelConfig{ModelID: 1},
			Messages:    []*druntime.Message{{}},
			BizParam:    &druntime.BizParam{Scenario: ptr.Of(common.ScenarioPromptDebug), ScenarioEntityID: ptr.Of("id")},
		}},
		{"missing scenario", &runtime.ChatRequest{
			ModelConfig: &druntime.ModelConfig{ModelID: 1},
			Messages:    []*druntime.Message{{}},
			BizParam:    &druntime.BizParam{WorkspaceID: ptr.Of(int64(1)), ScenarioEntityID: ptr.Of("id")},
		}},
		{"missing entity id", &runtime.ChatRequest{
			ModelConfig: &druntime.ModelConfig{ModelID: 1},
			Messages:    []*druntime.Message{{}},
			BizParam:    &druntime.BizParam{WorkspaceID: ptr.Of(int64(1)), Scenario: ptr.Of(common.ScenarioPromptDebug)},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.validateChatReq(context.Background(), tt.req)
			assert.Error(t, err)
		})
	}
}

type mockChatStreamServer struct {
}

func (m *mockChatStreamServer) Send(ctx context.Context, resp *runtime.ChatResponse) error {
	return nil
}

func (m *mockChatStreamServer) RecvMsg(ctx context.Context, msg interface{}) error {
	return nil
}

func (m *mockChatStreamServer) SendMsg(ctx context.Context, msg interface{}) error {
	return nil
}

func (m *mockChatStreamServer) SetHeader(header streaming.Header) error {
	return nil
}

func (m *mockChatStreamServer) SendHeader(header streaming.Header) error {
	return nil
}

func (m *mockChatStreamServer) SetTrailer(trailer streaming.Trailer) error {
	return nil
}

func Test_runtimeApp_ChatStream(t *testing.T) {
	req := &runtime.ChatRequest{
		ModelConfig: &druntime.ModelConfig{
			ModelID: 1,
		},
		Messages: []*druntime.Message{
			{Role: druntime.RoleUser, Content: ptr.Of("hi")},
		},
		BizParam: &druntime.BizParam{
			WorkspaceID:      ptr.Of(int64(1)),
			Scenario:         ptr.Of(common.ScenarioPromptDebug),
			ScenarioEntityID: ptr.Of("entity_id"),
		},
	}

	model := &entity.Model{
		ID:   1,
		Name: "model",
		Ability: &entity.Ability{
			MultiModal: true,
			AbilityMultiModal: &entity.AbilityMultiModal{
				Image: true,
				AbilityImage: &entity.AbilityImage{
					URLEnabled: true,
				},
			},
		},
		Protocol: "ark",
		ProtocolConfig: &entity.ProtocolConfig{
			BaseURL: "http://test.com",
		},
		ScenarioConfigs: map[entity.Scenario]*entity.ScenarioConfig{
			entity.ScenarioPromptDebug: {
				Scenario: entity.ScenarioPromptDebug,
				Quota: &entity.Quota{
					Qpm: 10,
				},
			},
		},
	}

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockManage := llmservicemocks.NewMockIManage(ctrl)
		mockRuntime := llmservicemocks.NewMockIRuntime(ctrl)
		mockLimiter := limitermocks.NewMockIRateLimiter(ctrl)
		mockStream := entitymocks.NewMockIStreamReader(ctrl)

		r := &runtimeApp{
			manageSrv:   mockManage,
			runtimeSrv:  mockRuntime,
			rateLimiter: mockLimiter,
		}

		mockManage.EXPECT().GetModelByID(gomock.Any(), gomock.Any()).Return(model, nil)
		mockLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil).AnyTimes()
		mockRuntime.EXPECT().HandleMsgsPreCallModel(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		mockRuntime.EXPECT().Stream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockStream, nil)
		mockStream.EXPECT().Recv().Return(&entity.Message{Content: "h"}, nil)
		mockStream.EXPECT().Recv().Return(nil, io.EOF)
		mockRuntime.EXPECT().CreateModelRequestRecord(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		err := r.ChatStream(context.Background(), req, &mockChatStreamServer{})
		assert.NoError(t, err)
	})

	t.Run("validate_fail", func(t *testing.T) {
		r := &runtimeApp{}
		err := r.ChatStream(context.Background(), &runtime.ChatRequest{}, &mockChatStreamServer{})
		assert.Error(t, err)
	})

	t.Run("get_model_fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockManage := llmservicemocks.NewMockIManage(ctrl)
		r := &runtimeApp{manageSrv: mockManage}
		mockManage.EXPECT().GetModelByID(gomock.Any(), gomock.Any()).Return(nil, errors.New("err"))
		err := r.ChatStream(context.Background(), req, &mockChatStreamServer{})
		assert.Error(t, err)
	})

	t.Run("stream_fail", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockManage := llmservicemocks.NewMockIManage(ctrl)
		mockRuntime := llmservicemocks.NewMockIRuntime(ctrl)
		mockLimiter := limitermocks.NewMockIRateLimiter(ctrl)
		r := &runtimeApp{manageSrv: mockManage, runtimeSrv: mockRuntime, rateLimiter: mockLimiter}
		mockManage.EXPECT().GetModelByID(gomock.Any(), gomock.Any()).Return(model, nil)
		mockLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil).AnyTimes()
		mockRuntime.EXPECT().HandleMsgsPreCallModel(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
		mockRuntime.EXPECT().Stream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("err"))
		mockRuntime.EXPECT().CreateModelRequestRecord(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		err := r.ChatStream(context.Background(), req, &mockChatStreamServer{})
		assert.Error(t, err)
	})
}
