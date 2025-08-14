// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/fileserver"
	fsmocks "github.com/coze-dev/coze-loop/backend/infra/fileserver/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file"
	"github.com/coze-dev/coze-loop/backend/modules/foundation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func createFileHeader(filePath string) (*multipart.FileHeader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "testfile.txt")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	err = req.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		return nil, err
	}

	fileHeaders := req.MultipartForm.File["file"]
	if len(fileHeaders) == 0 {
		return nil, nil
	}

	return fileHeaders[0], nil
}

func TestFileServiceImpl_UploadLoopFile(t *testing.T) {
	type fields struct {
		storage fileserver.BatchObjectStorage
	}
	type args struct {
		ctx        context.Context
		spaceID    string
		fileHeader *multipart.FileHeader
	}

	header, err := createFileHeader("test_file.txt")
	if err != nil {
		return
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         string
		wantErr      error
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := fsmocks.NewMockBatchObjectStorage(ctrl)
				objectStorage.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).Return(nil)
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    "1234567890",
				fileHeader: header,
			},
			want:    "1234567890/testfile.txt",
			wantErr: nil,
		},
		{
			name: "bad_file_no_name",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := fsmocks.NewMockBatchObjectStorage(ctrl)
				objectStorage.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).Return(nil)
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    "1234567890",
				fileHeader: &multipart.FileHeader{},
			},
			want:    "1234567890/testfile.txt",
			wantErr: errorx.NewByCode(errno.CommonInvalidParamCode),
		},
		{
			name: "bad_file_empty",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := fsmocks.NewMockBatchObjectStorage(ctrl)
				objectStorage.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).Return(nil)
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx:     context.Background(),
				spaceID: "1234567890",
				fileHeader: &multipart.FileHeader{
					Filename: "testfile.txt",
				},
			},
			want:    "1234567890/testfile.txt",
			wantErr: errorx.NewByCode(errno.CommonInternalErrorCode),
		},
		{
			name: "upload_fail",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := fsmocks.NewMockBatchObjectStorage(ctrl)
				objectStorage.EXPECT().Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).Return(errors.New("upload fail"))
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx:        context.Background(),
				spaceID:    "1234567890",
				fileHeader: header,
			},
			want:    "1234567890/testfile.txt",
			wantErr: errors.New("upload fail"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			f := &fileService{
				client: ttFields.storage,
			}

			got, err := f.UploadLoopFile(tt.args.ctx, tt.args.fileHeader, tt.args.spaceID)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestFileServiceImpl_SignUploadFile(t *testing.T) {
	_ = os.Setenv("COZE_LOOP_OSS_PROTOCOL", "http")
	_ = os.Setenv("COZE_LOOP_OSS_DOMAIN", "cozeloop-minio")
	_ = os.Setenv("COZE_LOOP_OSS_PORT", "19000")

	type fields struct {
		storage fileserver.BatchObjectStorage
	}
	type args struct {
		ctx context.Context
		req *file.SignUploadFileRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantUris     []string
		wantHeads    []*file.SignHead
		wantErr      error
	}{
		{
			name: "success_localos",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := fsmocks.NewMockBatchObjectStorage(ctrl)
				objectStorage.EXPECT().BatchSignUploadReq(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).
					Return(
						[]string{"http://cozeloop-minio:19000/cozeloop-bucket/123.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioadmin%2F20250521%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20250521T035035Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host&X-Amz-Signature=ccf0af69985ba7ba4392ac2714b9f14cadba8e38cd64480fc25d531288455556"},
						[]http.Header{
							map[string][]string{
								HeaderAccessKeyId:     {"minioadmin"},
								HeaderSecretAccessKey: {"minioadmin"},
								HeaderSessionToken:    {"minioadmin"},
								HeaderExpiredTime:     {"1234567"},
								HeaderCurrentTime:     {"1234120"},
							},
						},
						nil)
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &file.SignUploadFileRequest{
					Keys:   []string{"testfile.txt"},
					Option: &file.SignFileOption{TTL: lo.ToPtr(int64(100))},
				},
			},
			wantUris: []string{"/cozeloop-bucket/123.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioadmin%2F20250521%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20250521T035035Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host&X-Amz-Signature=ccf0af69985ba7ba4392ac2714b9f14cadba8e38cd64480fc25d531288455556"},
			wantHeads: []*file.SignHead{
				{
					CurrentTime:     lo.ToPtr("1234120"),
					ExpiredTime:     lo.ToPtr("1234567"),
					SessionToken:    lo.ToPtr("minioadmin"),
					AccessKeyID:     lo.ToPtr("minioadmin"),
					SecretAccessKey: lo.ToPtr("minioadmin"),
				},
			},
			wantErr: nil,
		},
		{
			name: "success_remoteos",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := fsmocks.NewMockBatchObjectStorage(ctrl)
				objectStorage.EXPECT().BatchSignUploadReq(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).
					Return(
						[]string{"http://cozeloop.minio.com/cozeloop-bucket/123.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioadmin%2F20250521%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20250521T035035Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host&X-Amz-Signature=ccf0af69985ba7ba4392ac2714b9f14cadba8e38cd64480fc25d531288455556"},
						[]http.Header{
							map[string][]string{
								HeaderAccessKeyId:     {"minioadmin"},
								HeaderSecretAccessKey: {"minioadmin"},
								HeaderSessionToken:    {"minioadmin"},
								HeaderExpiredTime:     {"1234567"},
								HeaderCurrentTime:     {"1234120"},
							},
						},
						nil)
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &file.SignUploadFileRequest{
					Keys:   []string{"testfile.txt"},
					Option: &file.SignFileOption{TTL: lo.ToPtr(int64(100))},
				},
			},
			wantUris: []string{"http://cozeloop.minio.com/cozeloop-bucket/123.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioadmin%2F20250521%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20250521T035035Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host&X-Amz-Signature=ccf0af69985ba7ba4392ac2714b9f14cadba8e38cd64480fc25d531288455556"},
			wantHeads: []*file.SignHead{
				{
					CurrentTime:     lo.ToPtr("1234120"),
					ExpiredTime:     lo.ToPtr("1234567"),
					SessionToken:    lo.ToPtr("minioadmin"),
					AccessKeyID:     lo.ToPtr("minioadmin"),
					SecretAccessKey: lo.ToPtr("minioadmin"),
				},
			},
			wantErr: nil,
		},
		{
			name: "sign_fail",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := fsmocks.NewMockBatchObjectStorage(ctrl)
				objectStorage.EXPECT().BatchSignUploadReq(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).
					Return(
						nil,
						nil,
						errors.New("sign fail"))
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &file.SignUploadFileRequest{
					Keys:   []string{"testfile.txt"},
					Option: &file.SignFileOption{TTL: lo.ToPtr(int64(100))},
				},
			},
			wantUris:  nil,
			wantHeads: nil,
			wantErr:   errors.New("sign fail"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			f := &fileService{
				client: ttFields.storage,
			}

			uris, headers, err := f.SignUploadFile(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.Equal(t, tt.wantUris, uris)
				assert.Equal(t, tt.wantHeads, headers)
			}
		})
	}
}

func TestFileServiceImpl_SignDownloadFile(t *testing.T) {
	_ = os.Setenv("COZE_LOOP_OSS_PROTOCOL", "http")
	_ = os.Setenv("COZE_LOOP_OSS_DOMAIN", "cozeloop-minio")
	_ = os.Setenv("COZE_LOOP_OSS_PORT", "19000")

	type fields struct {
		storage fileserver.BatchObjectStorage
	}
	type args struct {
		ctx context.Context
		req *file.SignDownloadFileRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantUris     []string
		wantErr      error
	}{
		{
			name: "success_localos",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := fsmocks.NewMockBatchObjectStorage(ctrl)
				objectStorage.EXPECT().BatchSignDownloadReq(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).
					Return(
						[]string{"http://cozeloop-minio:19000/cozeloop-bucket/123.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioadmin%2F20250521%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20250521T035035Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host&X-Amz-Signature=ccf0af69985ba7ba4392ac2714b9f14cadba8e38cd64480fc25d531288455556"},
						nil,
						nil)
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &file.SignDownloadFileRequest{
					Keys:   []string{"testfile.txt"},
					Option: &file.SignFileOption{TTL: lo.ToPtr(int64(100))},
				},
			},
			wantUris: []string{"/cozeloop-bucket/123.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioadmin%2F20250521%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20250521T035035Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host&X-Amz-Signature=ccf0af69985ba7ba4392ac2714b9f14cadba8e38cd64480fc25d531288455556"},
			wantErr:  nil,
		},
		{
			name: "success_remoteos",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := fsmocks.NewMockBatchObjectStorage(ctrl)
				objectStorage.EXPECT().BatchSignDownloadReq(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).
					Return(
						[]string{"http://cozeloop.minio.com/cozeloop-bucket/123.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioadmin%2F20250521%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20250521T035035Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host&X-Amz-Signature=ccf0af69985ba7ba4392ac2714b9f14cadba8e38cd64480fc25d531288455556"},
						nil,
						nil)
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &file.SignDownloadFileRequest{
					Keys:   []string{"testfile.txt"},
					Option: &file.SignFileOption{TTL: lo.ToPtr(int64(100))},
				},
			},
			wantUris: []string{"http://cozeloop.minio.com/cozeloop-bucket/123.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioadmin%2F20250521%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20250521T035035Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host&X-Amz-Signature=ccf0af69985ba7ba4392ac2714b9f14cadba8e38cd64480fc25d531288455556"},
			wantErr:  nil,
		},
		{
			name: "sign_fail",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := fsmocks.NewMockBatchObjectStorage(ctrl)
				objectStorage.EXPECT().BatchSignDownloadReq(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).
					Return(
						[]string{"http://cozeloop-minio:19000/cozeloop-bucket/123.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioadmin%2F20250521%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20250521T035035Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host&X-Amz-Signature=ccf0af69985ba7ba4392ac2714b9f14cadba8e38cd64480fc25d531288455556"},
						nil,
						errors.New("sign fail"))
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &file.SignDownloadFileRequest{
					Keys:   []string{"testfile.txt"},
					Option: &file.SignFileOption{TTL: lo.ToPtr(int64(100))},
				},
			},
			wantUris: []string{"/cozeloop-bucket/123.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=minioadmin%2F20250521%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20250521T035035Z&X-Amz-Expires=86400&X-Amz-SignedHeaders=host&X-Amz-Signature=ccf0af69985ba7ba4392ac2714b9f14cadba8e38cd64480fc25d531288455556"},
			wantErr:  errors.New("sign fail"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ttFields := tt.fieldsGetter(ctrl)

			f := &fileService{
				client: ttFields.storage,
			}

			uris, err := f.SignDownLoadFile(tt.args.ctx, tt.args.req)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			if tt.wantErr == nil {
				assert.Equal(t, tt.wantUris, uris)
			}
		})
	}
}
