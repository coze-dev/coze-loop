// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { IconCozCopy } from '@coze-arch/coze-design/icons';

import { handleCopy } from '../utils/basic';
import { CodeEditor } from '../code-editor';

import styles from './code-item.module.less';

export enum SupportedLang {
  Golang = 'go',
  Js = 'javascript',
  Python = 'python',
}

interface UsageContentProps {
  content: string;
  copyable?: boolean;
  lang?: SupportedLang;
  height?: number;
}

const UsageContent = ({
  content,
  copyable,
  lang = SupportedLang.Js,
  height = 300,
}: UsageContentProps) => (
  <div className="bg-[var(--semi-color-fill-0)] p-[16px] rounded-[8px] text-[14px] flex">
    <div className="flex-1 whitespace-pre-wrap break-all">
      <CodeEditor
        width={'100%'}
        height={height}
        language={lang}
        value={content}
        theme={'code-block-grey'}
        options={{
          minimap: { enabled: false },
          wordWrap: 'on',
          scrollBeyondLastLine: false,
          unicodeHighlight: { ambiguousCharacters: false },
          lineNumbers: 'on',
          formatOnPaste: true,
          readOnly: true,
        }}
        beforeMount={monaco => {
          monaco.editor.defineTheme('code-block-grey', {
            base: 'vs',
            inherit: true,
            rules: [],
            colors: {
              'editor.background': '#F4F4F4', // Set the background color of the editor
            },
          });
        }}
      />
    </div>
    <div className="ml-[8px]">
      {copyable ? (
        <IconCozCopy
          className="ml-[4px] text-[var(--coz-fg-dim)] cursor-pointer hover:coz-fg-primary"
          onClick={() => handleCopy(content || '')}
        />
      ) : null}
    </div>
  </div>
);

interface UsageItemProps {
  title?: React.ReactNode;
  content?: string;
  lang?: SupportedLang;
  contentHeight?: number;
}

export const UsageItem = ({
  title,
  content,
  lang,
  contentHeight,
}: UsageItemProps) => (
  <div className={styles['usage-detail']}>
    <div className="text-[#16px] font-[500] mb-4">{title}</div>
    {content ? (
      <UsageContent
        content={content}
        copyable={true}
        lang={lang}
        height={contentHeight}
      />
    ) : null}
  </div>
);
