// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createRoot, type Root } from 'react-dom/client';

interface RenderDomResult {
  dom: HTMLSpanElement;
  root: Root;
  destroy: () => void;
}

export const renderDom = <T extends {}>(
  // eslint-disable-next-line @typescript-eslint/naming-convention
  Comp: React.ComponentType<T>,
  props: T,
): RenderDomResult => {
  const dom = document.createElement('span');
  const root = createRoot(dom);
  root.render(<Comp {...props} />);

  return {
    dom,
    root,
    destroy() {
      root.unmount();
    },
  };
};
