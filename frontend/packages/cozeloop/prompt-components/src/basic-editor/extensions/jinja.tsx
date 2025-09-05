/* eslint-disable complexity */
import { useLayoutEffect } from 'react';

import { useInjector } from '@coze-editor/editor/react';
import { astDecorator } from '@coze-editor/editor';
import { EditorView } from '@codemirror/view';

const prec = 'lowest';

function NormalJinjaHighlight({
  isJinja2Template,
}: {
  isJinja2Template?: boolean;
}) {
  const injector = useInjector();

  useLayoutEffect(
    () =>
      injector.inject([
        astDecorator.whole.of(cursor => {
          if (
            cursor.name === 'JinjaStatement' ||
            cursor.name === 'JinjaRawOpenStatement' ||
            cursor.name === 'JinjaRawCloseStatement'
          ) {
            if (isJinja2Template) {
              return {
                type: 'className',
                className: 'jinja-statement',
                prec,
              };
            }
          }

          if (cursor.name === 'JinjaStringLiteral') {
            if (isJinja2Template) {
              return {
                type: 'className',
                className: 'jinja-string',
                prec,
              };
            }
          }

          if (cursor.name === 'JinjaComment') {
            if (isJinja2Template) {
              return {
                type: 'className',
                className: 'jinja-comment',
                prec,
              };
            }
          }

          if (cursor.name === 'JinjaExpression') {
            return {
              type: 'className',
              className: 'jinja-expression',
              prec,
            };
          }

          if (
            cursor.name === 'JinjaFilterName' ||
            cursor.name === 'JinjaStatementStart' ||
            cursor.name === 'JinjaStatementEnd' ||
            cursor.name === 'JinjaKeyword' ||
            cursor.name === 'JinjaFilterName'
          ) {
            if (isJinja2Template) {
              return {
                type: 'className',
                className: 'jinja-statement-keyword',
                prec,
              };
            }
          }
        }),
        EditorView.theme({
          '.jinja-statement': {
            color: '#060709CC',
          },
          '.jinja-statement-keyword': {
            color: '#D1009D',
          },
          '.jinja-string': {
            color: '#060709CC',
          },
          '.jinja-comment': {
            color: '#0607094D',
          },
          '.jinja-expression': {
            color: 'var(--Green-COZColorGreen7, #00A136)',
          },
        }),
      ]),
    [injector, isJinja2Template],
  );

  return null;
}

export default NormalJinjaHighlight;
