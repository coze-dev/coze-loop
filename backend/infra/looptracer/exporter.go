// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package looptracer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"time"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/coze-dev/cozeloop-go/entity"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/infra/looptracer/rpc"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/foundation/file"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	CtxKeyEnv = "K_ENV"
	XttEnv    = "x_tt_env"
)

type MultiSpaceSpanExporter struct{}

func (e *MultiSpaceSpanExporter) ExportSpans(ctx context.Context, spans []*entity.UploadSpan) error {
	finalSpans := make([]*span.InputSpan, 0, len(spans))
	for _, uploadSpan := range spans {
		if uploadSpan == nil {
			continue
		}
		finalSpans = append(finalSpans, &span.InputSpan{
			StartedAtMicros:  uploadSpan.StartedATMicros,
			SpanID:           uploadSpan.SpanID,
			ParentID:         uploadSpan.ParentID,
			TraceID:          uploadSpan.TraceID,
			Duration:         uploadSpan.DurationMicros,
			CallType:         nil,
			WorkspaceID:      uploadSpan.WorkspaceID,
			SpanName:         uploadSpan.SpanName,
			SpanType:         uploadSpan.SpanType,
			Method:           "",
			StatusCode:       uploadSpan.StatusCode,
			Input:            uploadSpan.Input,
			Output:           uploadSpan.Output,
			ObjectStorage:    lo.ToPtr(uploadSpan.ObjectStorage),
			SystemTagsString: uploadSpan.SystemTagsString,
			SystemTagsLong:   uploadSpan.SystemTagsLong,
			SystemTagsDouble: uploadSpan.SystemTagsDouble,
			TagsString:       uploadSpan.TagsString,
			TagsLong:         uploadSpan.TagsLong,
			TagsDouble:       uploadSpan.TagsDouble,
			TagsBool:         uploadSpan.TagsBool,
			TagsBytes:        nil,
			DurationMicros:   lo.ToPtr(uploadSpan.DurationMicros),
		})
	}

	req := &trace.IngestTracesRequest{
		Spans: finalSpans,
	}

	if env := os.Getenv(XttEnv); env != "" {
		ctx = context.WithValue(ctx, CtxKeyEnv, env) //nolint:staticcheck,SA1029
	}
	resp, err := rpc.GetLoopTracerHandler().LocalTraceService.IngestTracesInner(ctx, req)
	if err != nil {
		logs.CtxError(ctx, "export spans fail, err:[%v], retry later", err)
		return err
	}
	if resp == nil {
		logs.CtxError(ctx, "export spans fail, resp is nil, retry later")
		return errorx.New("export spans fail, resp is nil, retry later")
	}
	if resp.GetCode() != 0 {
		logs.CtxError(ctx, "export spans fail, resp code:[%v], retry later", resp.GetCode())
		return errorx.New("export spans fail, resp code:[%v], retry later", resp.GetCode())
	}

	return nil
}

func (e *MultiSpaceSpanExporter) ExportFiles(ctx context.Context, files []*entity.UploadFile) error {
	for _, file := range files {
		if file == nil {
			continue
		}
		logs.CtxDebug(ctx, "uploadFile start, file name: %s", file.Name)
		err := uploadFile(ctx, file.TosKey, bytes.NewReader([]byte(file.Data)), map[string]string{"workspace_id": file.SpaceID})
		if err != nil {
			logs.CtxError(ctx, "export files[%s] fail, err:[%v], retry later", file.TosKey, err)
			return err
		}
	}

	return nil
}

func uploadFile(ctx context.Context, fileName string, reader io.Reader, form map[string]string) error {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return errorx.Wrapf(err, "create form file: %v", fileName)
	}

	if _, err = io.Copy(part, reader); err != nil {
		return errorx.Wrapf(err, "copy file content")
	}

	for key, value := range form {
		if err := writer.WriteField(key, value); err != nil {
			return errorx.Wrapf(err, "write field %s", key)
		}
	}

	if err := writer.Close(); err != nil {
		return errorx.Wrapf(err, "close multipart writer")
	}

	contentType := writer.FormDataContentType()
	resp, err := rpc.GetLoopTracerHandler().LocalFileService.UploadLoopFileInner(ctx, &file.UploadLoopFileInnerRequest{
		ContentType: contentType,
		Body:        body.Bytes(),
	})
	logs.CtxDebug(ctx, "span client upload file, content type:%s, response: %#v", contentType, resp)
	if err != nil {
		logger.CtxErrorf(ctx, fmt.Sprintf("http client UploadFile failed, err: %v", err))
		return errorx.Wrapf(err, "http client UploadFile failed")
	}
	if resp == nil || resp.GetCode() != 0 {
		logger.CtxErrorf(ctx, fmt.Sprintf("http client UploadFile failed, resp: %#v", resp))
		return errorx.Wrapf(err, "http client UploadFile failed, resp: %#v", resp)
	}

	return nil
}
