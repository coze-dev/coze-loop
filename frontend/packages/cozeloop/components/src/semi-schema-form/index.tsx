// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import type Form from '@rjsf/core';
import { withTheme } from '@rjsf/core';

export { default as schemaValidators } from '@rjsf/validator-ajv8';

import { widgets } from './components/widgets';
import { templates } from './components/tmpls';

export const SemiSchemaForm = withTheme({
  widgets,
  templates,
});

export type SemiSchemaFormInstance = Form;
