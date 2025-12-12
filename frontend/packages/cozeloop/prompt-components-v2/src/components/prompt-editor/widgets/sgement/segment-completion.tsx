/* eslint-disable max-len */
/* eslint-disable security/detect-object-injection */
/* eslint-disable max-params */
import { useLayoutEffect } from 'react';

import regexpDecorator from '@coze-editor/extension-regexp-decorator';
import { useEditor, useInjector, useLatest } from '@coze-editor/editor/react';
import { type EditorAPI } from '@coze-editor/editor/preset-prompt';
import { Decoration } from '@codemirror/view';

import { usePromptStore } from '@/store/use-prompt-store';
import { cunstomFacet } from '@/components/basic-prompt-editor/custom-facet';

import { SgementWidget } from './segment-widget';

export default function SegmentCompletion() {
  const editor = useEditor<EditorAPI>();
  const injector = useInjector();
  const editorRef = useLatest(editor);

  const regex = new RegExp(
    '<fornax_prompt>id=\\d+&version=[\\w.]+</fornax_prompt>',
    'gm',
  );

  useLayoutEffect(
    () =>
      injector.inject([
        regexpDecorator({
          regexp: regex,
          decorate: (add, from, to, matches, view) => {
            const facet = view.state.facet(cunstomFacet);
            const { setSnippetMap, snippetMap } = usePromptStore.getState();
            const matchText = matches[0];
            const prompt = snippetMap?.[matchText];
            let stateType = '';

            if (facet?.id === 'a') {
              const newValue = facet?.newValue;
              if (!newValue?.includes(matchText)) {
                stateType = 'delete';
              }
            }
            if (facet?.id === 'b') {
              const oldValue = facet?.oldValue;
              if (!oldValue?.includes(matchText)) {
                stateType = 'add';
              }
            }
            add(
              from,
              to,
              Decoration.replace({
                widget: new SgementWidget({
                  segment: prompt,
                  onDelete: () => {
                    editorRef.current?.replaceText({ from, to, text: '' });
                  },
                  onItemClick: info => {
                    if (!info) {
                      return;
                    }
                    const text = `<fornax_prompt>id=${info?.id}&version=${info?.prompt_commit?.commit_info?.version}</fornax_prompt>`;
                    setSnippetMap(map => {
                      if (!map) {
                        return {
                          [text]: info,
                        };
                      }
                      return {
                        ...map,
                        [text]: info,
                      };
                    });
                    editorRef.current?.replaceText({ from, to, text });
                  },
                  readonly: view.state.readOnly,
                  stateType,
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
