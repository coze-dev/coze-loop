import { type ReactJsonViewProps } from 'react-json-view';

export const jsonViewerConfig: Partial<ReactJsonViewProps> = {
  name: false,
  displayDataTypes: false,
  indentWidth: 2,
  iconStyle: 'triangle',
  enableClipboard: false,
  collapsed: 5,
  collapseStringsAfterLength: 300,
};
