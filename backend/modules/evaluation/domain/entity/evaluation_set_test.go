// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatasetStatus_String_FromString_Ptr_Scan_Value(t *testing.T) {
	assert.Equal(t, "Available", DatasetStatus_Available.String())
	assert.Equal(t, "Deleted", DatasetStatus_Deleted.String())
	assert.Equal(t, "Expired", DatasetStatus_Expired.String())
	assert.Equal(t, "Importing", DatasetStatus_Importing.String())
	assert.Equal(t, "Exporting", DatasetStatus_Exporting.String())
	assert.Equal(t, "Indexing", DatasetStatus_Indexing.String())
	var unknown DatasetStatus = 99
	assert.Equal(t, "<UNSET>", unknown.String())

	ds, err := DatasetStatusFromString("Available")
	assert.NoError(t, err)
	assert.Equal(t, DatasetStatus_Available, ds)
	_, err = DatasetStatusFromString("not-exist")
	assert.Error(t, err)
	_, err = DatasetStatusFromString("Deleted")
	assert.NoError(t, err)
	_, err = DatasetStatusFromString("Expired")
	assert.NoError(t, err)
	_, err = DatasetStatusFromString("Importing")
	assert.NoError(t, err)
	_, err = DatasetStatusFromString("Exporting")
	assert.NoError(t, err)
	_, err = DatasetStatusFromString("Indexing")
	assert.NoError(t, err)

	ptr := DatasetStatusPtr(DatasetStatus_Exporting)
	assert.Equal(t, DatasetStatus_Exporting, *ptr)

	var s DatasetStatus
	assert.NoError(t, s.Scan(int64(2)))
	assert.Equal(t, DatasetStatus_Deleted, s)
	val, err := s.Value()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), val)
	var nilPtr *DatasetStatus
	val, err = nilPtr.Value()
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestItemErrorType_String_FromString(t *testing.T) {
	assert.Equal(t, "MismatchSchema", ItemErrorType_MismatchSchema.String())
	assert.Equal(t, "EmptyData", ItemErrorType_EmptyData.String())
	assert.Equal(t, "ExceedMaxItemSize", ItemErrorType_ExceedMaxItemSize.String())
	assert.Equal(t, "ExceedDatasetCapacity", ItemErrorType_ExceedDatasetCapacity.String())
	assert.Equal(t, "MalformedFile", ItemErrorType_MalformedFile.String())
	assert.Equal(t, "IllegalContent", ItemErrorType_IllegalContent.String())
	assert.Equal(t, "InternalError", ItemErrorType_InternalError.String())
	assert.Equal(t, "MissingRequiredField", ItemErrorType_MissingRequiredField.String())
	assert.Equal(t, "ExceedMaxNestedDepth", ItemErrorType_ExceedMaxNestedDepth.String())
	assert.Equal(t, "TransformItemFailed", ItemErrorType_TransformItemFailed.String())
	var unknown ItemErrorType = 99
	assert.Equal(t, "<UNSET>", unknown.String())

	typ, err := ItemErrorTypeFromString("EmptyData")
	assert.NoError(t, err)
	assert.Equal(t, ItemErrorType_EmptyData, typ)
	typ1, err := ItemErrorTypeFromString("MissingRequiredField")
	assert.NoError(t, err)
	assert.Equal(t, ItemErrorType_MissingRequiredField, typ1)
	typ2, err := ItemErrorTypeFromString("ExceedMaxNestedDepth")
	assert.NoError(t, err)
	assert.Equal(t, ItemErrorType_ExceedMaxNestedDepth, typ2)
	_, err = ItemErrorTypeFromString("not-exist")
	assert.Error(t, err)
}

func TestFieldDisplayFormat_String_FromString_Ptr_Scan_Value(t *testing.T) {
	assert.Equal(t, "PlainText", FieldDisplayFormat_PlainText.String())
	assert.Equal(t, "Markdown", FieldDisplayFormat_Markdown.String())
	assert.Equal(t, "JSON", FieldDisplayFormat_JSON.String())
	assert.Equal(t, "YAML", FieldDisplayFormat_YAML.String())
	assert.Equal(t, "Code", FieldDisplayFormat_Code.String())
	var unknown FieldDisplayFormat = 99
	assert.Equal(t, "<UNSET>", unknown.String())

	f, err := FieldDisplayFormatFromString("Markdown")
	assert.NoError(t, err)
	assert.Equal(t, FieldDisplayFormat_Markdown, f)
	_, err = FieldDisplayFormatFromString("not-exist")
	assert.Error(t, err)

	ptr := FieldDisplayFormatPtr(FieldDisplayFormat_JSON)
	assert.Equal(t, FieldDisplayFormat_JSON, *ptr)

	var ff FieldDisplayFormat
	assert.NoError(t, ff.Scan(int64(2)))
	assert.Equal(t, FieldDisplayFormat_Markdown, ff)
	val, err := ff.Value()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), val)
	var nilPtr *FieldDisplayFormat
	val, err = nilPtr.Value()
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestFieldStatus_String_FromString_Ptr_Scan_Value(t *testing.T) {
	assert.Equal(t, "Available", FieldStatus_Available.String())
	assert.Equal(t, "Deleted", FieldStatus_Deleted.String())
	var unknown FieldStatus = 99
	assert.Equal(t, "<UNSET>", unknown.String())

	f, err := FieldStatusFromString("Deleted")
	assert.NoError(t, err)
	assert.Equal(t, FieldStatus_Deleted, f)
	_, err = FieldStatusFromString("not-exist")
	assert.Error(t, err)

	ptr := FieldStatusPtr(FieldStatus_Available)
	assert.Equal(t, FieldStatus_Available, *ptr)

	var fs FieldStatus
	assert.NoError(t, fs.Scan(int64(2)))
	assert.Equal(t, FieldStatus_Deleted, fs)
	val, err := fs.Value()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), val)
	var nilPtr *FieldStatus
	val, err = nilPtr.Value()
	assert.NoError(t, err)
	assert.Nil(t, val)
}
