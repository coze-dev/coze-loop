import { type EditorView, keymap } from '@codemirror/view';
import {
  gotoLine,
  searchKeymap,
  search as nativeSearch,
} from '@codemirror/search';

import { theme } from './theme';
import { SearchPanel } from './panel';

/*
 * TODO
 * 1. 考虑如何让用户自定义Panel
 * 2. 展示搜索组建时顶部可以额外滚动
 * 3. 组件 Search 拆分
 * */

export const search = () => [
  theme,
  nativeSearch({
    createPanel: (view: EditorView) => new SearchPanel(view),
  }),
  keymap.of([...searchKeymap, { key: 'Ctrl-g', run: gotoLine }]),
];
