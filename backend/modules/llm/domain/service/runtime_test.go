// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/component/conf"
	llmconfmocks "github.com/coze-dev/coze-loop/backend/modules/llm/domain/component/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/repo"
	llmrepomocks "github.com/coze-dev/coze-loop/backend/modules/llm/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/service/llmfactory"
	llmfactorymocks "github.com/coze-dev/coze-loop/backend/modules/llm/domain/service/llmfactory/mocks"
	llmifacemocks "github.com/coze-dev/coze-loop/backend/modules/llm/domain/service/llminterface/mocks"
	llm_errorx "github.com/coze-dev/coze-loop/backend/modules/llm/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestRuntimeImpl_Generate(t *testing.T) {
	var opts []entity.Option
	opts = append(opts, entity.WithTools([]*entity.ToolInfo{
		{
			Name:        "get_weather",
			Desc:        "Determine weather in my location",
			ToolDefType: entity.ToolDefTypeOpenAPIV3,
			Def:         "{\"type\":\"object\",\"properties\":{\"location\":{\"type\":\"string\",\"description\":\"The city and state e.g. San Francisco, CA\"},\"unit\":{\"type\":\"string\",\"enum\":[\"c\",\"f\"]}},\"required\":[\"location\"]}",
		},
	}))
	multimodalInput := []*entity.Message{
		{
			Role: entity.RoleUser,
			MultiModalContent: []*entity.ChatMessagePart{
				{
					Type: entity.ChatMessagePartTypeText,
					Text: "there is text",
				},
				{
					Type: entity.ChatMessagePartTypeImageURL,
					ImageURL: &entity.ChatMessageImageURL{
						URL:      "there is url",
						Detail:   entity.ImageURLDetailHigh,
						MIMEType: "image/png",
					},
				},
			},
		},
	}
	model := &entity.Model{
		ID:          0,
		WorkspaceID: 0,
		Name:        "model supports function call and multimodal",
		Desc:        "",
		Ability: &entity.Ability{
			FunctionCall: true,
			MultiModal:   true,
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
		Frame:    entity.FrameEino,
		Protocol: entity.ProtocolArk,
		ProtocolConfig: &entity.ProtocolConfig{
			APIKey: "your api key",
			Model:  "your model",
		},
	}
	modelNotSupportFC := &entity.Model{
		ID:          0,
		WorkspaceID: 0,
		Name:        "model supports multimodal",
		Desc:        "",
		Ability: &entity.Ability{
			FunctionCall: false,
			MultiModal:   true,
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
		Frame:    entity.FrameEino,
		Protocol: entity.ProtocolArk,
		ProtocolConfig: &entity.ProtocolConfig{
			APIKey: "your api key",
			Model:  "your model",
		},
	}
	modelNotSupportMultimodal := &entity.Model{
		ID:          0,
		WorkspaceID: 0,
		Name:        "model supports function call",
		Desc:        "",
		Ability: &entity.Ability{
			FunctionCall: true,
			MultiModal:   false,
		},
		Frame:    entity.FrameEino,
		Protocol: entity.ProtocolArk,
		ProtocolConfig: &entity.ProtocolConfig{
			APIKey: "your api key",
			Model:  "your model",
		},
	}
	type fields struct {
		llmFact     llmfactory.IFactory
		idGen       idgen.IIDGenerator
		runtimeRepo repo.IRuntimeRepo
		runtimeCfg  conf.IConfigRuntime
	}
	type args struct {
		ctx   context.Context
		model *entity.Model
		input []*entity.Message
		opts  []entity.Option
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *entity.Message
		wantErr      error
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				factMock := llmfactorymocks.NewMockIFactory(ctrl)
				llmMock := llmifacemocks.NewMockILLM(ctrl)
				factMock.EXPECT().CreateLLM(gomock.Any(), gomock.Any(), gomock.Any()).Return(llmMock, nil)
				llmMock.EXPECT().Generate(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Message{
					Role:    entity.RoleAssistant,
					Content: "there is content",
				}, nil)
				repoMock := llmrepomocks.NewMockIRuntimeRepo(ctrl)
				idgenMock := idgenmocks.NewMockIIDGenerator(ctrl)
				// idgenMock.EXPECT().GenID(gomock.Any()).Return(int64(1), nil)
				cfgMock := llmconfmocks.NewMockIConfigRuntime(ctrl)
				// cfgMock.EXPECT().NeedCvtURLToBase64().Return(true)
				return fields{
					llmFact:     factMock,
					idGen:       idgenMock,
					runtimeRepo: repoMock,
					runtimeCfg:  cfgMock,
				}
			},
			args: args{
				ctx:   context.Background(),
				model: model,
				input: multimodalInput,
				opts:  opts,
			},
			want: &entity.Message{
				Role:    entity.RoleAssistant,
				Content: "there is content",
			},
			wantErr: nil,
		},
		{
			name: "valid failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				factMock := llmfactorymocks.NewMockIFactory(ctrl)
				repoMock := llmrepomocks.NewMockIRuntimeRepo(ctrl)
				idgenMock := idgenmocks.NewMockIIDGenerator(ctrl)
				// idgenMock.EXPECT().GenID(gomock.Any()).Return(int64(1), nil)
				cfgMock := llmconfmocks.NewMockIConfigRuntime(ctrl)
				// cfgMock.EXPECT().NeedCvtURLToBase64().Return(true)
				return fields{
					llmFact:     factMock,
					idGen:       idgenMock,
					runtimeRepo: repoMock,
					runtimeCfg:  cfgMock,
				}
			},
			args: args{
				ctx:   context.Background(),
				model: modelNotSupportFC,
				input: multimodalInput,
				opts:  opts,
			},
			want:    nil,
			wantErr: errorx.NewByCode(llm_errorx.RequestNotCompatibleWithModelAbilityCode),
		},
		{
			name: "valid failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				factMock := llmfactorymocks.NewMockIFactory(ctrl)
				repoMock := llmrepomocks.NewMockIRuntimeRepo(ctrl)
				idgenMock := idgenmocks.NewMockIIDGenerator(ctrl)
				// idgenMock.EXPECT().GenID(gomock.Any()).Return(int64(1), nil)
				cfgMock := llmconfmocks.NewMockIConfigRuntime(ctrl)
				// cfgMock.EXPECT().NeedCvtURLToBase64().Return(true)
				return fields{
					llmFact:     factMock,
					idGen:       idgenMock,
					runtimeRepo: repoMock,
					runtimeCfg:  cfgMock,
				}
			},
			args: args{
				ctx:   context.Background(),
				model: modelNotSupportMultimodal,
				input: multimodalInput,
				opts:  opts,
			},
			want:    nil,
			wantErr: errorx.NewByCode(llm_errorx.RequestNotCompatibleWithModelAbilityCode),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ttFields := tt.fieldsGetter(ctrl)
			r := &RuntimeImpl{
				llmFact:     ttFields.llmFact,
				idGen:       ttFields.idGen,
				runtimeRepo: ttFields.runtimeRepo,
				runtimeCfg:  ttFields.runtimeCfg,
			}
			got, err := r.Generate(tt.args.ctx, tt.args.model, tt.args.input, tt.args.opts...)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if err != nil {
				return
			}
			assert.Equal(t, tt.want.Content, got.Content)
		})
	}
}

func TestRuntimeImpl_HandleMsgsPreCallModel(t *testing.T) {
	_ = os.Setenv("COZE_LOOP_OSS_PROTOCOL", "http")
	_ = os.Setenv("COZE_LOOP_OSS_DOMAIN", "cozeloop-minio")
	_ = os.Setenv("COZE_LOOP_OSS_PORT", "19000")

	type fields struct {
		llmFact     llmfactory.IFactory
		idGen       idgen.IIDGenerator
		runtimeRepo repo.IRuntimeRepo
		runtimeCfg  conf.IConfigRuntime
	}
	type args struct {
		ctx   context.Context
		model *entity.Model
		msgs  []*entity.Message
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantBase64   string
		wantMimeType string
		wantErr      error
		needMockURL  bool
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				factMock := llmfactorymocks.NewMockIFactory(ctrl)
				repoMock := llmrepomocks.NewMockIRuntimeRepo(ctrl)
				idgenMock := idgenmocks.NewMockIIDGenerator(ctrl)
				// idgenMock.EXPECT().GenID(gomock.Any()).Return(int64(1), nil)
				cfgMock := llmconfmocks.NewMockIConfigRuntime(ctrl)
				cfgMock.EXPECT().NeedCvtURLToBase64().Return(true)
				return fields{
					llmFact:     factMock,
					idGen:       idgenMock,
					runtimeRepo: repoMock,
					runtimeCfg:  cfgMock,
				}
			},
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Ability: &entity.Ability{
						MultiModal: true,
					},
				},
				msgs: []*entity.Message{
					{
						Role: entity.RoleAssistant,
						MultiModalContent: []*entity.ChatMessagePart{
							{
								Type: entity.ChatMessagePartTypeImageURL,
								ImageURL: &entity.ChatMessageImageURL{
									URL: "/your_url",
								},
							},
						},
					},
				},
			},
			wantBase64:   "data:image/jpeg;base64,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",
			wantMimeType: "image/jpeg",
			wantErr:      nil,
			needMockURL:  true,
		},
		{
			name: "model is nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				factMock := llmfactorymocks.NewMockIFactory(ctrl)
				repoMock := llmrepomocks.NewMockIRuntimeRepo(ctrl)
				idgenMock := idgenmocks.NewMockIIDGenerator(ctrl)
				// idgenMock.EXPECT().GenID(gomock.Any()).Return(int64(1), nil)
				cfgMock := llmconfmocks.NewMockIConfigRuntime(ctrl)
				return fields{
					llmFact:     factMock,
					idGen:       idgenMock,
					runtimeRepo: repoMock,
					runtimeCfg:  cfgMock,
				}
			},
			args: args{
				ctx:   context.Background(),
				model: nil,
				msgs: []*entity.Message{
					{
						Role: entity.RoleAssistant,
						MultiModalContent: []*entity.ChatMessagePart{
							{
								Type: entity.ChatMessagePartTypeImageURL,
								ImageURL: &entity.ChatMessageImageURL{
									URL: "https://your_url",
								},
							},
						},
					},
				},
			},
			wantBase64:   "https://your_url",
			wantMimeType: "",
			wantErr:      nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ttFields := tt.fieldsGetter(ctrl)
			httpmock.Activate(t)
			defer httpmock.DeactivateAndReset()

			if tt.needMockURL {
				// 注册URL匹配和响应
				httpmock.RegisterResponder("GET", "http://cozeloop-minio:19000/your_url",
					httpmock.NewBytesResponder(200, []byte{255, 216, 255, 224, 0, 16, 74, 70, 73, 70, 0, 1, 1, 0, 0, 1, 0, 1, 0, 0, 255, 219, 0, 67, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 255, 219, 0, 67, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 255, 192, 0, 17, 8, 0, 160, 0, 214, 3, 1, 34, 0, 2, 17, 1, 3, 17, 1, 255, 196, 0, 31, 0, 1, 0, 1, 3, 5, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 7, 8, 10, 1, 2, 3, 5, 6, 4, 11, 255, 196, 0, 62, 16, 0, 0, 6, 2, 1, 3, 3, 1, 3, 8, 7, 9, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 17, 8, 18, 33, 9, 19, 20, 49, 21, 34, 65, 22, 35, 37, 50, 51, 81, 114, 178, 24, 55, 67, 82, 97, 119, 182, 36, 52, 54, 66, 98, 115, 117, 177, 181, 255, 196, 0, 29, 1, 1, 0, 1, 5, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 1, 3, 4, 6, 8, 5, 7, 9, 255, 196, 0, 56, 17, 0, 2, 2, 2, 1, 2, 3, 7, 2, 3, 7, 5, 1, 0, 0, 0, 1, 2, 3, 4, 0, 5, 17, 6, 18, 19, 33, 49, 7, 20, 34, 50, 81, 129, 177, 35, 65, 21, 97, 145, 8, 22, 36, 66, 82, 98, 113, 37, 51, 115, 130, 209, 240, 255, 218, 0, 12, 3, 1, 0, 2, 17, 3, 17, 0, 63, 0, 207, 114, 181, 153, 46, 76, 149, 101, 33, 147, 143, 239, 161, 45, 52, 194, 191, 92, 155, 79, 97, 119, 44, 140, 136, 200, 248, 109, 37, 247, 136, 140, 204, 212, 125, 169, 79, 111, 61, 224, 0, 199, 198, 0, 0, 49, 128, 0, 12, 96, 0, 3, 24, 0, 0, 198, 0, 0, 49, 128, 0, 12, 96, 0, 3, 24, 0, 1, 121, 62, 81, 247, 252, 156, 96, 0, 4, 177, 128, 0, 12, 96, 0, 3, 24, 0, 1, 101, 254, 99, 246, 252, 12, 96, 0, 4, 113, 128, 0, 12, 96, 0, 3, 24, 0, 0, 198, 0, 0, 49, 128, 0, 12, 96, 0, 104, 165, 37, 36, 106, 81, 146, 82, 146, 51, 82, 148, 100, 68, 68, 94, 76, 204, 207, 193, 17, 23, 147, 51, 240, 65, 140, 212, 5, 189, 236, 46, 160, 113, 252, 85, 111, 214, 99, 200, 103, 35, 186, 107, 148, 56, 180, 56, 162, 169, 134, 239, 208, 210, 244, 150, 188, 203, 113, 31, 85, 179, 17, 125, 164, 124, 182, 185, 13, 56, 74, 36, 241, 97, 157, 70, 98, 151, 202, 98, 30, 68, 210, 241, 155, 7, 59, 81, 239, 60, 163, 145, 82, 227, 167, 193, 127, 190, 37, 36, 184, 169, 82, 185, 50, 57, 77, 161, 164, 23, 133, 200, 231, 131, 60, 225, 173, 188, 97, 19, 138, 210, 24, 207, 152, 224, 124, 125, 188, 3, 221, 225, 252, 253, 164, 31, 38, 237, 224, 143, 63, 79, 60, 210, 27, 218, 63, 68, 166, 225, 180, 111, 212, 52, 18, 242, 16, 140, 89, 216, 83, 19, 22, 237, 53, 206, 195, 183, 220, 132, 234, 222, 77, 17, 156, 50, 183, 192, 127, 80, 21, 23, 18, 3, 134, 60, 152, 243, 25, 110, 68, 71, 217, 147, 29, 212, 146, 154, 126, 59, 168, 121, 151, 18, 126, 73, 72, 113, 181, 41, 10, 35, 47, 197, 38, 100, 57, 134, 15, 167, 174, 110, 202, 202, 202, 25, 88, 50, 176, 5, 89, 72, 42, 192, 249, 130, 8, 228, 16, 71, 152, 35, 200, 224, 0, 5, 228, 249, 71, 223, 242, 114, 184, 0, 1, 44, 96, 0, 3, 24, 0, 0, 198, 0, 0, 89, 127, 152, 253, 191, 3, 24, 0, 1, 28, 96, 0, 3, 24, 0, 0, 198, 0, 0, 49, 128, 0, 12, 96, 7, 207, 42, 92, 88, 49, 222, 151, 54, 75, 17, 34, 199, 66, 157, 126, 76, 151, 80, 195, 12, 182, 130, 53, 41, 110, 58, 226, 146, 132, 37, 36, 70, 102, 165, 40, 136, 136, 90, 150, 197, 234, 66, 59, 9, 126, 171, 1, 65, 72, 127, 149, 54, 238, 65, 41, 179, 248, 237, 145, 114, 147, 251, 58, 35, 137, 37, 62, 179, 63, 41, 147, 32, 146, 210, 72, 136, 208, 203, 189, 196, 164, 101, 85, 167, 98, 227, 246, 65, 25, 110, 8, 238, 115, 228, 136, 15, 238, 205, 232, 60, 188, 192, 28, 177, 253, 129, 57, 172, 245, 55, 87, 232, 58, 70, 161, 183, 186, 188, 144, 22, 86, 53, 234, 71, 196, 183, 174, 50, 255, 0, 146, 181, 96, 67, 191, 153, 10, 210, 185, 72, 35, 44, 12, 178, 198, 15, 57, 112, 57, 150, 127, 140, 96, 176, 206, 85, 244, 244, 52, 242, 219, 82, 226, 215, 179, 195, 182, 19, 76, 185, 34, 75, 17, 200, 200, 251, 77, 69, 218, 111, 58, 109, 178, 131, 231, 189, 194, 224, 197, 143, 108, 61, 225, 147, 230, 228, 237, 124, 69, 42, 134, 129, 74, 63, 209, 240, 221, 63, 147, 45, 5, 225, 63, 104, 77, 74, 80, 227, 169, 227, 147, 56, 205, 123, 81, 140, 207, 243, 136, 116, 210, 133, 38, 145, 90, 90, 216, 92, 77, 145, 103, 109, 53, 249, 211, 100, 40, 220, 145, 46, 91, 170, 113, 213, 153, 23, 5, 220, 181, 159, 221, 66, 18, 68, 148, 32, 184, 67, 104, 34, 74, 73, 41, 34, 34, 165, 25, 38, 197, 129, 90, 78, 196, 169, 236, 176, 156, 158, 80, 110, 145, 153, 194, 97, 101, 224, 204, 214, 147, 47, 144, 164, 249, 251, 141, 40, 145, 201, 112, 167, 60, 26, 79, 120, 211, 116, 209, 121, 20, 69, 17, 183, 100, 112, 90, 70, 94, 33, 135, 253, 222, 127, 10, 128, 125, 29, 201, 98, 71, 40, 1, 61, 185, 200, 157, 125, 237, 155, 115, 212, 73, 53, 74, 178, 54, 139, 68, 229, 147, 221, 96, 148, 251, 237, 232, 253, 56, 185, 97, 56, 118, 14, 57, 45, 86, 191, 100, 0, 49, 73, 141, 128, 162, 76, 247, 118, 22, 112, 42, 216, 57, 54, 18, 154, 138, 201, 31, 4, 183, 85, 193, 173, 95, 94, 212, 32, 185, 91, 139, 63, 238, 161, 42, 87, 248, 4, 11, 40, 22, 140, 148, 136, 18, 217, 148, 215, 142, 84, 210, 201, 70, 131, 63, 161, 56, 143, 215, 109, 94, 15, 238, 173, 41, 63, 7, 224, 90, 181, 157, 181, 133, 196, 131, 149, 99, 37, 201, 46, 249, 36, 119, 159, 8, 105, 38, 124, 246, 52, 217, 112, 134, 209, 207, 252, 169, 34, 231, 234, 124, 159, 145, 195, 14, 116, 202, 231, 211, 38, 12, 151, 162, 190, 159, 163, 140, 172, 208, 163, 47, 197, 42, 227, 194, 210, 127, 138, 22, 74, 73, 254, 36, 99, 232, 99, 164, 135, 187, 142, 109, 145, 107, 212, 240, 156, 193, 252, 147, 143, 41, 15, 254, 78, 71, 175, 253, 191, 47, 63, 132, 29, 231, 234, 249, 65, 204, 62, 158, 109, 196, 191, 205, 191, 116, 255, 0, 215, 247, 255, 0, 88, 203, 241, 196, 246, 14, 91, 133, 72, 75, 212, 54, 242, 24, 103, 159, 206, 215, 188, 163, 147, 91, 33, 60, 249, 39, 97, 58, 106, 100, 148, 101, 224, 158, 104, 155, 125, 4, 103, 237, 186, 158, 79, 155, 175, 194, 186, 148, 162, 180, 54, 225, 101, 240, 213, 69, 49, 93, 169, 77, 140, 110, 249, 53, 78, 168, 252, 114, 234, 120, 57, 48, 204, 207, 241, 52, 200, 104, 190, 170, 117, 178, 47, 49, 97, 71, 180, 31, 111, 181, 139, 216, 196, 250, 60, 17, 76, 138, 68, 135, 146, 95, 78, 94, 96, 207, 177, 223, 223, 220, 217, 180, 127, 244, 40, 252, 138, 177, 87, 117, 89, 114, 207, 189, 93, 49, 153, 41, 241, 222, 132, 171, 181, 230, 140, 252, 240, 235, 42, 225, 214, 207, 247, 119, 36, 136, 254, 169, 51, 47, 35, 76, 220, 116, 203, 161, 99, 114, 169, 95, 217, 110, 65, 230, 167, 211, 130, 100, 0, 143, 228, 22, 101, 13, 235, 192, 30, 185, 244, 254, 142, 246, 165, 212, 93, 54, 99, 143, 81, 181, 51, 83, 83, 220, 250, 125, 143, 117, 138, 132, 114, 11, 4, 133, 220, 73, 95, 159, 86, 122, 82, 197, 201, 249, 153, 188, 198, 76, 61, 125, 149, 125, 180, 70, 167, 86, 77, 141, 62, 27, 197, 220, 212, 152, 143, 54, 251, 43, 47, 196, 137, 109, 169, 73, 238, 35, 240, 164, 153, 146, 146, 126, 20, 68, 101, 192, 251, 68, 84, 98, 249, 134, 75, 136, 201, 84, 172, 126, 222, 93, 114, 148, 164, 169, 230, 91, 112, 213, 18, 71, 111, 60, 20, 152, 142, 119, 71, 127, 130, 240, 70, 227, 102, 164, 145, 153, 37, 69, 200, 187, 189, 109, 191, 228, 100, 179, 33, 208, 95, 80, 75, 118, 217, 245, 19, 109, 207, 161, 142, 236, 150, 30, 252, 13, 217, 80, 62, 243, 176, 208, 218, 121, 91, 239, 161, 231, 88, 74, 73, 78, 26, 24, 66, 120, 26, 117, 173, 37, 138, 232, 100, 136, 137, 225, 80, 73, 35, 133, 145, 84, 31, 50, 202, 79, 4, 15, 220, 161, 39, 143, 50, 160, 122, 117, 7, 71, 251, 107, 233, 254, 161, 146, 190, 191, 105, 12, 186, 77, 172, 236, 144, 198, 173, 221, 102, 133, 153, 221, 130, 162, 65, 102, 52, 241, 34, 105, 9, 228, 37, 152, 99, 69, 36, 32, 158, 70, 224, 155, 156, 0, 1, 227, 103, 218, 112, 0, 1, 140, 0, 0, 99, 0, 0, 44, 191, 204, 126, 223, 129, 140, 0, 0, 142, 48, 0, 2, 77, 243, 31, 255, 0, 126, 216, 192, 0, 8, 227, 0, 0, 24, 192, 0, 208, 204, 136, 140, 207, 193, 17, 25, 153, 255, 0, 129, 125, 67, 25, 104, 155, 199, 8, 218, 215, 50, 29, 176, 98, 105, 223, 227, 77, 26, 148, 205, 53, 66, 85, 21, 202, 246, 210, 94, 23, 34, 184, 220, 82, 236, 92, 63, 170, 164, 182, 228, 151, 72, 249, 225, 136, 237, 18, 72, 172, 135, 32, 190, 175, 198, 80, 231, 218, 139, 91, 50, 155, 53, 36, 160, 118, 241, 53, 110, 36, 248, 54, 253, 133, 246, 169, 179, 35, 240, 165, 61, 237, 161, 60, 31, 42, 228, 184, 20, 247, 170, 95, 86, 248, 42, 122, 231, 4, 233, 117, 81, 39, 74, 173, 155, 97, 75, 125, 178, 236, 218, 105, 244, 192, 177, 132, 243, 145, 37, 194, 198, 232, 220, 53, 113, 42, 51, 237, 173, 46, 88, 221, 182, 68, 149, 39, 181, 138, 165, 145, 162, 88, 136, 24, 91, 215, 104, 51, 125, 99, 145, 90, 229, 118, 185, 69, 133, 204, 215, 44, 45, 215, 147, 77, 147, 112, 171, 9, 111, 43, 151, 94, 113, 233, 78, 170, 67, 43, 89, 112, 158, 99, 186, 202, 73, 9, 66, 9, 36, 132, 32, 147, 216, 222, 206, 191, 179, 103, 95, 111, 250, 121, 55, 59, 117, 215, 104, 161, 146, 24, 231, 213, 106, 172, 187, 71, 178, 216, 68, 223, 23, 137, 104, 64, 146, 195, 174, 89, 83, 134, 139, 198, 230, 204, 140, 120, 154, 26, 192, 137, 143, 12, 251, 96, 254, 239, 77, 191, 183, 63, 77, 237, 54, 59, 45, 155, 204, 227, 100, 54, 18, 189, 157, 125, 121, 7, 151, 131, 70, 228, 205, 239, 78, 35, 63, 8, 128, 172, 149, 98, 28, 44, 54, 2, 40, 133, 101, 35, 34, 206, 109, 111, 77, 76, 54, 163, 129, 95, 228, 138, 51, 11, 62, 247, 75, 159, 7, 37, 226, 225, 78, 127, 219, 79, 107, 69, 253, 213, 25, 119, 15, 18, 40, 86, 23, 191, 49, 28, 148, 152, 137, 110, 178, 198, 173, 151, 218, 133, 55, 57, 212, 157, 107, 206, 159, 5, 254, 207, 96, 100, 132, 33, 43, 63, 213, 76, 180, 48, 162, 51, 236, 37, 44, 251, 84, 186, 232, 149, 37, 105, 37, 33, 68, 164, 168, 137, 73, 82, 76, 148, 149, 36, 203, 146, 50, 50, 228, 140, 140, 188, 145, 145, 240, 101, 244, 23, 54, 125, 43, 179, 233, 41, 134, 183, 103, 169, 159, 85, 34, 247, 20, 73, 99, 253, 57, 194, 144, 26, 72, 108, 169, 120, 173, 175, 60, 3, 44, 82, 202, 57, 224, 22, 231, 203, 57, 222, 218, 218, 89, 73, 182, 28, 72, 125, 11, 250, 16, 56, 249, 8, 248, 10, 143, 47, 151, 203, 239, 154, 128, 0, 243, 115, 23, 3, 154, 60, 153, 17, 29, 75, 241, 159, 118, 59, 168, 242, 151, 89, 113, 77, 173, 60, 121, 253, 100, 153, 31, 30, 60, 151, 60, 126, 241, 228, 114, 172, 203, 29, 195, 32, 252, 251, 251, 6, 162, 33, 125, 229, 26, 57, 31, 185, 50, 98, 208, 68, 106, 110, 44, 100, 159, 184, 241, 151, 37, 220, 162, 34, 109, 190, 226, 55, 22, 146, 50, 49, 101, 187, 7, 122, 228, 89, 103, 201, 173, 165, 55, 40, 40, 92, 238, 104, 219, 101, 207, 210, 51, 153, 250, 31, 203, 148, 130, 73, 178, 135, 75, 202, 163, 70, 52, 164, 146, 126, 211, 142, 188, 158, 227, 86, 251, 209, 222, 206, 58, 135, 173, 100, 13, 78, 184, 171, 170, 13, 217, 99, 109, 113, 25, 105, 168, 7, 182, 68, 129, 120, 238, 185, 50, 249, 143, 10, 30, 85, 88, 5, 154, 88, 65, 13, 153, 244, 245, 246, 109, 176, 49, 169, 72, 193, 243, 153, 185, 10, 56, 35, 229, 253, 217, 135, 236, 23, 208, 250, 145, 235, 151, 227, 39, 172, 12, 71, 4, 183, 175, 168, 201, 219, 177, 203, 152, 76, 182, 219, 184, 94, 46, 112, 126, 212, 129, 9, 42, 34, 120, 201, 233, 174, 177, 89, 46, 113, 39, 159, 102, 49, 184, 217, 169, 68, 101, 42, 67, 28, 146, 149, 52, 93, 51, 238, 238, 154, 118, 101, 12, 86, 180, 182, 85, 79, 38, 201, 200, 173, 187, 101, 69, 102, 105, 174, 206, 154, 116, 147, 220, 234, 109, 171, 103, 123, 115, 31, 91, 74, 238, 37, 187, 94, 114, 170, 254, 233, 156, 55, 214, 193, 33, 67, 18, 237, 121, 173, 179, 189, 177, 148, 193, 195, 117, 230, 51, 107, 150, 100, 182, 74, 51, 98, 182, 170, 58, 159, 90, 26, 73, 145, 61, 50, 99, 198, 105, 143, 6, 12, 114, 81, 42, 76, 233, 142, 177, 21, 132, 159, 115, 174, 167, 146, 230, 126, 186, 69, 244, 181, 161, 214, 210, 105, 118, 38, 244, 180, 252, 164, 206, 96, 201, 141, 103, 83, 136, 208, 204, 149, 19, 29, 199, 37, 71, 90, 31, 140, 187, 27, 72, 206, 177, 50, 254, 197, 167, 82, 151, 22, 211, 63, 18, 169, 149, 163, 217, 52, 217, 183, 249, 229, 61, 183, 251, 43, 246, 51, 209, 93, 51, 10, 236, 122, 175, 107, 168, 234, 193, 11, 205, 78, 158, 187, 192, 217, 89, 221, 202, 252, 246, 180, 253, 62, 101, 175, 29, 58, 42, 234, 203, 21, 195, 119, 95, 12, 96, 58, 201, 61, 219, 1, 35, 110, 159, 246, 33, 95, 168, 170, 108, 204, 154, 190, 158, 165, 184, 170, 89, 98, 187, 187, 217, 171, 86, 146, 138, 14, 222, 244, 169, 178, 236, 155, 178, 70, 82, 11, 213, 138, 189, 153, 164, 5, 12, 157, 145, 130, 226, 97, 128, 8, 136, 136, 136, 188, 17, 23, 4, 95, 184, 136, 7, 5, 242, 126, 167, 250, 159, 254, 231, 101, 96, 0, 3, 147, 245, 63, 212, 227, 0, 0, 28, 159, 169, 254, 167, 24, 0, 1, 113, 64, 32, 18, 1, 63, 83, 230, 125, 113, 128, 0, 18, 224, 125, 7, 244, 24, 192, 0, 11, 79, 243, 31, 183, 224, 99, 0, 0, 35, 140, 0, 0, 99, 3, 99, 159, 179, 115, 248, 21, 252, 166, 55, 141, 142, 126, 205, 207, 224, 87, 242, 152, 170, 250, 143, 249, 31, 156, 103, 230, 133, 176, 111, 174, 113, 237, 193, 178, 231, 82, 89, 75, 173, 148, 157, 133, 153, 126, 114, 51, 166, 130, 89, 39, 36, 178, 50, 75, 205, 159, 45, 60, 142, 127, 179, 117, 11, 65, 254, 41, 21, 107, 13, 234, 53, 38, 76, 66, 205, 97, 118, 168, 187, 91, 59, 170, 214, 140, 210, 162, 240, 94, 236, 184, 4, 102, 105, 63, 197, 197, 67, 228, 143, 207, 183, 21, 62, 18, 116, 59, 109, 127, 90, 187, 55, 252, 193, 204, 255, 0, 212, 118, 66, 159, 15, 215, 157, 30, 223, 97, 172, 171, 80, 213, 157, 130, 26, 240, 22, 130, 78, 94, 7, 253, 36, 245, 140, 158, 1, 62, 133, 144, 163, 241, 232, 195, 56, 235, 105, 169, 161, 178, 121, 69, 168, 21, 159, 196, 112, 179, 39, 193, 50, 252, 71, 142, 36, 3, 146, 7, 250, 95, 185, 63, 219, 146, 153, 83, 117, 83, 125, 17, 19, 169, 236, 98, 88, 196, 112, 136, 210, 244, 87, 146, 225, 23, 37, 207, 107, 137, 35, 239, 105, 101, 244, 83, 110, 37, 11, 73, 242, 74, 73, 25, 25, 10, 187, 134, 109, 108, 195, 9, 91, 77, 215, 216, 28, 202, 198, 204, 187, 233, 236, 123, 164, 66, 82, 60, 242, 150, 79, 184, 158, 136, 124, 31, 41, 84, 103, 91, 34, 81, 23, 114, 22, 146, 52, 28, 56, 81, 228, 87, 120, 220, 212, 79, 163, 178, 149, 93, 37, 6, 70, 106, 142, 233, 165, 183, 72, 191, 179, 144, 193, 242, 204, 150, 143, 130, 229, 167, 219, 113, 6, 100, 71, 219, 201, 17, 148, 184, 116, 121, 161, 122, 169, 234, 206, 138, 214, 251, 15, 212, 19, 167, 98, 244, 241, 165, 45, 25, 244, 137, 49, 49, 220, 114, 230, 116, 46, 82, 245, 61, 74, 238, 29, 140, 221, 173, 161, 184, 149, 182, 104, 168, 114, 75, 12, 60, 70, 204, 179, 136, 101, 200, 218, 182, 93, 93, 209, 182, 245, 114, 69, 214, 223, 194, 181, 250, 230, 41, 28, 178, 238, 164, 174, 186, 195, 36, 159, 4, 101, 108, 89, 40, 43, 204, 204, 79, 132, 220, 171, 161, 224, 164, 221, 195, 145, 162, 93, 232, 77, 141, 134, 104, 117, 181, 164, 220, 171, 6, 111, 118, 134, 22, 146, 216, 69, 28, 179, 120, 40, 27, 196, 10, 63, 207, 25, 13, 207, 159, 98, 229, 236, 97, 91, 223, 16, 202, 59, 34, 89, 58, 88, 221, 169, 164, 191, 49, 98, 234, 10, 11, 234, 252, 74, 53, 129, 246, 181, 221, 227, 195, 114, 9, 133, 171, 146, 38, 253, 197, 114, 69, 228, 54, 23, 80, 245, 245, 135, 38, 171, 10, 67, 118, 115, 211, 222, 202, 238, 158, 73, 157, 116, 103, 56, 52, 169, 81, 26, 81, 37, 83, 156, 109, 92, 146, 92, 81, 38, 33, 168, 187, 146, 114, 26, 227, 186, 214, 179, 124, 7, 52, 214, 217, 4, 188, 87, 61, 198, 46, 113, 60, 134, 23, 6, 253, 85, 228, 23, 224, 201, 246, 205, 74, 74, 36, 50, 79, 33, 40, 147, 21, 211, 66, 189, 153, 113, 148, 236, 103, 137, 38, 109, 58, 178, 35, 49, 81, 116, 111, 78, 59, 123, 168, 156, 131, 236, 29, 97, 138, 202, 182, 67, 14, 180, 221, 173, 252, 178, 114, 22, 51, 66, 135, 79, 238, 187, 113, 114, 182, 150, 196, 127, 187, 203, 137, 138, 202, 100, 79, 121, 9, 81, 198, 134, 241, 151, 3, 83, 62, 202, 189, 152, 233, 222, 94, 176, 189, 178, 136, 116, 204, 85, 210, 234, 71, 123, 103, 92, 232, 98, 70, 33, 146, 195, 95, 44, 30, 205, 71, 5, 4, 48, 201, 102, 68, 149, 152, 171, 53, 133, 116, 141, 116, 90, 221, 39, 36, 251, 53, 163, 94, 141, 235, 23, 90, 95, 5, 53, 130, 25, 94, 113, 56, 60, 52, 109, 15, 103, 143, 202, 241, 241, 35, 143, 135, 134, 50, 30, 209, 229, 72, 173, 174, 109, 242, 43, 5, 216, 91, 205, 149, 103, 97, 33, 68, 147, 117, 245, 27, 142, 40, 212, 175, 186, 211, 72, 34, 237, 66, 59, 149, 194, 26, 105, 9, 65, 115, 194, 82, 92, 137, 32, 233, 91, 211, 55, 106, 111, 20, 64, 203, 182, 49, 205, 213, 154, 221, 245, 180, 243, 46, 207, 136, 73, 204, 50, 56, 156, 146, 150, 170, 74, 121, 72, 253, 31, 21, 212, 120, 102, 214, 225, 164, 50, 225, 40, 158, 137, 14, 123, 68, 102, 114, 203, 210, 191, 166, 238, 163, 208, 10, 173, 203, 50, 228, 70, 217, 251, 62, 51, 109, 186, 155, 155, 104, 72, 86, 55, 143, 205, 52, 165, 75, 94, 53, 69, 36, 157, 71, 191, 29, 206, 83, 26, 230, 200, 157, 176, 79, 106, 95, 138, 213, 107, 138, 83, 105, 145, 194, 34, 34, 34, 34, 34, 34, 34, 34, 34, 46, 8, 136, 188, 17, 17, 23, 130, 34, 47, 4, 69, 244, 28, 239, 237, 75, 251, 91, 71, 20, 82, 244, 239, 178, 90, 169, 86, 188, 43, 238, 223, 222, 139, 85, 18, 37, 72, 208, 21, 11, 162, 212, 203, 26, 164, 49, 168, 10, 35, 183, 126, 32, 66, 247, 44, 122, 244, 34, 59, 25, 212, 125, 17, 236, 41, 35, 16, 236, 58, 197, 135, 195, 195, 197, 161, 169, 32, 8, 190, 133, 69, 251, 80, 145, 207, 30, 125, 213, 170, 48, 95, 148, 181, 166, 29, 241, 101, 29, 211, 26, 11, 84, 104, 28, 109, 188, 99, 87, 226, 112, 104, 34, 155, 76, 166, 194, 200, 200, 229, 222, 221, 188, 210, 120, 249, 87, 87, 15, 247, 76, 156, 242, 214, 106, 115, 177, 75, 68, 86, 20, 181, 38, 36, 104, 237, 118, 182, 154, 198, 0, 56, 115, 99, 178, 216, 109, 238, 217, 217, 109, 110, 218, 216, 236, 46, 72, 211, 90, 187, 118, 121, 109, 90, 177, 43, 122, 188, 211, 204, 207, 36, 141, 232, 57, 102, 60, 0, 0, 224, 0, 51, 163, 234, 212, 171, 70, 188, 85, 41, 87, 134, 165, 88, 16, 71, 13, 122, 241, 36, 48, 196, 131, 209, 82, 56, 194, 162, 143, 223, 200, 14, 73, 36, 249, 147, 128, 0, 24, 89, 145, 128, 0, 12, 96, 0, 3, 24, 0, 1, 121, 62, 81, 247, 252, 156, 96, 0, 4, 177, 128, 0, 22, 95, 230, 63, 111, 192, 198, 0, 0, 71, 24, 0, 0, 198, 6, 199, 63, 102, 231, 240, 43, 249, 76, 111, 27, 28, 253, 154, 255, 0, 129, 95, 250, 49, 85, 245, 31, 242, 63, 56, 207, 204, 175, 109, 127, 90, 187, 55, 252, 193, 204, 255, 0, 212, 118, 67, 143, 90, 234, 237, 137, 184, 243, 10, 140, 7, 86, 225, 153, 6, 119, 152, 222, 73, 110, 45, 109, 14, 57, 92, 253, 140, 215, 84, 226, 210, 131, 125, 255, 0, 105, 38, 204, 24, 17, 251, 189, 217, 182, 83, 221, 141, 95, 2, 58, 92, 147, 54, 84, 120, 237, 184, 234, 102, 47, 167, 31, 71, 61, 251, 213, 102, 217, 206, 115, 125, 134, 204, 205, 39, 165, 30, 207, 242, 201, 136, 200, 175, 235, 221, 78, 95, 152, 68, 119, 36, 176, 90, 17, 134, 99, 82, 16, 219, 159, 21, 246, 143, 187, 242, 130, 233, 80, 171, 82, 218, 208, 229, 123, 86, 234, 239, 105, 188, 172, 122, 96, 232, 243, 65, 116, 135, 134, 71, 195, 244, 182, 17, 6, 153, 197, 50, 218, 111, 114, 235, 4, 53, 99, 155, 101, 114, 146, 69, 238, 77, 200, 178, 39, 25, 68, 169, 74, 113, 100, 107, 110, 4, 98, 137, 81, 8, 143, 218, 175, 175, 138, 209, 18, 7, 123, 117, 175, 183, 46, 155, 232, 250, 113, 235, 181, 70, 62, 160, 223, 165, 104, 81, 171, 87, 151, 254, 159, 70, 79, 9, 71, 248, 251, 137, 220, 172, 232, 126, 106, 149, 139, 205, 200, 49, 204, 245, 137, 13, 159, 4, 210, 116, 46, 203, 113, 41, 177, 111, 187, 95, 65, 164, 102, 18, 72, 159, 226, 39, 94, 254, 127, 66, 22, 227, 133, 97, 233, 52, 188, 39, 152, 100, 89, 71, 35, 33, 111, 161, 111, 65, 220, 87, 10, 93, 110, 199, 235, 30, 84, 12, 227, 38, 109, 49, 229, 213, 233, 250, 41, 139, 115, 14, 165, 144, 92, 56, 106, 204, 46, 99, 169, 14, 101, 82, 218, 81, 18, 10, 174, 181, 113, 232, 91, 50, 81, 201, 145, 116, 135, 18, 134, 114, 47, 164, 163, 166, 198, 169, 235, 113, 252, 118, 166, 186, 138, 138, 154, 20, 122, 218, 138, 106, 136, 81, 235, 170, 235, 43, 226, 54, 150, 98, 194, 129, 2, 35, 108, 198, 137, 22, 59, 73, 75, 108, 176, 195, 72, 109, 180, 36, 146, 148, 145, 16, 237, 0, 113, 151, 86, 117, 175, 81, 245, 173, 227, 123, 168, 54, 50, 90, 42, 91, 221, 170, 39, 49, 80, 164, 141, 234, 149, 42, 41, 49, 196, 56, 0, 52, 135, 190, 121, 120, 6, 105, 100, 97, 221, 159, 102, 213, 105, 117, 186, 88, 4, 26, 250, 203, 16, 32, 120, 146, 159, 142, 121, 136, 255, 0, 52, 210, 145, 220, 231, 159, 48, 163, 132, 94, 72, 68, 81, 229, 148, 195, 105, 105, 141, 93, 186, 169, 19, 143, 237, 12, 42, 147, 47, 174, 104, 214, 184, 135, 103, 20, 142, 117, 115, 142, 17, 18, 222, 172, 178, 100, 218, 176, 174, 117, 125, 169, 247, 21, 14, 75, 62, 233, 37, 36, 233, 45, 36, 68, 61, 22, 15, 129, 225, 186, 219, 28, 129, 136, 224, 120, 221, 78, 43, 142, 86, 163, 182, 37, 85, 52, 54, 161, 198, 74, 149, 199, 184, 251, 164, 218, 73, 114, 101, 190, 100, 75, 145, 46, 66, 157, 147, 33, 127, 125, 231, 86, 175, 35, 214, 128, 241, 159, 111, 181, 147, 91, 22, 154, 77, 158, 193, 245, 16, 88, 107, 80, 234, 222, 229, 134, 215, 67, 101, 199, 107, 216, 138, 145, 144, 214, 142, 118, 82, 67, 74, 177, 135, 32, 158, 91, 204, 243, 150, 180, 105, 45, 183, 190, 181, 42, 173, 233, 34, 16, 73, 113, 96, 136, 90, 120, 84, 242, 177, 61, 128, 158, 43, 70, 164, 2, 17, 156, 168, 35, 200, 96, 0, 7, 157, 153, 88, 0, 0, 198, 0, 0, 49, 128, 0, 12, 96, 0, 3, 24, 0, 1, 121, 62, 81, 247, 252, 156, 96, 0, 4, 177, 128, 0, 22, 95, 230, 63, 111, 192, 198, 0, 0, 71, 24, 0, 0, 198, 6, 199, 28, 109, 150, 220, 117, 215, 16, 211, 77, 33, 78, 58, 235, 138, 74, 27, 109, 180, 36, 212, 183, 28, 90, 140, 146, 132, 33, 36, 106, 82, 148, 100, 148, 164, 140, 204, 200, 136, 204, 111, 29, 22, 81, 255, 0, 12, 228, 95, 248, 43, 127, 254, 124, 129, 36, 94, 231, 69, 39, 142, 230, 85, 231, 233, 201, 3, 156, 161, 242, 4, 253, 1, 202, 48, 231, 87, 29, 41, 52, 181, 180, 239, 83, 157, 61, 54, 227, 107, 83, 110, 54, 230, 232, 214, 232, 91, 107, 65, 154, 86, 133, 161, 89, 41, 41, 43, 74, 136, 210, 164, 168, 136, 210, 100, 100, 100, 70, 66, 172, 97, 153, 238, 13, 177, 233, 147, 145, 235, 220, 211, 19, 207, 49, 229, 73, 126, 18, 111, 176, 204, 138, 159, 40, 166, 84, 200, 166, 146, 147, 17, 54, 148, 115, 39, 65, 57, 49, 205, 104, 39, 216, 39, 205, 214, 77, 73, 39, 16, 158, 226, 231, 16, 15, 78, 121, 222, 149, 209, 181, 150, 200, 71, 92, 40, 215, 170, 217, 199, 183, 50, 37, 80, 30, 89, 95, 177, 37, 207, 252, 143, 248, 21, 69, 16, 153, 115, 18, 140, 245, 122, 98, 253, 168, 86, 157, 169, 121, 69, 43, 220, 247, 13, 69, 237, 155, 102, 121, 6, 214, 111, 142, 140, 58, 61, 232, 134, 235, 127, 104, 42, 186, 207, 232, 247, 94, 187, 43, 156, 90, 159, 9, 110, 234, 50, 50, 252, 182, 218, 212, 168, 209, 91, 88, 140, 175, 178, 194, 44, 155, 59, 198, 19, 30, 84, 153, 136, 109, 136, 113, 163, 201, 156, 150, 93, 109, 146, 67, 191, 89, 235, 63, 103, 80, 116, 253, 241, 166, 212, 210, 235, 59, 219, 41, 118, 181, 53, 84, 173, 222, 210, 65, 83, 71, 179, 158, 210, 114, 145, 235, 239, 165, 134, 51, 205, 35, 144, 34, 94, 197, 86, 84, 149, 153, 149, 99, 36, 234, 186, 110, 161, 147, 97, 1, 185, 106, 109, 52, 21, 150, 164, 182, 166, 138, 189, 233, 38, 189, 89, 35, 35, 150, 179, 92, 196, 162, 52, 85, 228, 185, 238, 36, 18, 128, 3, 221, 146, 60, 2, 6, 46, 125, 74, 122, 240, 214, 26, 199, 21, 234, 155, 115, 116, 105, 132, 213, 116, 181, 148, 75, 164, 147, 38, 70, 47, 157, 74, 153, 180, 113, 60, 87, 38, 150, 212, 108, 127, 33, 190, 172, 145, 45, 248, 169, 102, 212, 164, 195, 248, 104, 122, 162, 185, 7, 34, 108, 24, 214, 14, 85, 185, 50, 57, 57, 122, 125, 112, 245, 197, 35, 167, 30, 142, 169, 58, 172, 212, 84, 216, 230, 197, 175, 202, 102, 107, 169, 24, 203, 55, 207, 217, 70, 168, 179, 198, 246, 4, 111, 159, 10, 208, 151, 92, 236, 121, 168, 119, 224, 59, 29, 230, 155, 82, 147, 218, 165, 169, 15, 32, 148, 147, 73, 107, 83, 251, 62, 234, 72, 47, 106, 104, 44, 90, 251, 114, 110, 182, 82, 105, 233, 79, 67, 107, 175, 191, 79, 248, 180, 15, 18, 79, 174, 179, 110, 165, 137, 97, 171, 110, 3, 52, 70, 72, 231, 100, 229, 28, 72, 133, 227, 5, 135, 160, 157, 65, 173, 120, 45, 216, 47, 98, 37, 165, 89, 110, 78, 147, 213, 177, 4, 222, 232, 225, 138, 88, 142, 25, 99, 71, 150, 39, 8, 193, 89, 3, 124, 64, 171, 112, 220, 12, 145, 64, 17, 245, 210, 127, 80, 189, 90, 110, 229, 229, 89, 254, 220, 233, 222, 135, 78, 104, 251, 44, 30, 46, 87, 166, 236, 223, 201, 161, 217, 103, 57, 58, 159, 226, 67, 78, 229, 20, 145, 175, 230, 73, 162, 139, 103, 84, 164, 90, 194, 139, 34, 154, 11, 241, 217, 117, 182, 158, 148, 251, 134, 149, 57, 96, 58, 147, 212, 235, 175, 94, 164, 53, 214, 127, 148, 244, 249, 209, 222, 5, 153, 76, 213, 217, 78, 67, 95, 151, 220, 216, 102, 18, 42, 241, 247, 107, 107, 25, 106, 69, 117, 46, 45, 81, 97, 144, 86, 222, 228, 185, 124, 168, 173, 206, 155, 61, 168, 50, 62, 36, 70, 138, 190, 51, 81, 228, 77, 156, 203, 43, 173, 111, 103, 251, 251, 83, 108, 226, 138, 109, 24, 77, 60, 154, 248, 118, 118, 228, 234, 13, 68, 116, 41, 207, 179, 241, 69, 120, 36, 188, 246, 197, 83, 50, 203, 12, 149, 230, 138, 57, 93, 226, 178, 4, 44, 189, 231, 140, 73, 191, 161, 18, 86, 118, 91, 165, 174, 45, 135, 171, 10, 208, 182, 211, 204, 149, 124, 51, 43, 172, 2, 19, 40, 78, 217, 22, 68, 118, 80, 173, 31, 50, 3, 218, 9, 204, 128, 192, 88, 223, 65, 189, 103, 64, 235, 71, 166, 182, 55, 154, 49, 55, 241, 139, 170, 155, 140, 131, 22, 204, 49, 58, 199, 95, 186, 40, 217, 38, 55, 10, 13, 148, 134, 104, 220, 246, 91, 151, 61, 139, 42, 219, 74, 201, 149, 236, 41, 147, 150, 75, 152, 80, 213, 238, 60, 217, 173, 118, 69, 152, 245, 237, 234, 43, 87, 130, 108, 142, 161, 96, 116, 77, 135, 97, 122, 11, 89, 205, 180, 122, 198, 147, 110, 101, 247, 24, 198, 232, 184, 198, 41, 95, 105, 22, 55, 240, 168, 86, 184, 145, 97, 161, 136, 238, 27, 202, 97, 234, 217, 42, 121, 109, 62, 213, 81, 219, 155, 39, 221, 98, 183, 66, 245, 5, 141, 166, 203, 76, 233, 175, 163, 127, 85, 122, 45, 109, 168, 182, 91, 109, 109, 0, 219, 11, 18, 188, 53, 233, 213, 123, 54, 81, 110, 77, 97, 209, 188, 63, 118, 50, 199, 217, 219, 35, 186, 70, 202, 205, 114, 77, 229, 8, 234, 214, 184, 173, 98, 120, 45, 192, 214, 98, 106, 213, 44, 216, 34, 186, 32, 146, 73, 165, 88, 163, 99, 18, 70, 172, 59, 188, 78, 214, 238, 229, 85, 75, 2, 4, 224, 128, 178, 154, 158, 166, 243, 93, 209, 209, 60, 46, 168, 250, 104, 193, 160, 100, 153, 149, 254, 190, 147, 154, 227, 26, 215, 49, 118, 90, 14, 202, 210, 142, 68, 184, 217, 30, 26, 82, 234, 31, 140, 227, 182, 233, 153, 85, 109, 89, 71, 41, 181, 183, 22, 202, 107, 80, 148, 180, 48, 196, 206, 90, 177, 41, 190, 175, 50, 179, 45, 41, 211, 36, 189, 23, 128, 99, 185, 175, 83, 253, 64, 231, 174, 235, 187, 45, 57, 99, 58, 213, 17, 112, 27, 172, 118, 66, 97, 230, 114, 175, 27, 134, 182, 174, 163, 87, 196, 92, 154, 235, 26, 199, 164, 169, 164, 170, 146, 99, 150, 50, 156, 237, 133, 33, 34, 154, 254, 133, 234, 93, 155, 217, 142, 173, 20, 50, 81, 218, 207, 166, 217, 71, 37, 154, 241, 62, 178, 229, 106, 246, 173, 76, 219, 16, 242, 1, 82, 162, 67, 70, 225, 55, 101, 34, 177, 122, 179, 70, 37, 50, 47, 105, 172, 251, 189, 117, 113, 27, 73, 51, 118, 207, 85, 46, 86, 101, 138, 70, 22, 97, 146, 72, 162, 65, 95, 133, 253, 89, 89, 231, 132, 8, 87, 153, 120, 149, 24, 175, 107, 115, 147, 134, 3, 226, 173, 251, 67, 236, 232, 31, 107, 156, 67, 181, 248, 81, 126, 211, 58, 244, 188, 136, 7, 97, 236, 55, 243, 78, 18, 36, 45, 215, 211, 16, 228, 251, 191, 25, 47, 184, 227, 201, 103, 176, 156, 90, 214, 74, 81, 253, 163, 81, 35, 130, 71, 32, 240, 72, 228, 121, 131, 193, 227, 144, 127, 112, 125, 71, 242, 207, 84, 30, 64, 62, 99, 145, 207, 7, 212, 127, 207, 243, 192, 0, 10, 101, 112, 0, 1, 140, 0, 0, 188, 159, 40, 251, 254, 78, 48, 0, 2, 88, 192, 0, 11, 47, 243, 31, 183, 224, 99, 0, 0, 35, 140, 0, 0, 99, 3, 163, 201, 200, 207, 26, 200, 72, 136, 204, 206, 142, 216, 136, 136, 185, 51, 51, 129, 32, 136, 136, 139, 201, 153, 159, 208, 135, 120, 2, 72, 221, 142, 173, 199, 61, 172, 173, 199, 167, 61, 164, 30, 57, 243, 227, 158, 62, 153, 66, 57, 4, 125, 71, 25, 143, 79, 162, 6, 143, 214, 57, 151, 78, 251, 126, 203, 102, 106, 12, 15, 42, 186, 103, 127, 229, 49, 97, 207, 206, 181, 254, 61, 121, 104, 213, 114, 104, 113, 183, 91, 141, 26, 86, 65, 83, 42, 91, 112, 146, 251, 175, 184, 134, 90, 112, 152, 39, 92, 117, 105, 79, 122, 214, 103, 122, 126, 168, 61, 43, 92, 238, 30, 133, 179, 45, 79, 161, 48, 186, 168, 150, 216, 173, 190, 63, 156, 227, 90, 255, 0, 17, 170, 175, 164, 137, 108, 140, 122, 205, 217, 214, 180, 244, 181, 21, 204, 68, 128, 86, 18, 226, 203, 155, 50, 44, 70, 154, 109, 83, 167, 54, 150, 81, 223, 38, 66, 9, 82, 134, 3, 232, 55, 250, 255, 0, 101, 119, 173, 23, 172, 224, 136, 193, 36, 59, 138, 187, 138, 186, 187, 22, 165, 185, 78, 25, 106, 248, 97, 97, 36, 173, 112, 232, 226, 54, 86, 100, 138, 23, 9, 35, 4, 42, 126, 44, 240, 235, 232, 107, 67, 165, 58, 103, 127, 17, 94, 156, 180, 229, 178, 145, 36, 51, 58, 75, 221, 203, 129, 204, 156, 50, 247, 2, 3, 51, 142, 84, 18, 8, 242, 204, 98, 183, 175, 168, 150, 39, 212, 183, 68, 112, 186, 39, 212, 186, 167, 106, 222, 117, 97, 158, 98, 120, 14, 166, 200, 117, 99, 248, 29, 172, 79, 200, 121, 216, 204, 234, 15, 202, 107, 91, 41, 47, 180, 77, 252, 54, 88, 161, 119, 224, 41, 40, 105, 117, 202, 157, 30, 125, 186, 107, 89, 129, 41, 9, 172, 254, 166, 26, 114, 251, 78, 122, 57, 234, 157, 49, 55, 222, 187, 200, 245, 210, 116, 30, 49, 118, 186, 230, 158, 152, 135, 110, 170, 97, 155, 55, 106, 134, 150, 208, 167, 87, 94, 205, 151, 202, 106, 26, 148, 146, 50, 134, 134, 59, 201, 39, 201, 12, 130, 83, 29, 132, 188, 185, 9, 101, 164, 200, 117, 41, 67, 175, 165, 180, 19, 206, 33, 6, 102, 132, 45, 210, 73, 45, 105, 65, 168, 205, 41, 82, 140, 146, 102, 124, 17, 114, 99, 148, 122, 241, 123, 68, 163, 175, 187, 211, 242, 105, 58, 104, 208, 214, 233, 186, 165, 186, 190, 205, 11, 27, 118, 189, 62, 195, 107, 34, 65, 17, 141, 111, 29, 117, 113, 78, 148, 48, 65, 224, 214, 132, 84, 177, 42, 7, 121, 39, 154, 195, 145, 198, 35, 116, 236, 214, 32, 190, 183, 182, 34, 197, 155, 154, 177, 168, 138, 120, 233, 136, 18, 189, 85, 46, 221, 198, 15, 120, 147, 198, 157, 228, 126, 249, 92, 203, 26, 158, 2, 198, 145, 174, 82, 108, 21, 10, 78, 139, 195, 155, 52, 26, 84, 157, 77, 143, 32, 208, 105, 50, 82, 84, 88, 124, 68, 246, 26, 120, 228, 140, 143, 199, 111, 28, 145, 248, 224, 68, 135, 161, 60, 9, 176, 58, 112, 222, 45, 78, 133, 42, 19, 174, 117, 15, 154, 56, 134, 229, 70, 122, 59, 142, 54, 170, 138, 82, 75, 137, 67, 200, 66, 148, 131, 50, 50, 74, 136, 141, 38, 100, 100, 71, 224, 78, 64, 13, 82, 183, 82, 123, 190, 139, 169, 116, 158, 228, 174, 58, 139, 97, 169, 188, 108, 248, 253, 166, 161, 213, 207, 118, 97, 16, 139, 193, 62, 56, 159, 223, 59, 123, 252, 72, 124, 63, 15, 158, 217, 59, 248, 95, 86, 77, 112, 123, 218, 219, 190, 49, 31, 195, 171, 219, 128, 71, 225, 243, 227, 123, 210, 87, 94, 226, 253, 227, 179, 179, 192, 231, 142, 215, 238, 238, 227, 149, 227, 207, 30, 239, 70, 36, 108, 124, 127, 211, 99, 169, 105, 26, 250, 165, 227, 218, 48, 246, 102, 241, 176, 215, 53, 150, 81, 22, 209, 78, 205, 88, 211, 216, 27, 184, 131, 14, 71, 148, 134, 201, 216, 242, 178, 22, 160, 176, 162, 89, 123, 78, 17, 173, 10, 62, 59, 184, 140, 24, 119, 122, 27, 112, 116, 205, 179, 149, 188, 111, 122, 191, 222, 126, 160, 172, 80, 236, 105, 25, 14, 186, 200, 167, 236, 87, 177, 204, 14, 234, 186, 194, 219, 225, 94, 78, 173, 66, 153, 199, 233, 49, 12, 103, 31, 110, 13, 173, 130, 45, 36, 52, 219, 50, 219, 122, 154, 45, 74, 77, 112, 96, 30, 105, 227, 137, 49, 216, 67, 174, 62, 134, 90, 67, 239, 18, 18, 243, 201, 109, 9, 117, 212, 182, 70, 77, 165, 199, 9, 36, 181, 146, 8, 204, 144, 74, 81, 146, 72, 207, 183, 142, 76, 111, 149, 189, 172, 36, 27, 190, 163, 222, 127, 0, 150, 27, 59, 205, 221, 29, 220, 82, 107, 247, 2, 141, 202, 222, 230, 178, 35, 106, 172, 223, 254, 23, 52, 214, 245, 54, 188, 65, 37, 154, 245, 215, 93, 44, 146, 69, 25, 241, 248, 28, 15, 14, 94, 149, 50, 82, 215, 82, 247, 228, 104, 232, 210, 158, 147, 45, 138, 126, 60, 82, 120, 197, 8, 181, 20, 30, 244, 137, 13, 184, 187, 123, 98, 146, 67, 97, 85, 89, 190, 14, 124, 242, 29, 122, 21, 221, 152, 239, 76, 30, 145, 26, 171, 111, 103, 144, 108, 156, 133, 131, 225, 57, 180, 214, 241, 232, 177, 220, 69, 221, 253, 211, 219, 23, 48, 106, 143, 27, 174, 142, 227, 102, 180, 207, 187, 176, 118, 52, 54, 92, 117, 191, 106, 50, 30, 84, 199, 248, 140, 195, 138, 40, 155, 212, 90, 247, 120, 116, 107, 153, 232, 207, 85, 29, 135, 134, 86, 217, 81, 117, 3, 177, 179, 52, 238, 45, 121, 81, 137, 118, 73, 212, 184, 22, 219, 154, 202, 177, 220, 142, 149, 182, 163, 57, 34, 182, 214, 92, 83, 177, 179, 247, 217, 106, 25, 170, 51, 149, 184, 228, 199, 141, 119, 214, 38, 172, 188, 192, 121, 212, 61, 164, 197, 66, 223, 84, 88, 94, 158, 134, 100, 235, 45, 198, 198, 215, 80, 67, 53, 242, 125, 227, 71, 176, 91, 199, 248, 21, 105, 5, 32, 106, 60, 86, 47, 61, 163, 180, 69, 105, 164, 154, 189, 81, 238, 209, 197, 28, 177, 205, 126, 126, 155, 51, 195, 171, 136, 223, 116, 109, 53, 58, 209, 107, 221, 32, 3, 195, 189, 95, 192, 2, 244, 171, 227, 126, 168, 104, 235, 136, 189, 213, 136, 69, 73, 102, 253, 70, 118, 86, 78, 186, 162, 218, 186, 250, 166, 174, 242, 162, 91, 51, 234, 110, 107, 161, 91, 85, 206, 142, 162, 91, 19, 107, 172, 99, 53, 50, 20, 182, 22, 94, 22, 204, 152, 207, 52, 243, 106, 47, 10, 66, 210, 101, 224, 199, 98, 0, 62, 90, 193, 59, 143, 106, 240, 188, 158, 208, 196, 51, 5, 231, 200, 22, 10, 161, 136, 30, 68, 133, 80, 79, 152, 81, 233, 155, 56, 231, 129, 207, 4, 241, 230, 64, 224, 19, 251, 240, 57, 60, 15, 229, 201, 227, 234, 112, 0, 2, 156, 15, 160, 254, 131, 43, 128, 0, 14, 7, 208, 127, 65, 140, 0, 0, 180, 196, 130, 64, 36, 15, 160, 242, 253, 177, 128, 0, 20, 228, 253, 79, 245, 56, 192, 0, 10, 191, 204, 126, 223, 129, 140, 0, 0, 142, 48, 0, 1, 140, 0, 0, 99, 0, 0, 47, 39, 202, 62, 255, 0, 147, 140, 0, 0, 150, 48, 0, 1, 140, 0, 0, 99, 0, 0, 24, 192, 0, 6, 48, 0, 1, 140, 0, 0, 99, 0, 0, 44, 191, 204, 126, 223, 129, 140, 0, 0, 142, 48, 0, 2, 79, 243, 31, 183, 224, 99, 0, 0, 35, 140, 0, 0, 99, 0, 0, 24, 192, 0, 11, 201, 242, 143, 191, 228, 227, 0, 0, 37, 140, 0, 0, 99, 0, 0, 24, 192, 0, 6, 48, 0, 1, 140, 0, 0, 99, 0, 0, 24, 192, 0, 11, 47, 243, 31, 183, 224, 99, 0, 0, 35, 140, 255, 217}))
			}

			r := &RuntimeImpl{
				llmFact:     ttFields.llmFact,
				idGen:       ttFields.idGen,
				runtimeRepo: ttFields.runtimeRepo,
				runtimeCfg:  ttFields.runtimeCfg,
			}

			got, err := r.HandleMsgsPreCallModel(tt.args.ctx, tt.args.model, tt.args.msgs)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			assert.Equal(t, tt.wantMimeType, got[0].MultiModalContent[0].ImageURL.MIMEType)
			assert.Equal(t, tt.wantBase64, got[0].MultiModalContent[0].ImageURL.URL)
		})
	}
}

type mockIStreamReader struct {
	callTimes int
	recv      func(callTimes int) (*entity.Message, error)
}

func (m *mockIStreamReader) Recv() (msg *entity.Message, err error) {
	m.callTimes++
	return m.recv(m.callTimes)
}

func TestRuntimeImpl_Stream(t *testing.T) {
	var opts []entity.Option
	opts = append(opts, entity.WithTools([]*entity.ToolInfo{
		{
			Name:        "get_weather",
			Desc:        "Determine weather in my location",
			ToolDefType: entity.ToolDefTypeOpenAPIV3,
			Def:         "{\"type\":\"object\",\"properties\":{\"location\":{\"type\":\"string\",\"description\":\"The city and state e.g. San Francisco, CA\"},\"unit\":{\"type\":\"string\",\"enum\":[\"c\",\"f\"]}},\"required\":[\"location\"]}",
		},
	}))
	multimodalInput := []*entity.Message{
		{
			Role: entity.RoleUser,
			MultiModalContent: []*entity.ChatMessagePart{
				{
					Type: entity.ChatMessagePartTypeText,
					Text: "there is text",
				},
				{
					Type: entity.ChatMessagePartTypeImageURL,
					ImageURL: &entity.ChatMessageImageURL{
						URL:      "there is url",
						Detail:   entity.ImageURLDetailHigh,
						MIMEType: "image/png",
					},
				},
			},
		},
	}
	model := &entity.Model{
		ID:          0,
		WorkspaceID: 0,
		Name:        "model supports function call and multimodal",
		Desc:        "",
		Ability: &entity.Ability{
			FunctionCall: true,
			MultiModal:   true,
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
		Frame:    entity.FrameEino,
		Protocol: entity.ProtocolArk,
		ProtocolConfig: &entity.ProtocolConfig{
			APIKey: "your api key",
			Model:  "your model",
		},
	}
	type fields struct {
		llmFact     llmfactory.IFactory
		idGen       idgen.IIDGenerator
		runtimeRepo repo.IRuntimeRepo
		runtimeCfg  conf.IConfigRuntime
	}
	type args struct {
		ctx   context.Context
		model *entity.Model
		input []*entity.Message
		opts  []entity.Option
	}
	tests := []struct {
		name             string
		fieldsGetter     func(ctrl *gomock.Controller) fields
		args             args
		wantFinalContent string
		wantErr          error
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				factMock := llmfactorymocks.NewMockIFactory(ctrl)
				llmMock := llmifacemocks.NewMockILLM(ctrl)
				factMock.EXPECT().CreateLLM(gomock.Any(), gomock.Any(), gomock.Any()).Return(llmMock, nil)
				streamMock := &mockIStreamReader{
					callTimes: 0,
					recv: func(callTimes int) (*entity.Message, error) {
						switch callTimes {
						case 1:
							return &entity.Message{
								Role:    entity.RoleAssistant,
								Content: "there ",
							}, nil
						case 2:
							return &entity.Message{
								Role:    entity.RoleAssistant,
								Content: "is ",
							}, nil
						case 3:
							return &entity.Message{
								Role:    entity.RoleAssistant,
								Content: "content",
							}, nil
						default:
							return nil, io.EOF
						}
					},
				}
				llmMock.EXPECT().Stream(gomock.Any(), gomock.Any(), gomock.Any()).Return(streamMock, nil)
				repoMock := llmrepomocks.NewMockIRuntimeRepo(ctrl)
				idgenMock := idgenmocks.NewMockIIDGenerator(ctrl)
				// idgenMock.EXPECT().GenID(gomock.Any()).Return(int64(1), nil)
				cfgMock := llmconfmocks.NewMockIConfigRuntime(ctrl)
				// cfgMock.EXPECT().NeedCvtURLToBase64().Return(true)
				return fields{
					llmFact:     factMock,
					idGen:       idgenMock,
					runtimeRepo: repoMock,
					runtimeCfg:  cfgMock,
				}
			},
			args: args{
				ctx:   context.Background(),
				model: model,
				input: multimodalInput,
				opts:  opts,
			},
			wantFinalContent: "there is content",
			wantErr:          nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ttFields := tt.fieldsGetter(ctrl)
			r := &RuntimeImpl{
				llmFact:     ttFields.llmFact,
				idGen:       ttFields.idGen,
				runtimeRepo: ttFields.runtimeRepo,
				runtimeCfg:  ttFields.runtimeCfg,
			}
			got, err := r.Stream(tt.args.ctx, tt.args.model, tt.args.input, tt.args.opts...)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			var content string
			for {
				msg, err := got.Recv()
				if err == io.EOF {
					break
				}
				assert.Nil(t, err)
				content += msg.Content
			}
			assert.Equal(t, tt.wantFinalContent, content)
		})
	}
}
