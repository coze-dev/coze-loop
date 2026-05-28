// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package size_util

import (
	"unsafe"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func SizeOfSpanNew(span *loop_span.Span) int {
	count := 0
	count += int(unsafe.Sizeof(span.StartTime))
	count += SizeOfString(span.SpanID)
	count += SizeOfString(span.ParentID)
	count += SizeOfString(span.LogID)
	count += SizeOfString(span.TraceID)
	count += int(unsafe.Sizeof(span.DurationMicros))
	count += SizeOfString(span.PSM)
	count += SizeOfString(span.CallType)
	count += SizeOfString(span.WorkspaceID)
	count += SizeOfString(span.SpanName)
	count += SizeOfString(span.SpanType)
	count += SizeOfString(span.Method)
	count += int(unsafe.Sizeof(span.StatusCode))
	count += SizeOfString(span.Input)
	count += SizeOfString(span.Output)
	count += SizeOfString(span.ObjectStorage)

	for k, v := range span.SystemTagsString {
		count += SizeOfString(k)
		count += SizeOfString(v)
	}
	for k, v := range span.SystemTagsLong {
		count += SizeOfString(k)
		count += int(unsafe.Sizeof(v))
	}
	for k, v := range span.SystemTagsDouble {
		count += SizeOfString(k)
		count += int(unsafe.Sizeof(v))
	}
	for k, v := range span.TagsString {
		count += SizeOfString(k)
		count += SizeOfString(v)
	}
	for k, v := range span.TagsLong {
		count += SizeOfString(k)
		count += int(unsafe.Sizeof(v))
	}
	for k, v := range span.TagsDouble {
		count += SizeOfString(k)
		count += int(unsafe.Sizeof(v))
	}
	for k, v := range span.TagsByte {
		count += SizeOfString(k)
		count += SizeOfString(v)
	}
	for k, v := range span.TagsBool {
		count += SizeOfString(k)
		count += int(unsafe.Sizeof(v))
	}

	return count
}

func SizeOfString(s string) int {
	return len(s)
}
