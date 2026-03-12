// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// ProcessorScene 定义处理器场景类型
type ProcessorScene string

const (
	SceneGetTrace        ProcessorScene = "get_trace"
	SceneListSpans       ProcessorScene = "list_spans"
	SceneAdvanceInfo     ProcessorScene = "advance_info"
	SceneIngestTrace     ProcessorScene = "ingest_trace"
	SceneSearchTraceOApi ProcessorScene = "search_trace_oapi"
	SceneListSpansOApi   ProcessorScene = "list_spans_oapi"
)
