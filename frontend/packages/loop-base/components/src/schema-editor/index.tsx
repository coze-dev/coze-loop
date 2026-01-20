// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';

import {
  CodeMirrorJsonEditor,
  CodeMirrorRawTextEditor,
} from '../codemirror-editor';

interface SchemaEditorProps {
  value?: string;
  readOnly?: boolean;
  onChange?: (value?: string) => void;
  language?: string;
  placeholder?: string;
  showLineNumbs?: boolean;
  className?: string;
}

export const SchemaEditor = ({
  value,
  onChange,
  placeholder,
  readOnly,
  language,
  className,
}: SchemaEditorProps) => (
  <div
    className={classNames(
      'w-full h-[500px] border border-solid coz-stroke-primary rounded-[4px] overflow-hidden relative bg-white',
      { 'opacity-70': readOnly },
      className,
    )}
  >
    {language === 'json' ? (
      <CodeMirrorJsonEditor
        className="w-full h-full overflow-y-auto"
        onChange={onChange}
        value={value || ''}
        placeholder={placeholder}
        readonly={readOnly}
        borderRadius={4}
      />
    ) : (
      <CodeMirrorRawTextEditor
        className="w-full h-full overflow-y-auto"
        onChange={onChange}
        value={value || ''}
        placeholder={placeholder}
        readonly={readOnly}
      />
    )}
  </div>
);
