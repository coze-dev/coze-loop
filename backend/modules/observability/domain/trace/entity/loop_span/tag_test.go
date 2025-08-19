// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"context"
	"testing"
)

func TestTagToAttr(t *testing.T) {
	tags := TagSlice{
		NewStringTag("a", "as"),
		NewBoolTag("b", true),
		NewLongTag("c", 123),
		NewDoubleTag("d", 123.456),
	}
	type s struct {
		A string  `json:"a"`
		B bool    `json:"b"`
		C int64   `json:"c"`
		D float64 `json:"d"`
	}
	tmp := new(s)
	if err := tags.toAttr(context.Background(), tmp); err != nil {
		t.Fatal(err)
	}
	if tmp.A != "as" ||
		tmp.B != true ||
		tmp.C != 123 ||
		tmp.D != 123.456 {
		t.Fatal("parse failed")
	}
	tmp2 := s{}
	if err := tags.toAttr(context.Background(), tmp2); err == nil {
		t.Fatal("should fail, should pass ptrutil of struct")
	}
}
