// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAnnotation(t *testing.T) {
	span := &Span{
		StartTime:   time.Now().UnixMicro(),
		SpanID:      "123",
		ParentID:    "0",
		TraceID:     "1234",
		WorkspaceID: "12345",
	}
	strAnnotation, err := span.BuildFeedback(AnnotationTypeManualFeedback, "123", NewStringValue("123"), "", "user1", false)
	if err != nil {
		t.Fatal(err)
	}
	numAnnotation, err := span.BuildFeedback(AnnotationTypeManualFeedback, "1234", NewLongValue(123), "", "user1", false)
	if err != nil {
		t.Fatal(err)
	}
	boolAnnotation, err := span.BuildFeedback(AnnotationTypeManualFeedback, "123", NewBoolValue(true), "reason", "user2", false)
	if err != nil {
		t.Fatal(err)
	}
	floatAnnotation, err := span.BuildFeedback(AnnotationTypeManualFeedback, "123", NewDoubleValue(123.2), "reason", "", true)
	if err != nil {
		t.Fatal(err)
	}
	floatAnnotation2, err := span.BuildFeedback(AnnotationTypeAutoEvaluate, "123", NewDoubleValue(123.2), "reason", "", true)
	if err != nil {
		t.Fatal(err)
	}
	spans := SpanList{span}
	spans.SetAnnotations(AnnotationList{strAnnotation, boolAnnotation, floatAnnotation, numAnnotation, floatAnnotation2})
	assert.Equal(t, spans.GetUserIDs(), []string{"user1", "user2"})
	assert.Equal(t, spans.GetAnnotationTagIDs(), []string{"123", "1234"})
	assert.Equal(t, spans.GetEvaluatorVersionIDs(), []int64{})
	assert.Equal(t, floatAnnotation2.GetDatasetMetadata(), (*ManualDatasetMetadata)(nil))
}
