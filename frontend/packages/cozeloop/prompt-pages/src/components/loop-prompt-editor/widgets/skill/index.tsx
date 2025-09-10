// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useLayoutEffect } from 'react';

import {
  astDecorator,
  type EditorAPIPrompt,
  useEditor,
  useInjector,
  useLatest,
} from '@cozeloop/prompt-components';

import { type SkillDataInfo, SkillWidget } from './widget';
import { TemplateParser } from './template-parser';

const templateParser = new TemplateParser({ mark: 'LibraryBlock' });

export default function LibraryBlockWidget({ librarys }: { librarys?: any[] }) {
  const editor = useEditor<EditorAPIPrompt>();
  const injector = useInjector();
  const librarysRef = useLatest(librarys);

  useLayoutEffect(
    () =>
      injector.inject([
        astDecorator.whole.of((cursor, state) => {
          if (templateParser.isOpenNode(cursor.node, state)) {
            const open = cursor.node;
            const close = templateParser.findCloseNode(open, state);

            if (close) {
              const openTemplate = state.sliceDoc(open.from, open.to);

              const dataInfo = templateParser.getData(
                openTemplate,
              ) as SkillDataInfo;

              return [
                {
                  type: 'replace',
                  widget: new SkillWidget({
                    librarys: librarysRef.current,
                    dataInfo,
                    readonly: false,
                    from: open.from,
                    to: close.to,
                  }),
                  atomicRange: true,
                  from: open.from,
                  to: close.to,
                },
              ];
            }
          }
        }),
        templateParser.markInfoField,
      ]),
    [injector],
  );

  useEffect(() => {
    if (!editor) {
      return;
    }
    editor?.updateWholeDecorations();
  }, [editor]);

  return null;
}
