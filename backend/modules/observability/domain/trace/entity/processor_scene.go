// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type ProcessorScene string

const (
	SceneGetTrace        ProcessorScene = "get_trace"
	SceneListSpans       ProcessorScene = "list_spans"
	SceneAdvanceInfo     ProcessorScene = "advance_info"
	SceneIngestTrace     ProcessorScene = "ingest_trace"
	SceneSearchTraceOApi ProcessorScene = "search_trace_oapi"
	SceneListSpansOApi   ProcessorScene = "list_spans_oapi"
	SceneTraceChat       ProcessorScene = "trace_chat"
	SceneThreadChat      ProcessorScene = "thread_chat"
	SceneThreadStat      ProcessorScene = "thread_stat"
)
