// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export interface BlackSchemaEditorGroupValue {
  inputValue: string;
  outputValue: string;
}

export interface BlackSchemaEditorGroupProps {
  value: BlackSchemaEditorGroupValue;
  onChange?: (value: { inputValue: string; outputValue: string }) => void;
  disabled?: boolean;
}
