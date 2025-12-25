// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-params */
/* eslint-disable @typescript-eslint/no-magic-numbers */
import { useEffect, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  Mention,
  type MentionOpenChangeEvent,
  getCurrentMentionReplaceRange,
  useEditor,
  PositionMirror,
  useLatest,
} from '@coze-editor/editor/react';
import { type EditorAPI } from '@coze-editor/editor/preset-prompt';
import { Popover, Typography } from '@coze-arch/coze-design';

export interface VariableType {
  /** 变量名字 */
  key?: string;
  /** 变量值/mock值 */
  value?: string;
}

function Variable({
  variables,
  isGoTemplate,
}: {
  variables: VariableType[];
  isGoTemplate?: boolean;
}) {
  const [posKey, setPosKey] = useState('');
  const [visible, setVisible] = useState(false);
  const [position, setPosition] = useState(-1);
  const editor = useEditor<EditorAPI>();

  function insert(variableName: string) {
    const range = getCurrentMentionReplaceRange(editor.$view.state);

    if (!range) {
      return;
    }

    editor.replaceText({
      ...range,
      text: isGoTemplate ? `{{.${variableName}}}` : `{{${variableName}}}`,
      cursorOffset: variableName ? 0 : -2,
    });

    setVisible(false);
  }

  function handleOpenChange(e: MentionOpenChangeEvent) {
    setPosition(e.state.selection.main.head);
    setVisible(e.value && variables.length > 0);
  }

  useEffect(() => {
    if (!editor) {
      return;
    }

    // 当变量浮层出现时，禁用 上、下、回车键 在编辑器中的默认行为
    if (visible) {
      setTimeout(() => {
        editor.disableKeybindings(['ArrowUp', 'ArrowDown', 'Enter']);
      }, 100);
    } else {
      setTimeout(() => {
        editor.disableKeybindings([]);
      }, 100);
    }
  }, [editor, visible]);

  const selectedIndex = usePopoverNavigation({
    enable: visible,
    variables,
    onApply: insert,
  });

  return (
    <>
      <Mention
        search={false}
        trigger={tr => {
          if (tr.docChanged) {
            let context:
              | {
                  from: number;
                  to: number;
                  triggerCharacter: string;
                  cursorPosition: number;
                }
              | undefined = undefined;

            tr.changes.iterChanges((fromA, toA, fromB, toB, inserted) => {
              if (
                fromA > 0 &&
                // 新增，非替换
                fromA === toA &&
                // 新增内容为 {
                inserted.toString() === '{}' &&
                // { 前还有个 {
                tr.state.sliceDoc(
                  Math.max(0, fromA - 1),
                  Math.max(0, fromA),
                ) === '{'
              ) {
                context = {
                  from: fromB - 1,
                  to: toB + 1,
                  triggerCharacter: '{{',
                  cursorPosition: tr.state.selection.main.head,
                };
              }
            });

            return context;
          }

          return undefined;
        }}
        onOpenChange={handleOpenChange}
      />

      <Popover
        visible={visible}
        trigger="custom"
        position="topLeft"
        rePosKey={posKey}
        content={
          <div className="p-1 min-w-[100px] flex flex-col gap-1">
            <Typography.Text type="secondary" strong>
              {I18n.t('insert_variable')}
            </Typography.Text>
            <div className="max-h-[200px] overflow-y-auto">
              {variables.map((variable, index) => (
                <div
                  key={variable.key}
                  className={`cursor-pointer hover:bg-gray-100 px-2 py-1 mb-0.5 rounded ${selectedIndex === index ? '!bg-gray-200' : ''}`}
                  onMouseDown={e => e.preventDefault()}
                  onClick={() => variable?.key && insert(variable.key)}
                >
                  {variable.key}
                </div>
              ))}
              {/* <div
               key="new-variable"
               className={`hover:bg-gray-200 px-2 py-1 rounded ${selectedIndex === variables.length ? 'bg-gray-200' : ''}`}
               onMouseDown={e => e.preventDefault()}
               onClick={() => insert('')}
              >
               插入新变量
              </div> */}
            </div>
          </div>
        }
        onClickOutSide={() => setVisible(false)}
      >
        {/* PositionMirror 可以让 Popover 出现在指定的光标位置 */}
        <PositionMirror
          position={position}
          // 当文档内容滚动时，需要更新 Popover 位置
          onChange={() => setPosKey(String(Math.random()))}
        />
      </Popover>
    </>
  );
}

// 上键：上一项
// 下键：下一项
// 回车：插入变量
function usePopoverNavigation(options: {
  enable: boolean;
  variables: VariableType[];
  onApply: (name: string) => void;
}) {
  const { enable, variables, onApply } = options;
  const [selectedIndex, setSelectedIndex] = useState(0);
  const indexRef = useLatest(selectedIndex);
  const variablesRef = useLatest([...variables, { key: '' }]);
  const onApplyRef = useLatest(onApply);

  useEffect(() => {
    setSelectedIndex(0);
  }, [enable]);

  useEffect(() => {
    if (!enable) {
      return;
    }

    function handleNavigation(e: KeyboardEvent) {
      switch (e.key) {
        case 'ArrowUp':
          setSelectedIndex(prevIndex => {
            const nextIndex = prevIndex - 1;
            return Math.max(0, nextIndex);
          });
          break;
        case 'ArrowDown':
          setSelectedIndex(prevIndex => {
            const nextIndex = prevIndex + 1;
            return Math.min(variablesRef.current.length - 1, nextIndex);
          });
          break;
        case 'Enter': {
          const variableName = variablesRef.current[indexRef.current];
          if (variableName) {
            onApplyRef.current(variableName.key || '');
          }
          break;
        }
        default:
          break;
      }
    }

    document.addEventListener('keydown', handleNavigation, false);

    return () => {
      document.removeEventListener('keydown', handleNavigation, false);
    };
  }, [enable]);

  return selectedIndex;
}

export default Variable;
