import { Rule } from 'eslint';

export const noDestructuringUseRequestRule: Rule.RuleModule = {
  meta: {
    type: 'suggestion',
    docs: {
      description: 'Should not destructure useRequest',
      recommended: true,
    },
    messages: {
      noDestructuring:
        'Do not destructure useRequest return value. Use "const request = useRequest()" instead.',
    },
    schema: [],
  },

  create(context) {
    return {
      VariableDeclarator(node) {
        // 检查初始化表达式是否是 useRequest 调用
        if (
          node.init &&
          node.init.type === 'CallExpression' &&
          node.init.callee.type === 'Identifier' &&
          node.init.callee.name === 'useRequest' &&
          node.id.type === 'ObjectPattern'
        ) {
          context.report({
            node: node.id,
            messageId: 'noDestructuring',
          });
        }
      },
    };
  },
};
