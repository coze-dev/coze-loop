// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useLayoutEffect } from 'react';
import { useInjector } from '@coze-editor/editor/react';
import { astDecorator } from '@coze-editor/editor';
import { EditorView } from '@codemirror/view';

function JinjaValidation() {
  const injector = useInjector();

  useLayoutEffect(
    () =>
      injector.inject([
        astDecorator.whole.of((cursor, state) => {
          // 验证 Jinja2 语法错误
          if (cursor.name === 'JinjaExpression' || cursor.name === 'JinjaStatement') {
            const text = state.sliceDoc(cursor.from, cursor.to);

            // 检查括号匹配
            if (!isValidJinjaSyntax(text)) {
              return {
                type: 'className',
                className: 'jinja-error',
                from: cursor.from,
                to: cursor.to,
              };
            }
          }
        }),
        EditorView.theme({
          '.jinja-error': {
            backgroundColor: 'rgba(255, 0, 0, 0.1)',
            borderBottom: '2px wavy red',
          },
        }),
      ]),
    [injector],
  );

  return null;
}

// 验证Jinja2语法
function isValidJinjaSyntax(text: string): boolean {
  // 基本的语法验证
  const openBraces = (text.match(/\{\{/g) || []).length;
  const closeBraces = (text.match(/\}\}/g) || []).length;
  const openStatements = (text.match(/\{\%/g) || []).length;
  const closeStatements = (text.match(/\%\}/g) || []).length;

  // 检查括号匹配
  if (openBraces !== closeBraces || openStatements !== closeStatements) {
    return false;
  }

  // 检查控制结构配对
  const controlPairs = [
    ['if', 'endif'],
    ['for', 'endfor'],
    ['set', 'endset'],
    ['block', 'endblock'],
    ['macro', 'endmacro'],
    ['call', 'endcall'],
    ['filter', 'endfilter'],
    ['with', 'endwith'],
    ['autoescape', 'endautoescape'],
    ['raw', 'endraw'],
  ];

  for (const [start, end] of controlPairs) {
    const startCount = (text.match(new RegExp(`\\{%\\s*${start}\\b`, 'g')) || []).length;
    const endCount = (text.match(new RegExp(`\\{%\\s*${end}\\b`, 'g')) || []).length;

    if (startCount !== endCount) {
      return false;
    }
  }

  // 检查过滤器语法
  const filterPattern = /\|\s*\w+/g;
  const filters = text.match(filterPattern);
  if (filters) {
    for (const filter of filters) {
      const filterName = filter.trim().substring(1); // 去掉管道符号
      if (!isValidFilterName(filterName)) {
        return false;
      }
    }
  }

  return true;
}

// 验证过滤器名称
function isValidFilterName(name: string): boolean {
  const validFilters = [
    'upper', 'lower', 'title', 'capitalize',
    'trim', 'truncate', 'wordwrap',
    'default', 'length', 'reverse',
    'sort', 'join', 'replace',
    'safe', 'escape', 'striptags',
    'abs', 'round', 'int', 'float',
    'list', 'string', 'bool',
    'strip', 'split', 'replace',
    'max', 'min', 'sum', 'avg',
    'first', 'last', 'random',
    'unique', 'groupby', 'map',
    'select', 'reject', 'batch',
    'slice', 'indent', 'nl2br',
    'urlize', 'markdown', 'striptags',
  ];

  return validFilters.includes(name.toLowerCase());
}

export default JinjaValidation;
