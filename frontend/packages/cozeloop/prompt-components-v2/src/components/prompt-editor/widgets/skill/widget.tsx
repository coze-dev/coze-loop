// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/no-deep-relative-import */
/* eslint-disable @typescript-eslint/no-explicit-any */

import { type Root } from 'react-dom/client';
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { Tooltip } from '@coze-arch/coze-design';
import { type EditorView, WidgetType } from '@codemirror/view';

import { renderDom } from '../render-dom';
import {
  pluginIcon,
  workflowIcon,
  imageflowIcon,
  tableIcon,
  textIcon,
  imageIcon,
  volcanoIcon,
} from '../../../../assets/library-block';

import styles from './index.module.less';

export interface SkillDataInfo {
  apiId?: string;
  id?: string;
  type?: string;
  uuid?: string;
}

interface SkillDisplayProps {
  librarys?: any[];
  dataInfo?: SkillDataInfo;
  readonly?: boolean;
}

const defaultLibraryBlockInfo: Record<
  string,
  {
    icon: string;
  }
> = {
  plugin: {
    icon: pluginIcon,
  },
  workflow: {
    icon: workflowIcon,
  },
  imageflow: {
    icon: imageflowIcon,
  },
  table: {
    icon: tableIcon,
  },
  text: {
    icon: textIcon,
  },
  image: {
    icon: imageIcon,
  },
  volcanoStructured: {
    icon: volcanoIcon,
  },
  volcanoUnstructured: {
    icon: volcanoIcon,
  },
};

function SkillDisPlay({ librarys, dataInfo }: SkillDisplayProps) {
  const library = librarys?.find(it => it.id === dataInfo?.id);

  return (
    <Tooltip
      content={I18n.t('prompt_kouzi_compass_no_skill_reference_debug')}
      theme="dark"
    >
      <span className={classNames(styles['skill-widget'])}>
        <img
          src={defaultLibraryBlockInfo[dataInfo?.type ?? '']?.icon}
          className="w-3 h-3"
        />

        <span>{library?.name || dataInfo?.uuid}</span>
      </span>
    </Tooltip>
  );
}

interface SkillWidgetOptions {
  librarys?: any[];
  dataInfo?: {
    apiId?: string;
    id?: string;
    type?: string;
    uuid?: string;
  };
  readonly?: boolean;
  from: number;
  to: number;
}

export class SkillWidget extends WidgetType {
  root?: Root;

  constructor(public options: SkillWidgetOptions) {
    super();
  }

  toDOM(view: EditorView): HTMLElement {
    const { root, dom } = renderDom<SkillDisplayProps>(SkillDisPlay, {
      librarys: this.options.librarys,
      dataInfo: this.options.dataInfo,
      readonly: this.options.readonly,
    });
    this.root = root;
    return dom;
  }

  getEqKey() {
    return [
      this.options.dataInfo?.id,
      this.options.dataInfo?.type,
      this.options.dataInfo?.uuid,
      this.options.from,
      this.options.to,
    ].join('');
  }

  eq(prev) {
    return prev.getEqKey() === this.getEqKey();
  }

  destroy(): void {
    this.root?.unmount();
  }
}
