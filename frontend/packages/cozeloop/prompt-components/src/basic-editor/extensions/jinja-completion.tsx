// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useLayoutEffect } from 'react';
import { useInjector } from '@coze-editor/editor/react';
import { autocompletion } from '@codemirror/autocomplete';
import { EditorView } from '@codemirror/view';

// Jinja2关键字列表
const jinjaKeywords = [
  'if', 'endif', 'else', 'elif',
  'for', 'endfor', 'in',
  'set', 'endset',
  'block', 'endblock',
  'macro', 'endmacro',
  'call', 'endcall',
  'filter', 'endfilter',
  'with', 'endwith',
  'autoescape', 'endautoescape',
  'raw', 'endraw',
  'do', 'flush',
  'include', 'import', 'from',
  'extends', 'super',
];

// Jinja2过滤器列表
const jinjaFilters = [
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

// Jinja2内置函数列表
const jinjaFunctions = [
  'range', 'lipsum', 'dict',
  'cycler', 'joiner', 'namespace',
  'now', 'url_for', 'get_flashed_messages',
  'config', 'request', 'session',
  'g', 'url', 'redirect',
];

// 创建自动补全函数
function createJinjaCompletion() {
  return autocompletion({
    override: [
      (context) => {
        const word = context.matchBefore(/\w*/);
        if (!word) return null;

        // 检查是否在Jinja2块内
        const line = context.state.doc.lineAt(context.pos);
        const lineText = line.text;
        const posInLine = context.pos - line.from;

        // 检查是否在Jinja2语法块内
        const beforePos = lineText.slice(0, posInLine);
        const isInJinjaBlock = /{%|{{|{#/.test(beforePos);

        if (!isInJinjaBlock) return null;

        const options = [];

        // 关键字补全
        for (const keyword of jinjaKeywords) {
          if (keyword.toLowerCase().startsWith(word.text.toLowerCase())) {
            options.push({
              label: keyword,
              type: 'keyword',
              info: `Jinja2 keyword: ${keyword}`,
              boost: 10,
            });
          }
        }

        // 过滤器补全（当输入管道符号后）
        if (context.state.sliceDoc(context.pos - 1, context.pos) === '|') {
          for (const filter of jinjaFilters) {
            options.push({
              label: filter,
              type: 'function',
              info: `Jinja2 filter: ${filter}`,
              boost: 8,
            });
          }
        }

        // 函数补全
        for (const func of jinjaFunctions) {
          if (func.toLowerCase().startsWith(word.text.toLowerCase())) {
            options.push({
              label: func,
              type: 'function',
              info: `Jinja2 function: ${func}`,
              boost: 6,
            });
          }
        }

        // 过滤器补全（一般情况）
        for (const filter of jinjaFilters) {
          if (filter.toLowerCase().startsWith(word.text.toLowerCase())) {
            options.push({
              label: filter,
              type: 'function',
              info: `Jinja2 filter: ${filter}`,
              boost: 5,
            });
          }
        }

        // 如果没有匹配项，提供所有选项
        if (options.length === 0) {
          options.push(
            ...jinjaKeywords.map(keyword => ({
              label: keyword,
              type: 'keyword',
              info: `Jinja2 keyword: ${keyword}`,
              boost: 3,
            })),
            ...jinjaFilters.map(filter => ({
              label: filter,
              type: 'function',
              info: `Jinja2 filter: ${filter}`,
              boost: 2,
            })),
            ...jinjaFunctions.map(func => ({
              label: func,
              type: 'function',
              info: `Jinja2 function: ${func}`,
              boost: 1,
            }))
          );
        }

        return {
          from: word.from,
          options: options.slice(0, 50), // 限制选项数量
        };
      },
    ],
  });
}

function JinjaCompletion() {
  const injector = useInjector();

  useLayoutEffect(
    () =>
      injector.inject([
        createJinjaCompletion(),
        EditorView.theme({
          '.cm-tooltip.cm-tooltip-autocomplete': {
            fontFamily: 'monospace',
            fontSize: '13px',
          },
          '.cm-tooltip.cm-tooltip-autocomplete ul': {
            maxHeight: '200px',
            overflowY: 'auto',
          },
        }),
      ]),
    [injector],
  );

  return null;
}

export default JinjaCompletion;
