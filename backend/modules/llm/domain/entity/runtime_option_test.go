// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestWithResponseFormat(t *testing.T) {
	type args struct {
		r *ResponseFormat
	}
	tests := []struct {
		name string
		args args
		want *Options
	}{
		{
			name: "normal",
			args: args{
				r: &ResponseFormat{
					Type: ResponseFormatTypeText,
				},
			},
			want: &Options{
				ResponseFormat: &ResponseFormat{
					Type: ResponseFormatTypeText,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ApplyOptions(nil, WithResponseFormat(tt.args.r))
			assert.Equal(t, tt.want.ResponseFormat.Type, opts.ResponseFormat.Type)
		})
	}
}

func TestWithTopK(t *testing.T) {
	type args struct {
		r *int32
	}
	tests := []struct {
		name string
		args args
		want int32
	}{
		{
			name: "normal",
			args: args{
				r: ptr.Of(int32(1)),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ApplyOptions(nil, WithTopK(tt.args.r))
			assert.Equal(t, tt.want, *opts.TopK)
		})
	}
}

func TestWithFrequencyPenalty(t *testing.T) {
	type args struct {
		r float32
	}
	tests := []struct {
		name string
		args args
		want float32
	}{
		{
			name: "normal",
			args: args{
				r: float32(1.0),
			},
			want: 1.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ApplyOptions(nil, WithFrequencyPenalty(tt.args.r))
			assert.Equal(t, tt.want, *opts.FrequencyPenalty)
		})
	}
}

func TestWithPresencePenalty(t *testing.T) {
	type args struct {
		r float32
	}
	tests := []struct {
		name string
		args args
		want float32
	}{
		{
			name: "normal",
			args: args{
				r: float32(1.0),
			},
			want: 1.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ApplyOptions(nil, WithPresencePenalty(tt.args.r))
			assert.Equal(t, tt.want, *opts.PresencePenalty)
		})
	}
}

func TestWrapAndGetSpecificOptFn(t *testing.T) {
	type specific struct {
		name string
	}
	type args struct {
		f func(sp *specific)
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "normal",
			args: args{
				f: func(sp *specific) {
					sp.name = "test"
				},
			},
			want: "test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := GetImplSpecificOptions(&specific{}, WrapImplSpecificOptFn(tt.args.f))
			assert.Equal(t, tt.want, opts.name)
		})
	}
}
