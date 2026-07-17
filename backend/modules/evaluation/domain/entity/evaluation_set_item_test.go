// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestItemErrorType_String_Extra(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "IllegalExtension", ItemErrorType_IllegalExtension.String())
	assert.Equal(t, "<UNSET>", ItemErrorType(0).String())
}

func TestItemErrorType_FromString_Extra(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want ItemErrorType
	}{
		{"MismatchSchema", "MismatchSchema", ItemErrorType_MismatchSchema},
		{"ExceedMaxItemSize", "ExceedMaxItemSize", ItemErrorType_ExceedMaxItemSize},
		{"ExceedDatasetCapacity", "ExceedDatasetCapacity", ItemErrorType_ExceedDatasetCapacity},
		{"MalformedFile", "MalformedFile", ItemErrorType_MalformedFile},
		{"IllegalContent", "IllegalContent", ItemErrorType_IllegalContent},
		{"TransformItemFailed", "TransformItemFailed", ItemErrorType_TransformItemFailed},
		{"IllegalExtension", "IllegalExtension", ItemErrorType_IllegalExtension},
		{"InternalError", "InternalError", ItemErrorType_InternalError},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ItemErrorTypeFromString(tt.in)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	got, err := ItemErrorTypeFromString("unknown")
	assert.Error(t, err)
	assert.Equal(t, ItemErrorType(0), got)
}
