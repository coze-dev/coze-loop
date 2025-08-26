// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useLayoutEffect } from 'react';

import { useInjector } from '@coze-editor/editor/react';
import { astDecorator } from '@coze-editor/editor';
import { EditorView } from '@codemirror/view';

function JinjaHighlight() {
  const injector = useInjector();

  useLayoutEffect(
    () =>
      injector.inject([
        astDecorator.whole.of(cursor => {
          // 语句块高亮
          if (
            cursor.name === 'JinjaStatementStart' ||
            cursor.name === 'JinjaStatementEnd'
          ) {
            return {
              type: 'className',
              className: 'jinja-statement-bracket',
            };
          }

          // 控制结构高亮
          if (cursor.name === 'JinjaControlStructure') {
            return {
              type: 'className',
              className: 'jinja-control',
            };
          }

          // 过滤器高亮
          if (cursor.name === 'JinjaFilter') {
            return {
              type: 'className',
              className: 'jinja-filter',
            };
          }

          // 注释高亮
          if (cursor.name === 'JinjaComment') {
            return {
              type: 'className',
              className: 'jinja-comment',
            };
          }

          // 表达式高亮
          if (cursor.name === 'JinjaExpression') {
            return {
              type: 'className',
              className: 'jinja-expression',
            };
          }

          // 变量高亮
          if (cursor.name === 'JinjaVariable') {
            return {
              type: 'className',
              className: 'jinja-variable',
            };
          }
        }),
        EditorView.theme({
          '.jinja-expression': {
            color: 'var(--Green-COZColorGreen7, #00A136)',
          },
          '.jinja-control': {
            color: 'var(--Blue-COZColorBlue7, #0066CC)',
            fontWeight: 'bold',
          },
          '.jinja-filter': {
            color: 'var(--Purple-COZColorPurple7, #7B68EE)',
          },
          '.jinja-comment': {
            color: 'var(--Gray-COZColorGray6, #999999)',
            fontStyle: 'italic',
          },
          '.jinja-variable': {
            color: 'var(--Orange-COZColorOrange7, #FF8C00)',
          },
          '.jinja-statement-bracket': {
            color: 'var(--Red-COZColorRed7, #FF4444)',
            fontWeight: 'bold',
          },
        }),
      ]),
    [injector],
  );

  return null;
}

export default JinjaHighlight;
