// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStorageProvider_String(t *testing.T) {
	testCases := []struct {
		provider StorageProvider
		expect   string
	}{
		{StorageProvider_TOS, "TOS"},
		{StorageProvider_VETOS, "VETOS"},
		{StorageProvider_HDFS, "HDFS"},
		{StorageProvider_ImageX, "ImageX"},
		{StorageProvider_S3, "S3"},
		{StorageProvider_Abase, "Abase"},
		{StorageProvider_RDS, "RDS"},
		{StorageProvider_LocalFS, "LocalFS"},
		{StorageProvider(999), "<UNSET>"}, // 未知值
	}

	for _, tc := range testCases {
		t.Run(tc.expect, func(t *testing.T) {
			assert.Equal(t, tc.expect, tc.provider.String())
		})
	}
}

func TestStorageProviderFromString(t *testing.T) {
	testCases := []struct {
		input    string
		expect   StorageProvider
		expectOk bool
	}{
		{"TOS", StorageProvider_TOS, true},
		{"VETOS", StorageProvider_VETOS, true},
		{"HDFS", StorageProvider_HDFS, true},
		{"ImageX", StorageProvider_ImageX, true},
		{"S3", StorageProvider_S3, true},
		{"Abase", StorageProvider_Abase, true},
		{"RDS", StorageProvider_RDS, true},
		{"LocalFS", StorageProvider_LocalFS, true},
		{"unknown", StorageProvider(0), false},
		{"", StorageProvider(0), false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			val, err := StorageProviderFromString(tc.input)
			if tc.expectOk {
				assert.NoError(t, err)
				assert.Equal(t, tc.expect, val)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
