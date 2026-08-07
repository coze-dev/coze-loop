// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type AttrTosProcessor struct {
	fileProvider rpc.IFileProvider
}

func (a *AttrTosProcessor) GetName() string {
	return "AttrTosProcessor"
}

func (a *AttrTosProcessor) Transform(ctx context.Context, spans loop_span.SpanList) (loop_span.SpanList, error) {
	for _, s := range spans {
		if s.ObjectStorage == "" {
			continue
		}
		attrTos := new(loop_span.AttrTos)
		var objectStorage loop_span.ObjectStorage
		objectStorageData := []byte(s.ObjectStorage)
		err := json.Unmarshal(objectStorageData, &objectStorage)
		if err != nil {
			logs.CtxWarn(ctx, "fail to unmarshal span object storage %s", s.ObjectStorage)
			continue
		}
		var tosKeyList []string
		if objectStorage.InputTosKey != "" {
			tosKeyList = append(tosKeyList, objectStorage.InputTosKey)
		}
		if objectStorage.OutputTosKey != "" {
			tosKeyList = append(tosKeyList, objectStorage.OutputTosKey)
		}
		if len(objectStorage.Attachments) != 0 {
			for _, tosKey := range objectStorage.Attachments {
				if tosKey.TosKey != "" {
					tosKeyList = append(tosKeyList, tosKey.TosKey)
				}
			}
		}
		urlMap, err := a.fileProvider.GetDownloadUrls(ctx, s.WorkspaceID, tosKeyList)
		if err != nil {
			logs.CtxWarn(ctx, "fail to sign download request for %s, %v", tosKeyList, err)
		}
		multimodalData := make(map[string]string)
		for _, v := range objectStorage.Attachments {
			if url, exist := urlMap[v.TosKey]; exist {
				multimodalData[v.TosKey] = url
			}
		}
		if objectStorage.InputTosKey != "" {
			attrTos.InputDataURL = urlMap[objectStorage.InputTosKey]
		}
		if objectStorage.OutputTosKey != "" {
			attrTos.OutputDataURL = urlMap[objectStorage.OutputTosKey]
		}
		if len(objectStorage.Attachments) != 0 {
			attrTos.MultimodalData = multimodalData
		}
		s.AttrTos = attrTos
	}
	return spans, nil
}

type AttrTosProcessorFactory struct {
	fileProvider rpc.IFileProvider
}

func (c *AttrTosProcessorFactory) CreateProcessor(ctx context.Context, set Settings) (Processor, error) {
	return &AttrTosProcessor{
		fileProvider: c.fileProvider,
	}, nil
}

func NewAttrTosProcessorFactory(fileProvider rpc.IFileProvider) Factory {
	return &AttrTosProcessorFactory{
		fileProvider: fileProvider,
	}
}
