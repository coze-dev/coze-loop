/* eslint-disable max-lines-per-function */
import createEle from 'crelt';
import {
  type EditorView,
  type Panel,
  runScopeHandlers,
  type ViewUpdate,
} from '@codemirror/view';
import { type EditorState } from '@codemirror/state';
import {
  closeSearchPanel,
  findNext,
  findPrevious,
  getSearchQuery,
  replaceAll,
  replaceNext,
  SearchQuery,
  setSearchQuery,
} from '@codemirror/search';

import { MatchCount } from './dom/match-count';
import {
  IconCaseSensitive,
  IconRegExp,
  IconWholeWord,
  IconArrowUp,
  IconArrowDown,
  IconReplace,
  IconReplaceAll,
  IconClose,
  IconChevronRight,
} from './dom/icon';

export class SearchPanel implements Panel {
  searchField!: HTMLInputElement;
  replaceField!: HTMLInputElement;
  matchCount: MatchCount;

  caseField!: IconCaseSensitive;
  reField!: IconRegExp;
  wordField!: IconWholeWord;
  expand!: IconChevronRight;
  arrowUp!: IconArrowUp;
  arrowDown!: IconArrowDown;
  replace!: IconReplace;
  replaceAll!: IconReplaceAll;
  dom: HTMLElement;
  query: SearchQuery;

  constructor(readonly view: EditorView) {
    this.commit = this.commit.bind(this);
    this.query = getSearchQuery(view.state);
    this.matchCount = new MatchCount();

    const searchLine = this.initSearchLine(view);

    const replaceLine = this.initReplaceLine(view);

    const replaceLineStatus = () => {
      if (view.state.readOnly) {
        replaceLine.style.display = 'none';
        return;
      }

      if (!this.expand?.checked) {
        replaceLine.style.display = 'none';
        return;
      }

      replaceLine.style.display = 'flex';
    };

    replaceLineStatus();

    const expand = this.initExpandButton(view, () => replaceLineStatus());

    const onMouseDown = useDragSearchPanel(this);

    this.dom = createEle(
      'div',
      {
        onkeydown: (e: KeyboardEvent) => this.keydown(e),
        class: 'cm-custom-search',
      },
      [
        createEle('div', {
          class: 'cm-custom-search-drag',
          onmousedown: onMouseDown,
        }),
        ...expand,
        createEle(
          'div',
          {
            class: 'cm-custom-search-panel-content',
          },
          [searchLine, replaceLine],
        ),
      ],
    );

    this.updateMatchCount(view.state, this.query);
  }

  private initExpandButton(view: EditorView, onExpandChange?: () => void) {
    this.expand = new IconChevronRight({
      name: 'expand',
      title: phrase(view, '展开'),
      onchange: onExpandChange,
    });

    return view.state.readOnly
      ? []
      : [
          createEle(
            'div',
            {
              class: 'cm-custom-search-panel-expand',
            },
            [this.expand.dom],
          ),
        ];
  }

  private initSearchLine(view: EditorView) {
    const inputWrapperRef: { current: null | HTMLDivElement } = {
      current: null,
    };

    const handleInputFocus = () => {
      inputWrapperRef.current?.focus();
      if (
        inputWrapperRef.current &&
        !inputWrapperRef.current.classList.contains(
          'cm-custom-search-panel-content-input-wrapper-focus',
        )
      ) {
        inputWrapperRef.current.classList.add(
          'cm-custom-search-panel-content-input-wrapper-focus',
        );
      }
    };

    const handleInputBlur = () => {
      inputWrapperRef.current?.blur();
      if (inputWrapperRef.current) {
        inputWrapperRef.current.classList.remove(
          'cm-custom-search-panel-content-input-wrapper-focus',
        );
      }
    };

    this.searchField = createEle('input', {
      value: this.query.search,
      placeholder: phrase(view, '查找'),
      title: phrase(view, '查找'),
      class: 'cm-custom-text-field',
      name: 'search',
      form: '',
      'main-field': 'true',
      onchange: this.commit,
      onkeyup: this.commit,
      onfocus: handleInputFocus,
      onblur: handleInputBlur,
    }) as HTMLInputElement;

    this.caseField = new IconCaseSensitive({
      checked: this.query.caseSensitive,
      onchange: () => {
        this.searchField.focus();
        this.commit();
      },
      name: 'case',
      title: phrase(view, '区分大小写'),
    });

    this.reField = new IconRegExp({
      name: 're',
      checked: this.query.regexp,
      onchange: () => {
        this.searchField.focus();
        this.commit();
      },
      title: phrase(view, '正则表达式'),
    });

    this.wordField = new IconWholeWord({
      name: 'word',
      checked: this.query.wholeWord,
      onchange: () => {
        this.searchField.focus();
        this.commit();
      },
      title: phrase(view, '全词匹配'),
    });

    // const enter = new IconEnter({
    //   name: 'enter',
    //   title: phrase(view, 'Next Line'),
    //   onchange: () => findNext(this.view),
    // });

    this.arrowUp = new IconArrowUp({
      name: 'previous',
      onclick: () => {
        findPrevious(view);
        this.updateMatchCount(view.state, this.query);
      },
      title: phrase(view, '上一个匹配'),
    });

    this.arrowDown = new IconArrowDown({
      name: 'next',
      onclick: () => {
        findNext(view);
        this.updateMatchCount(view.state, this.query);
      },
      title: phrase(view, '下一个匹配'),
    });

    inputWrapperRef.current = createEle(
      'div',
      {
        class: 'cm-custom-search-panel-content-input-wrapper',
      },
      [
        this.searchField,
        // enter.dom,
        this.caseField.dom,
        this.reField.dom,
        this.wordField.dom,
      ],
    ) as HTMLDivElement;

    return createEle(
      'div',
      {
        class: 'cm-custom-search-panel-content-top',
      },
      [
        inputWrapperRef.current,
        this.matchCount.dom,
        createEle(
          'div',
          {
            class: 'cm-custom-search-action-wrapper',
          },
          [
            this.arrowUp.dom,
            this.arrowDown.dom,
            new IconClose({
              name: 'close-icon',
              onclick: () => closeSearchPanel(view),
              title: phrase(view, '关闭'),
            }).dom,
          ],
        ),
      ],
    );
  }

  private initReplaceLine(view: EditorView) {
    const inputWrapperRef: { current: null | HTMLDivElement } = {
      current: null,
    };

    const handleInputFocus = () => {
      inputWrapperRef.current?.focus();
      if (
        inputWrapperRef.current &&
        !inputWrapperRef.current.classList.contains(
          'cm-custom-search-panel-content-input-wrapper-focus',
        )
      ) {
        inputWrapperRef.current.classList.add(
          'cm-custom-search-panel-content-input-wrapper-focus',
        );
      }
    };

    const handleInputBlur = () => {
      inputWrapperRef.current?.blur();
      if (inputWrapperRef.current) {
        inputWrapperRef.current.classList.remove(
          'cm-custom-search-panel-content-input-wrapper-focus',
        );
      }
    };

    this.replaceField = createEle('input', {
      value: this.query.replace,
      placeholder: phrase(view, '替换'),
      title: phrase(view, '替换'),
      class: 'cm-custom-text-field',
      name: 'replace',
      form: '',
      onchange: this.commit,
      onkeyup: this.commit,
      onfocus: handleInputFocus,
      onblur: handleInputBlur,
    }) as HTMLInputElement;

    this.replace = new IconReplace({
      name: 'replace',
      onclick: () => {
        replaceNext(view);
        this.updateMatchCount(view.state, this.query);
      },
      title: phrase(view, '替换'),
    });

    this.replaceAll = new IconReplaceAll({
      name: 'replaceAll',
      onclick: () => {
        replaceAll(view);
        this.updateMatchCount(view.state, this.query);
      },
      title: phrase(view, '替换所有'),
    });

    inputWrapperRef.current = createEle(
      'div',
      {
        class: 'cm-custom-search-panel-content-input-wrapper',
      },
      [this.replaceField],
    ) as HTMLDivElement;

    return createEle(
      'div',
      {
        class: 'cm-custom-search-panel-content-bottom',
      },
      [inputWrapperRef.current, this.replace.dom, this.replaceAll.dom],
    );
  }

  commit() {
    const query = new SearchQuery({
      search: this.searchField.value,
      caseSensitive: this.caseField.checked,
      regexp: this.reField.checked,
      wholeWord: this.wordField.checked,
      replace: this.replaceField.value,
    });

    if (!query.eq(this.query)) {
      this.query = query;
      this.view.dispatch({ effects: setSearchQuery.of(query) });
    }

    this.updateMatchCount(this.view.state, this.query);
  }

  keydown(e: KeyboardEvent) {
    if (runScopeHandlers(this.view, e, 'search-panel')) {
      e.preventDefault();
    } else if (
      (e.keyCode === 13 || e.key === 'Enter') &&
      e.target === this.searchField
    ) {
      e.preventDefault();
      (e.shiftKey ? findPrevious : findNext)(this.view);
    } else if (
      (e.keyCode === 13 || e.key === 'Enter') &&
      e.target === this.replaceField
    ) {
      e.preventDefault();
      replaceNext(this.view);
    }
  }

  updateMatchCount(state: EditorState, query: SearchQuery) {
    this.matchCount.data = {
      ...findMatchIndex(state, query),
      searching: !!query.search,
    };

    if (this.matchCount.data.matchCount > 0) {
      this.replace.disabled = false;
      this.replaceAll.disabled = false;
      this.arrowUp.disabled = false;
      this.arrowDown.disabled = false;
    } else {
      this.replace.disabled = true;
      this.replaceAll.disabled = true;
      this.arrowUp.disabled = true;
      this.arrowDown.disabled = true;
    }
  }

  update(update: ViewUpdate) {
    for (const tr of update.transactions) {
      for (const effect of tr.effects) {
        if (effect.is(setSearchQuery) && !effect.value.eq(this.query)) {
          this.setQuery(effect.value);
          this.updateMatchCount(update.state, effect.value);
        }
      }
    }
  }

  setQuery(query: SearchQuery) {
    this.query = query;
    this.searchField.value = query.search;
    this.replaceField.value = query.replace;
    this.caseField.checked = query.caseSensitive;
    this.reField.checked = query.regexp;
    this.wordField.checked = query.wholeWord;
  }

  mount() {
    this.searchField.select();
  }

  get top() {
    return true;
  }
}

function phrase(view: EditorView, p: string) {
  return view.state.phrase(p);
}
function findMatchIndex(state: EditorState, query: SearchQuery) {
  try {
    const { from, to } = state.selection.main;
    let index = 0;
    let matchIndex = -1;
    const cursor = query.getCursor(state);
    let data = cursor.next();
    while (!data.done) {
      if (data.value.from === from && data.value.to === to) {
        matchIndex = index;
      }
      data = cursor.next();
      index++;
    }

    return { matchIndex, matchCount: index };
  } catch (e) {
    return { matchIndex: -1, matchCount: 0 };
  }
}
function useDragSearchPanel(panel?: SearchPanel) {
  let isResizing = false;
  let startX = 0;
  let startWidth = 0;

  const onMouseMove = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();

    if (!isResizing || !panel?.dom) {
      return;
    }

    const newWidth = startX - e.clientX + startWidth;
    panel.dom.style.width = `${newWidth}px`;
  };

  const onMouseUp = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();

    isResizing = false;
    startX = 0;
    startWidth = 0;

    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', onMouseUp);
  };

  const onMouseDown = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();

    isResizing = true;
    startX = e.clientX;
    startWidth = panel?.dom.clientWidth ?? 0;

    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  };

  return onMouseDown;
}
