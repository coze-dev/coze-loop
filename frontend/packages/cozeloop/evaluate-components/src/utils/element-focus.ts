// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export const elementFocus = (id: string) => {
  setTimeout(() => {
    const element = document.getElementById(id);
    if (element) {
      element.scrollIntoView();
      highlightCollapse(element);
    }
  });
};

export const highlightCollapse = (element: HTMLElement | null) => {
  const hlElement = element?.querySelector('.semi-collapse-item');
  if (hlElement) {
    highlightElement(hlElement as HTMLElement);
  }
};

export const highlightElement = (element?: HTMLElement) => {
  if (!element) {
    return;
  }
  element.style.border = '1px solid rgba(var(--coze-up-brand-9),1)';
  setTimeout(() => {
    element.style.border = '';
  }, 2000);
};
