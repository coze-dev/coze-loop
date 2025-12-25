// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type RegistryWidgetsType } from '@rjsf/utils';

import TextAreaWidget from './textarea';
import SelectWidget from './select';
import RangeWidget from './range';
import RadioWidget from './radio';
import CheckboxesWidget from './checkboxs';
import CheckboxWidget from './checkbox';

export const widgets: RegistryWidgetsType = {
  TextAreaWidget,
  SelectWidget,
  CheckboxWidget,
  CheckboxesWidget,
  RadioWidget,
  RangeWidget,
};
