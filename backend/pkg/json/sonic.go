// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package json

import (
	"io"

	"github.com/bytedance/sonic"
)

var stdConfig = sonic.ConfigStd

func Marshal(v interface{}) ([]byte, error) {
	return stdConfig.Marshal(v)
}

func MarshalString(v interface{}) (string, error) {
	return stdConfig.MarshalToString(v)
}

func MarshalStringIgnoreErr(v interface{}) string {
	res, _ := stdConfig.MarshalToString(v)
	return res
}

func MarshalIndent(v interface{}) ([]byte, error) {
	return stdConfig.MarshalIndent(v, "", "\t")
}

func Unmarshal(data []byte, v interface{}) error {
	return stdConfig.Unmarshal(data, v)
}

func Decode(reader io.Reader, v interface{}) error {
	return stdConfig.NewDecoder(reader).Decode(v)
}

func Valid(data []byte) bool {
	return stdConfig.Valid(data)
}

func NewDecoder(reader io.Reader) sonic.Decoder {
	return stdConfig.NewDecoder(reader)
}

func Jsonify(data interface{}) string {
	dump, _ := sonic.MarshalString(data)
	return dump
}
