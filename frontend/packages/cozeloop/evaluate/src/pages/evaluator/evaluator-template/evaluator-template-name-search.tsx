// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Search, type SearchProps } from '@coze-arch/coze-design';

export function EvaluatorTemplateNameSearchInput(props: SearchProps) {
  return (
    <Search placeholder={I18n.t('evaluator_search_placeholder')} {...props} />
  );
}
