// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type MultiPartSchema } from '../../span-definition/prompt/schema';

export interface DatasetItemProps {
  fieldContent?: MultiPartSchema;
  className?: string;
  expand?: boolean;
}

export enum MultiPartType {
  Text = 'text',
  ImageUrl = 'image_url',
}
