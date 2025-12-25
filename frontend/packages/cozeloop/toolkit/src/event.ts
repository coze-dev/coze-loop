// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/*
 * 获取事件冒泡路径，兼容ie11,edge,chrome,firefox,safari
 * @param evt
 * @returns {*}
 */
export function eventPath(evt: Event): Element[] {
  const path = (evt.composedPath && evt.composedPath()) || (evt as any).path,
    { target } = evt;

  // 额外类型断言，确保 target 存在于 EventTarget | null
  if (target === null) {
    throw new Error('Event target cannot be null');
  }

  if (path !== null) {
    return (path.indexOf(window) < 0 ? path.concat(window) : path) as Element[];
  }

  if (target === window) {
    return [];
  }

  function getParents(node: Node | null, memo: Node[] = []): Node[] {
    const parentNode = node ? node.parentNode : null;

    if (!parentNode) {
      return memo;
    } else {
      return getParents(parentNode, memo.concat(parentNode));
    }
  }

  // 这里需要注意的是，target 和 parentNode 类型不一样，但都可以认为是 EventTarget
  // 为了类型安全，我们将 parentNode 断言为 EventTarget
  return [target, ...getParents(target as Node)] as Element[];
}

/**
 * 递归寻找btm dom节点
 */
export function findBtmNode(dom: HTMLElement | null): HTMLElement | null {
  let current = dom;
  while (current) {
    // 判断是否有data-btm属性，且是否是d位
    const btmId = current.getAttribute('data-btm');
    if (btmId && /^d(\d+)$/.test(btmId)) {
      return current;
    }

    current = current.parentElement;
  }
  return null;
}
