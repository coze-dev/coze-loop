// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useLayoutEffect } from 'react';

import { useInjector } from '@coze-editor/editor/react';
import { languageSupport } from '@coze-editor/editor/preset-prompt';

function LanguageSupport() {
  const injector = useInjector();

  useLayoutEffect(() => injector.inject([languageSupport]), [injector]);

  return null;
}

export default LanguageSupport;
