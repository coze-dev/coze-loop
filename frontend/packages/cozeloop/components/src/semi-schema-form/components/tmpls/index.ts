// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type TemplatesType } from '@rjsf/utils';

import TitleFieldTemplate from './title-field';
import SubmitButton from './submit';
import ObjectFieldTemplate from './object-field';
import {
  AddButton,
  CopyButton,
  MoveDownButton,
  MoveUpButton,
  RemoveButton,
} from './icon-button';
import FieldErrorTemplate from './field-error';
import FieldTemplate from './field';
import ErrorListTemplate from './error-list';
import DescriptionFieldTemplate from './description-field';
import BaseInputTemplate from './base-input';
import ArrayFieldItemTemplate from './array-field-item';
import ArrayFieldTemplate from './array-field';

export const templates: Partial<TemplatesType> = {
  ArrayFieldTemplate,
  ArrayFieldItemTemplate,
  BaseInputTemplate,
  FieldTemplate,
  ObjectFieldTemplate,
  ButtonTemplates: {
    AddButton,
    CopyButton,
    MoveDownButton,
    MoveUpButton,
    RemoveButton,
    SubmitButton,
  },
  TitleFieldTemplate,
  DescriptionFieldTemplate,
  FieldErrorTemplate,
  ErrorListTemplate,
};
