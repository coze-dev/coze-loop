// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"
	"testing"
)

func TestEvaluationResourceRequestID(t *testing.T) {
	parent := context.Background()
	if value, known := EvaluationResourceRequestID(parent); value != "" || known {
		t.Fatalf("absent source must be unknown: value=%q known=%v", value, known)
	}
	for _, requestID := range []string{"", "persisted-request"} {
		t.Run(requestID, func(t *testing.T) {
			ctx := WithEvaluationResourceRequestID(parent, requestID)
			value, known := EvaluationResourceRequestID(ctx)
			if !known || value != requestID {
				t.Fatalf("source generation not retained: value=%q known=%v", value, known)
			}
			replaced := WithEvaluationResourceRequestID(ctx, "next-request")
			if value, known := EvaluationResourceRequestID(replaced); !known || value != "next-request" {
				t.Fatal("child must carry the next generation")
			}
			if value, known := EvaluationResourceRequestID(ctx); !known || value != requestID {
				t.Fatal("child must not change its source context")
			}
			if _, known := EvaluationResourceRequestID(parent); known {
				t.Fatal("source injection must not change the parent")
			}
		})
	}
}
