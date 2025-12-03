// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"

	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/infra/fileserver"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file"
	"github.com/coze-dev/coze-loop/backend/modules/foundation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/foundation/domain/file/service"
	"github.com/coze-dev/coze-loop/backend/modules/foundation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type FileApplicationImpl struct {
	auth        rpc.IAuthProvider
	fileService service.FileService
}

func NewFileApplication(objectStorage fileserver.BatchObjectStorage, auth rpc.IAuthProvider) file.FileService {
	return &FileApplicationImpl{
		auth:        auth,
		fileService: service.NewFileService(objectStorage),
	}
}

func (p *FileApplicationImpl) UploadFileForServer(ctx context.Context, req *file.UploadFileForServerRequest) (r *file.UploadFileForServerResponse, err error) {
	if req == nil || req.MimeType == "" || len(req.Body) == 0 || req.WorkspaceID == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	spaceID := strconv.FormatInt(req.WorkspaceID, 10)

	// Extract custom mime type mappings and file name from option
	var customMimeTypeExtMap map[string]string
	var fileName string
	if req.Option != nil {
		customMimeTypeExtMap = req.Option.MimeTypeExtMapping
		if req.Option.FileName != nil {
			fileName = *req.Option.FileName
		}
	}

	key, err := p.fileService.UploadFileForServer(ctx, req.MimeType, req.Body, spaceID, customMimeTypeExtMap, fileName)
	if err != nil {
		return nil, err
	}

	return &file.UploadFileForServerResponse{
		Data: &file.FileData{
			Bytes:    lo.ToPtr(int64(len(req.Body))),
			FileName: lo.ToPtr(key),
		},
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (p *FileApplicationImpl) UploadLoopFileInner(ctx context.Context, req *file.UploadLoopFileInnerRequest) (r *file.UploadLoopFileInnerResponse, err error) {
	if req == nil || req.ContentType == "" || len(req.Body) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	form, err := parseMultipartFormData(ctx, req.ContentType, req.Body)
	if err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}
	if form == nil || len(form.File["file"]) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}
	fileHeader := form.File["file"][0]
	spaceID := ""
	if form.Value != nil && len(form.Value["workspace_id"]) > 0 {
		spaceID = form.Value["workspace_id"][0]
	}
	if spaceID == "" {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	key, err := p.fileService.UploadLoopFile(ctx, fileHeader, spaceID)
	if err != nil {
		return nil, err
	}

	return &file.UploadLoopFileInnerResponse{
		Data: &file.FileData{
			Bytes:    lo.ToPtr(int64(len(req.Body))),
			FileName: lo.ToPtr(key),
		},
		BaseResp: base.NewBaseResp(),
	}, nil
}

func (p *FileApplicationImpl) SignUploadFile(ctx context.Context, req *file.SignUploadFileRequest) (r *file.SignUploadFileResponse, err error) {
	if req.WorkspaceID == nil || *req.WorkspaceID == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}

	err = p.auth.CheckWorkspacePermission(ctx, rpc.AuthActionFileUpload, strconv.FormatInt(*req.WorkspaceID, 10))
	if err != nil {
		return nil, err
	}

	signUris, headers, err := p.fileService.SignUploadFile(ctx, req)
	if err != nil {
		return nil, err
	}

	return &file.SignUploadFileResponse{
		Uris:      signUris,
		SignHeads: headers,
		BaseResp:  base.NewBaseResp(),
	}, nil
}

func (p *FileApplicationImpl) SignDownloadFile(ctx context.Context, req *file.SignDownloadFileRequest) (r *file.SignDownloadFileResponse, err error) {
	signURIs, err := p.fileService.SignDownLoadFile(ctx, req)
	if err != nil {
		return nil, err
	}

	return &file.SignDownloadFileResponse{
		Uris:     signURIs,
		BaseResp: base.NewBaseResp(),
	}, nil
}
