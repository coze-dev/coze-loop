// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-lines */
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable max-params */
/* eslint-disable complexity */
import { type SyntaxNode, type Tree } from '@lezer/common';
import { markdownLanguage } from '@coze-editor/editor/preset-prompt';
import {
  type StateCommand,
  type Text,
  type EditorState,
  EditorSelection,
  type ChangeSpec,
  countColumn,
  type Line,
  type TransactionSpec,
  type SelectionRange,
} from '@codemirror/state';
import { syntaxTree, indentUnit } from '@codemirror/language';

// --- 重构: 使用一个 props 对象来代替多个构造函数参数，以解决 `max-params` 问题 ---
interface ContextProps {
  node: SyntaxNode;
  from: number;
  to: number;
  spaceBefore: string;
  spaceAfter: string;
  type: string;
  item: SyntaxNode | null;
}

class Context {
  readonly node: SyntaxNode;
  readonly from: number;
  readonly to: number;
  readonly spaceBefore: string;
  readonly spaceAfter: string;
  readonly type: string;
  readonly item: SyntaxNode | null;

  constructor(props: ContextProps) {
    this.node = props.node;
    this.from = props.from;
    this.to = props.to;
    this.spaceBefore = props.spaceBefore;
    this.spaceAfter = props.spaceAfter;
    this.type = props.type;
    this.item = props.item;
  }

  blank(maxWidth: number | null, trailing = true): string {
    let result =
      this.spaceBefore + (this.node.name === 'Blockquote' ? '>' : '');
    if (maxWidth !== null) {
      while (result.length < maxWidth) {
        result += ' ';
      }
      return result;
    }

    for (
      let i = this.to - this.from - result.length - this.spaceAfter.length;
      i > 0;
      i--
    ) {
      result += ' ';
    }
    return result + (trailing ? this.spaceAfter : '');
  }

  marker(doc: Text, add: number): string {
    if (this.node.name === 'OrderedList' && this.item) {
      const numberMatch = itemNumber(this.item, doc);
      if (numberMatch) {
        const number = Number(numberMatch[2]) + add;
        return `${this.spaceBefore}${number}.${this.spaceAfter}`;
      }
    }
    return `${this.spaceBefore}${this.type}${this.spaceAfter}`;
  }
}

function getContext(node: SyntaxNode, doc: Text): Context[] {
  const context: Context[] = [];
  let current: SyntaxNode | null = node;

  while (current) {
    if (current.name === 'FencedCode') {
      break;
    }

    if (current.name === 'ListItem' || current.name === 'Blockquote') {
      const line = doc.lineAt(current.from);
      const startPos = current.from - line.from;
      const lineText = line.text.slice(startPos);
      let contextPart: Context | null = null;

      if (current.name === 'Blockquote') {
        const match = /^ *>( ?)/.exec(lineText);
        if (match) {
          contextPart = new Context({
            node: current,
            from: startPos,
            to: startPos + match[0].length,
            spaceBefore: '',
            spaceAfter: match[1],
            type: '>',
            item: null,
          });
        }
      } else if (current.name === 'ListItem' && current.parent) {
        if (current.parent.name === 'OrderedList') {
          const match = /^( *)(\d+)([.)])( *)/.exec(lineText);
          if (match) {
            let after = match[4];
            let len = match[0].length;
            if (after.length >= 4) {
              after = after.slice(0, after.length - 4);
              len -= 4;
            }
            contextPart = new Context({
              node: current.parent,
              from: startPos,
              to: startPos + len,
              spaceBefore: match[1],
              spaceAfter: after,
              type: match[2],
              item: current,
            });
          }
        } else if (current.parent.name === 'BulletList') {
          const match = /^( *)([-+*])( {1,4}\[[ xX]\])?( +)/.exec(lineText);
          if (match) {
            let after = match[4];
            let len = match[0].length;
            if (after.length > 4) {
              after = after.slice(0, after.length - 4);
              len -= 4;
            }
            let type = match[2];
            if (match[3]) {
              type += match[3].replace(/[xX]/, ' ');
            }
            contextPart = new Context({
              node: current.parent,
              from: startPos,
              to: startPos + len,
              spaceBefore: match[1],
              spaceAfter: after,
              type,
              item: current,
            });
          }
        }
      }

      if (contextPart) {
        context.unshift(contextPart); // Add to the beginning to maintain order
      }
    }
    current = current.parent;
  }
  return context;
}

// --- 修复: 移除 `!` 断言，使其返回可空类型 ---
function itemNumber(item: SyntaxNode, doc: Text): RegExpExecArray | null {
  return /^(\s*)(\d+)(?=[.)])/.exec(doc.sliceString(item.from, item.from + 10));
}

function renumberList(
  after: SyntaxNode,
  doc: Text,
  changes: ChangeSpec[],
  offset = 0,
) {
  let prev = -1;
  let node: SyntaxNode | null = after;
  while (node) {
    if (node.name === 'ListItem') {
      const m = itemNumber(node, doc);
      // --- 修复: 安全地处理 itemNumber 的可能 null 返回值 ---
      if (!m) {
        break;
      }

      const number = Number(m[2]);
      if (prev >= 0) {
        if (number !== prev + 1) {
          break;
        } // Not a continuous list
        changes.push({
          from: node.from + m[1].length,
          to: node.from + m[0].length,
          insert: String(prev + 2 + offset),
        });
      }
      prev = number;
    }
    node = node.nextSibling;
  }
}

function normalizeIndent(content: string, state: EditorState): string {
  // --- 修复: 安全地处理 `exec` 可能的 null 返回值 ---
  const match = /^[ \t]*/.exec(content);
  const blank = match ? match[0].length : 0;

  if (!blank || state.facet(indentUnit) !== '\t') {
    return content;
  }
  const col = countColumn(content, 4, blank);
  let space = '';
  for (let i = col; i > 0; ) {
    if (i >= 4) {
      space += '\t';
      i -= 4;
    } else {
      space += ' ';
      i--;
    }
  }
  return space + content.slice(blank);
}

// --- 辅助函数：处理列表中的空行 ---
function handleEmptyLineInList(
  state: EditorState,
  context: Context[],
  inner: Context,
  pos: number,
  line: Line,
): TransactionSpec | { range: SelectionRange } | null {
  if (!inner.item) {
    return null;
  }

  const { doc } = state;
  const firstItem = inner.node.firstChild;
  const secondItem = inner.node.getChild('ListItem', 'ListItem');

  if (!firstItem) {
    return null;
  }

  // 条件：不是列表的第二项，或者前面是空行 -> 删除一层标记
  if (
    firstItem.to >= pos ||
    (secondItem && secondItem.to < pos) ||
    (line.from > 0 && !/[^\s>]/.test(doc.lineAt(line.from - 1).text))
  ) {
    const changes: ChangeSpec[] = [];
    const nextContext = context.length > 1 ? context[context.length - 2] : null;
    let delTo: number;
    let insert = '';

    if (nextContext) {
      delTo =
        line.from + (nextContext.item ? nextContext.from : nextContext.to);
      if (nextContext.item) {
        insert = nextContext.marker(doc, 1);
      }
    } else {
      delTo = line.from;
    }

    changes.push({ from: delTo, to: pos, insert });

    if (inner.node.name === 'OrderedList' && inner.item) {
      renumberList(inner.item, doc, changes, -2);
    }
    if (nextContext?.node.name === 'OrderedList' && nextContext.item) {
      renumberList(nextContext.item, doc, changes);
    }

    return {
      range: EditorSelection.cursor(delTo + insert.length),
      changes,
    };
  }
  // 否则，在紧凑列表的两个项之间插入一个空行
  else {
    const insert = blankLine(context, state, line);
    return {
      range: EditorSelection.cursor(pos + insert.length + 1),
      changes: { from: line.from, insert: insert + state.lineBreak },
    };
  }
}

// --- 辅助函数：处理连续的空引用块 ---
function handleEmptyBlockquote(
  state: EditorState,
  inner: Context,
  line: Line,
): TransactionSpec | { range: SelectionRange } | null {
  if (inner.node.name !== 'Blockquote' || !line.from) {
    return null;
  }

  const prevLine = state.doc.lineAt(line.from - 1);
  const quoted = />\s*$/.exec(prevLine.text);

  if (quoted && quoted.index === inner.from) {
    const changes = state.changes([
      { from: prevLine.from + quoted.index, to: prevLine.to },
      { from: line.from + inner.from, to: line.to },
    ]);
    return {
      range: EditorSelection.cursor(changes.mapPos(line.from)),
      changes,
    };
  }
  return null;
}

// --- 辅助函数：创建新行并延续标记 ---
function continueMarkup(
  state: EditorState,
  context: Context[],
  inner: Context,
  pos: number,
  line: Line,
) {
  const { doc } = state;
  const changes: ChangeSpec[] = [];
  if (inner.node.name === 'OrderedList' && inner.item) {
    renumberList(inner.item, doc, changes);
  }

  const continued = !!inner.item && inner.item.from < line.from;
  let insert = '';
  const lineIndentMatch = /^[\s\d.)\-+*>]*/.exec(line.text);
  const lineIndentLen = lineIndentMatch ? lineIndentMatch[0].length : 0;

  if (!continued || lineIndentLen >= inner.to) {
    for (let i = 0; i < context.length; i++) {
      const c = context[i];
      if (i === context.length - 1 && !continued) {
        console.log('m', doc);
        insert += c.marker(doc, 1);
      } else {
        const maxWidth =
          i < context.length - 1
            ? countColumn(line.text, 4, context[i + 1].from) - insert.length
            : null;
        insert += c.blank(maxWidth);
      }
    }
  }

  let from = pos;
  while (
    from > line.from &&
    /\s/.test(line.text.charAt(from - line.from - 1))
  ) {
    from--;
  }

  insert = normalizeIndent(insert, state);
  if (nonTightList(inner.node, state.doc)) {
    insert = blankLine(context, state, line) + state.lineBreak + insert;
  }

  changes.push({ from, to: pos, insert: state.lineBreak + insert });

  return {
    range: EditorSelection.cursor(from + insert.length + 1),
    changes,
  };
}

export const insertNewlineContinueMarkup: StateCommand = ({
  state,
  dispatch,
}) => {
  const tree = syntaxTree(state);
  const { doc } = state;
  let fallthrough = false;

  const changes = state.changeByRange(range => {
    if (!range.empty || !markdownLanguage.isActiveAt(state, range.from)) {
      fallthrough = true;
      return { range };
    }

    const pos = range.from;
    const line = doc.lineAt(pos);
    const context = getContext(tree.resolveInner(pos, -1), doc);

    while (
      context.length &&
      context[context.length - 1].from > pos - line.from
    ) {
      context.pop();
    }

    if (!context.length) {
      fallthrough = true;
      return { range };
    }

    const inner = context[context.length - 1];

    if (inner.to - inner.spaceAfter.length > pos - line.from) {
      fallthrough = true;
      return { range };
    }

    const isAtEndOfMarkup = pos >= inner.to - inner.spaceAfter.length;
    const isLineEmptyAfterMarkup = !/\S/.test(
      line.text.slice(inner.to - (line.from + inner.from)),
    );

    if (isAtEndOfMarkup && isLineEmptyAfterMarkup) {
      const emptyLineInListResult = handleEmptyLineInList(
        state,
        context,
        inner,
        pos,
        line,
      );
      if (emptyLineInListResult) {
        return emptyLineInListResult;
      }

      const emptyBlockquoteResult = handleEmptyBlockquote(state, inner, line);
      if (emptyBlockquoteResult) {
        return emptyBlockquoteResult;
      }
    }

    return continueMarkup(state, context, inner, pos, line) as any;
  });

  if (fallthrough || changes.changes.empty) {
    return false;
  }

  dispatch(state.update(changes, { scrollIntoView: true, userEvent: 'input' }));
  return true;
};

// ... (其他辅助函数保持不变，但为了完整性包含在此)

function nonTightList(node: SyntaxNode, doc: Text): boolean {
  if (node.name !== 'OrderedList' && node.name !== 'BulletList') {
    return false;
  }

  const first = node.firstChild;
  const second = node.getChild('ListItem', 'ListItem');
  if (!first || !second) {
    return false;
  }

  const line1 = doc.lineAt(first.to);
  const line2 = doc.lineAt(second.from);
  const empty = /^[\s>]*$/.test(line1.text);
  return line1.number + (empty ? 0 : 1) < line2.number;
}

function blankLine(context: Context[], state: EditorState, line: Line): string {
  let insert = '';
  for (let i = 0; i < context.length - 1; i++) {
    const c = context[i];
    const next = context[i + 1];
    const maxWidth = countColumn(line.text, 4, next.from) - insert.length;
    insert += c.blank(maxWidth, true);
  }
  return normalizeIndent(insert, state);
}

function contextNodeForDelete(tree: Tree, pos: number): SyntaxNode {
  let node = tree.resolveInner(pos, -1);
  let scan = pos;

  const isMark = (n: SyntaxNode) =>
    n.name === 'QuoteMark' || n.name === 'ListMark';

  if (isMark(node) && node.parent) {
    scan = node.from;
    node = node.parent;
  }

  while (true) {
    const prev = node.childBefore(scan);
    if (!prev) {
      break;
    }

    if (isMark(prev)) {
      scan = prev.from;
    } else if (
      (prev.name === 'OrderedList' || prev.name === 'BulletList') &&
      prev.lastChild
    ) {
      node = prev.lastChild;
      scan = node.to;
    } else {
      break;
    }
  }
  return node;
}

export const deleteMarkupBackward: StateCommand = ({ state, dispatch }) => {
  const tree = syntaxTree(state);
  let fallthrough = false;

  const changes = state.changeByRange(range => {
    const { from: pos, to } = range;
    if (to !== pos || !markdownLanguage.isActiveAt(state, pos)) {
      fallthrough = true;
      return { range };
    }

    const line = state.doc.lineAt(pos);
    const context = getContext(contextNodeForDelete(tree, pos), state.doc);

    if (!context.length) {
      fallthrough = true;
      return { range };
    }

    const inner = context[context.length - 1];
    const spaceEnd =
      inner.to - inner.spaceAfter.length + (inner.spaceAfter ? 1 : 0);

    // Case 1: Delete extra trailing space after markup
    if (
      pos - line.from > spaceEnd &&
      !/\S/.test(line.text.slice(spaceEnd, pos - line.from))
    ) {
      const targetPos = line.from + spaceEnd;
      return {
        range: EditorSelection.cursor(targetPos),
        changes: { from: targetPos, to: pos },
      };
    }

    const isOnSyntaxLine =
      !inner.item ||
      line.from <= inner.item.from ||
      !/\S/.test(line.text.slice(0, inner.to));
    if (pos - line.from === spaceEnd && isOnSyntaxLine) {
      const start = line.from + inner.from;
      // Case 2: Replace a list item marker with blank space
      if (
        inner.item &&
        inner.node.from < inner.item.from &&
        /\S/.test(line.text.slice(inner.from, inner.to))
      ) {
        let insert = inner.blank(
          countColumn(line.text, 4, inner.to) -
            countColumn(line.text, 4, inner.from),
        );
        if (start === line.from) {
          insert = normalizeIndent(insert, state);
        }
        return {
          range: EditorSelection.cursor(start + insert.length),
          changes: { from: start, to: line.from + inner.to, insert },
        };
      }
      // Case 3: Delete one level of indentation/markup
      if (start < pos) {
        return {
          range: EditorSelection.cursor(start),
          changes: { from: start, to: pos },
        };
      }
    }

    fallthrough = true;
    return { range };
  });
  if (fallthrough || changes.changes.empty) {
    return false;
  }
  dispatch(
    state.update(changes, { scrollIntoView: true, userEvent: 'delete' }),
  );
  return true;
};

export const insertFourSpaces: StateCommand = ({ state, dispatch }) => {
  // 如果有选中区域，我们希望执行默认的缩进行为，而不是替换
  // 所以返回 false，让 keymap 继续查找下一个匹配项 (即 indentWithTab)
  if (!state.selection.main.empty) {
    return false;
  }

  dispatch(
    state.update(
      {
        changes: { from: state.selection.main.head, insert: '  ' },
        // 插入后，将光标移动到空格后面
        selection: EditorSelection.cursor(state.selection.main.head + 2),
      },
      { userEvent: 'input' },
    ),
  );

  // 返回 true 表示我们已经处理了这次按键，无需再执行其他命令
  return true;
};
