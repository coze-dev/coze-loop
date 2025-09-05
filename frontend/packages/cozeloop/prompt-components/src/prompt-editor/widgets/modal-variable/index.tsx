/* eslint-disable max-params */
import { useLayoutEffect } from 'react';

import { nanoid } from 'nanoid';
import regexpDecorator from '@coze-editor/extension-regexp-decorator';
import { useEditor, useInjector, useLatest } from '@coze-editor/editor/react';
import { type EditorAPI } from '@coze-editor/editor/preset-code';
import { Decoration } from '@codemirror/view';

import { ModalVariableWidget } from './widget';

export default function ModalVariableCompletion({
  isMultimodal,
}: {
  isMultimodal?: boolean;
}) {
  const editor = useEditor<EditorAPI>();
  const injector = useInjector();
  const editorRef = useLatest(editor);
  const regex = new RegExp(
    '<multimodal-variable>(.*?)</multimodal-variable>',
    'gm',
  );

  useLayoutEffect(
    () =>
      injector.inject([
        regexpDecorator({
          regexp: regex,
          decorate: (add, from, to, matches, view) => {
            // const facet = view.state.facet(cunstomFacet);

            // const matchText = matches[0];
            // const prompt = segmentMap?.[matchText];
            // let stateType = '';

            // if (facet?.id === 'a') {
            //   const newValue = facet?.newValue;
            //   if (!newValue?.includes(matchText)) {
            //     stateType = 'delete';
            //   }
            // }
            // if (facet?.id === 'b') {
            //   const oldValue = facet?.oldValue;
            //   if (!oldValue?.includes(matchText)) {
            //     stateType = 'add';
            //   }
            // }
            console.log('isMultimodal', isMultimodal);
            const matchText = matches[1];

            add(
              from,
              to,
              Decoration.replace({
                widget: new ModalVariableWidget({
                  dataInfo: {
                    variableKey: matchText,
                    uuid: nanoid(),
                  },
                  onDelete: () => {
                    editorRef.current?.replaceText({ from, to, text: '' });
                  },
                  readonly: view.state.readOnly,
                  isMultimodal,
                  from,
                  to,
                }),
                atomicRange: true,
              }),
            );
          },
        }),
      ]),
    [isMultimodal],
  );

  return null;
}
