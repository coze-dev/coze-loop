/* eslint-disable complexity */
/* eslint-disable security/detect-non-literal-regexp */
/* eslint-disable @typescript-eslint/naming-convention */
import { type ReactNode, useLayoutEffect } from 'react';

import { useInjector } from '@coze-editor/editor/react';
import { astDecorator } from '@coze-editor/editor';

import { VARIABLE_MAX_LEN } from '@/consts';

import { type VariableType } from './variable';

import styles from './validation.module.css';

function validate(text: string) {
  const regex = new RegExp(`^[a-zA-Z][\\w]{0,${VARIABLE_MAX_LEN - 1}}$`, 'gm');
  if (regex.test(text)) {
    return true;
  }

  return false;
}

/**
 * 解析Jinja变量语法，提取变量名
 * 支持以下格式：
 * - text.a (对象属性访问) -> 返回 text
 * - text[0] (数组索引访问) -> 返回 text
 * - text (简单变量) -> 返回 text
 */
function parseJinjaVariable(text: string): string {
  const trimmedText = text.trim();

  // 处理对象属性访问: text.a 或 text.a.b
  if (trimmedText.includes('.')) {
    return trimmedText.split('.')[0];
  }

  // 处理数组索引访问: text[0] 或 text[0][1]
  if (trimmedText.includes('[')) {
    return trimmedText.split('[')[0];
  }

  // 简单变量，直接返回
  return trimmedText;
}

export function Validation({
  variables,
  isNormalTemplate,
}: {
  variables?: VariableType[];
  isNormalTemplate?: boolean;
}): ReactNode {
  const injector = useInjector();

  // 用于校验 {{ }} 中的变量，如果变量无效，使用灰色标识
  useLayoutEffect(
    () =>
      injector.inject([
        astDecorator.whole.of((cursor, state) => {
          if (
            cursor.name === 'JinjaExpression' &&
            cursor.node.firstChild?.name === 'JinjaExpressionStart' &&
            cursor.node.lastChild?.name === 'JinjaExpressionEnd'
          ) {
            const from = cursor.node.firstChild.to;
            const to = cursor.node.lastChild.from;
            const text = state.sliceDoc(from, to);
            if (validate(text) && isNormalTemplate) {
              return {
                type: 'className',
                className: styles.valid,
                from,
                to,
              };
            }

            if (!isNormalTemplate) {
              const variableName = parseJinjaVariable(text);
              const item = variables?.find(
                variable => variable.key === variableName,
              );
              if (item) {
                return {
                  type: 'className',
                  className: styles.valid,
                  from,
                  to,
                };
              }
            }

            return {
              type: 'className',
              className: styles.invalid,
              from,
              to,
            };
          }

          if (
            cursor.name === 'JinjaStatement' &&
            cursor.node.firstChild?.name === 'JinjaStatementStart' &&
            cursor.node.lastChild?.name === 'JinjaStatementEnd' &&
            !isNormalTemplate
          ) {
            const from = cursor.node.firstChild.to;
            const to = cursor.node.lastChild.from;
            const text = state.sliceDoc(from, to);
            // 过滤掉 undefined 值
            return (variables || [])
              .map(variable => {
                if (variable.key) {
                  const index = text.indexOf(variable.key);
                  if (index !== -1) {
                    return {
                      type: 'className',
                      className: styles.valid,
                      from: from + index,
                      to: from + index + variable.key.length + 1,
                    };
                  }
                }
                return undefined;
              })
              .filter(
                (
                  item,
                ): item is {
                  type: 'className';
                  className: string;
                  from: number;
                  to: number;
                } => item !== undefined,
              );
          }
          return undefined;
        }),
      ]),
    [injector, variables],
  );

  return null;
}

export default Validation;
