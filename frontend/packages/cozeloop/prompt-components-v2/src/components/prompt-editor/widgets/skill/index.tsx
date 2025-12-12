/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useLayoutEffect } from 'react';

import { useEditor, useInjector, useLatest } from '@coze-editor/editor/react';
import { type EditorAPI } from '@coze-editor/editor/preset-prompt';
import { astDecorator } from '@coze-editor/editor';

import { type SkillDataInfo, SkillWidget } from './widget';
import { TemplateParser } from './template-parser';

const templateParser = new TemplateParser({ mark: 'LibraryBlock' });

export default function LibraryBlockWidget({ librarys }: { librarys?: any[] }) {
  const editor = useEditor<EditorAPI>();
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
