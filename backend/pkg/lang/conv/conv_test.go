// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conv

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnsafeBytesToString(t *testing.T) {
	type args struct {
		b []byte
	}

	str := "test string"

	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "test", args: args{b: []byte(str)}, want: str},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UnsafeBytesToString(tt.args.b); got != tt.want {
				t.Errorf("UnsafeBytesToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnsafeStringToBytes(t *testing.T) {
	type args struct {
		s string
	}

	str := "test string"

	tests := []struct {
		name  string
		args  args
		wantB []byte
	}{
		{name: "test", args: args{s: str}, wantB: []byte(str)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotB := UnsafeStringToBytes(tt.args.s); !reflect.DeepEqual(gotB, tt.wantB) {
				t.Errorf("UnsafeStringToBytes() = %v, want %v", gotB, tt.wantB)
			}
		})
	}
}

func TestInt64(t *testing.T) {
	got, err := Int64("1")
	assert.NoError(t, err)
	assert.Equal(t, got, int64(1))

	_, err = Int64("str")
	assert.Error(t, err)

	str := "1"
	got, err = Int64(&str)
	assert.NoError(t, err)
	assert.Equal(t, got, int64(1))
}

func TestReduceFloatSignificantDigit(t *testing.T) {
	got := ReduceFloatSignificantDigit(0.1234567890123456789, 10)
	assert.Equal(t, got, 0.123456789)
}
