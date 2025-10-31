// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file"
	service "github.com/coze-dev/coze-loop/backend/modules/foundation/domain/file/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/foundation/domain/file/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/foundation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

type dummyAuth struct{}

func (a dummyAuth) CheckWorkspacePermission(context.Context, string, string) error {
	return nil
}

func TestFileApplicationImpl_UploadFileForServer(t *testing.T) {
	type fields struct {
		fileService service.FileService
	}
	type args struct {
		ctx context.Context
		req *file.UploadFileForServerRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      error
		wantKey      string
	}{
		{
			name: "nil request returns error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: nil,
			},
			wantErr: errorx.NewByCode(errno.CommonInvalidParamCode),
		},
		{
			name: "missing mime type returns error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &file.UploadFileForServerRequest{
					Body:        []byte("data"),
					WorkspaceID: 1,
				},
			},
			wantErr: errorx.NewByCode(errno.CommonInvalidParamCode),
		},
		{
			name: "missing body returns error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &file.UploadFileForServerRequest{
					MimeType:    "text/plain",
					WorkspaceID: 1,
				},
			},
			wantErr: errorx.NewByCode(errno.CommonInvalidParamCode),
		},
		{
			name: "missing workspace id returns error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &file.UploadFileForServerRequest{
					MimeType: "text/plain",
					Body:     []byte("data"),
				},
			},
			wantErr: errorx.NewByCode(errno.CommonInvalidParamCode),
		},
		{
			name: "success without option",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockService := servicemocks.NewMockFileService(ctrl)
				mockService.EXPECT().
					UploadFileForServer(gomock.Any(), "image/png", []byte("img"), "123", gomock.AssignableToTypeOf(map[string]string(nil)), "").
					Return("123/generated.png", nil)
				return fields{fileService: mockService}
			},
			args: args{
				ctx: context.Background(),
				req: &file.UploadFileForServerRequest{
					MimeType:    "image/png",
					Body:        []byte("img"),
					WorkspaceID: 123,
				},
			},
			wantErr: nil,
			wantKey: "123/generated.png",
		},
		{
			name: "success with option and file name",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockService := servicemocks.NewMockFileService(ctrl)
				mockService.EXPECT().
					UploadFileForServer(gomock.Any(), "application/json", []byte("{\"a\":1}"), "45", gomock.Eq(map[string]string{"application/custom": ".cus"}), "name.json").
					Return("45/name.json", nil)
				return fields{fileService: mockService}
			},
			args: args{
				ctx: context.Background(),
				req: &file.UploadFileForServerRequest{
					MimeType:    "application/json",
					Body:        []byte("{\"a\":1}"),
					WorkspaceID: 45,
					Option: &file.UploadFileOption{
						FileName:           lo.ToPtr("name.json"),
						MimeTypeExtMapping: map[string]string{"application/custom": ".cus"},
					},
				},
			},
			wantErr: nil,
			wantKey: "45/name.json",
		},
		{
			name: "service returns error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockService := servicemocks.NewMockFileService(ctrl)
				mockService.EXPECT().
					UploadFileForServer(gomock.Any(), "text/plain", []byte("fail"), "9", gomock.AssignableToTypeOf(map[string]string(nil)), "").
					Return("", assert.AnError)
				return fields{fileService: mockService}
			},
			args: args{
				ctx: context.Background(),
				req: &file.UploadFileForServerRequest{
					MimeType:    "text/plain",
					Body:        []byte("fail"),
					WorkspaceID: 9,
				},
			},
			wantErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ttFields := tt.fieldsGetter(ctrl)
			app := &FileApplicationImpl{
				auth:        dummyAuth{},
				fileService: ttFields.fileService,
			}

			got, err := app.UploadFileForServer(tt.args.ctx, tt.args.req)

			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr != nil {
				assert.Nil(t, got)
				return
			}

			assert.NotNil(t, got)
			assert.NotNil(t, got.Data)
			assert.Equal(t, tt.wantKey, got.GetData().GetFileName())
			assert.Equal(t, int64(len(tt.args.req.Body)), got.GetData().GetBytes())
			assert.NotNil(t, got.BaseResp)
		})
	}
}
