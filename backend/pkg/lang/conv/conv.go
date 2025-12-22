// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conv

import (
	"reflect"
	"strconv"
	"unsafe"

	"github.com/bytedance/gg/gconv"
	"github.com/spf13/cast"
)

func UnsafeBytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// UnsafeStringToBytes
//
//nolint:staticcheck
func UnsafeStringToBytes(s string) (b []byte) {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len
	return b
}

func ToBool(v any) bool {
	return cast.ToBool(v)
}

func ToString(v any) string {
	return cast.ToString(v)
}

// Int64 will convert the given value to a int64, returns the default value of 0
// if a conversion can not be made.
func Int64(from interface{}) (int64, error) {
	return gconv.ToE[int64, any](from)
}

func ReduceFloatSignificantDigit(src float64, digit int) float64 {
	rateStr := strconv.FormatFloat(src, 'g', digit, 64)
	f, err := strconv.ParseFloat(rateStr, 64)
	if err != nil {
		return src
	}

	return f
}
