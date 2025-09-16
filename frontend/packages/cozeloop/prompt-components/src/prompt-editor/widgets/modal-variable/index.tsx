// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
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
  variableKeys,
  disabled,
}: {
  isMultimodal?: boolean;
  variableKeys?: string[];
  disabled?: boolean;
}) {
  const editor = useEditor<EditorAPI>();
  const injector = useInjector();
  const editorRef = useLatest(editor);
  const regex = new RegExp(
    '<multimodal-variable>(.*?)</multimodal-variable>',
    'gm',
  );
  const isMultimodalRef = useLatest(isMultimodal);
  const variableKeysRef = useLatest(variableKeys);
  const disabledRef = useLatest(disabled);

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
            const matchText = matches[1];
            const disabledKey = variableKeysRef.current?.some(
              key => key === matchText,
            );

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
                  isMultimodal: isMultimodalRef.current,
                  disabled: disabledRef.current || disabledKey,
                  disabledTip: disabledKey ? '多模态变量名冲突' : undefined,
                  from,
                  to,
                }),
                atomicRange: true,
              }),
            );
          },
        }),
      ]),
    [],
  );

  return null;
}
