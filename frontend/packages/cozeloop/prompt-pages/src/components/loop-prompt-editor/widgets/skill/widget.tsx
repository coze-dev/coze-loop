/* eslint-disable @coze-arch/no-deep-relative-import */
/* eslint-disable @typescript-eslint/no-explicit-any */

import { type Root } from 'react-dom/client';
import classNames from 'classnames';
import { type EditorView, WidgetType } from '@cozeloop/prompt-components';
import { Tooltip } from '@coze-arch/coze-design';

import { renderDom } from '../render-dom';

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

import {
  pluginIcon,
  workflowIcon,
  imageflowIcon,
  tableIcon,
  textIcon,
  imageIcon,
  volcanoIcon,
} from '../../../../assets/library-block';

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

const SkillDisPlay: React.FC<SkillDisplayProps> = ({
  librarys,
  dataInfo,
  readonly,
}) => {
  const library = librarys?.find(it => it.id === dataInfo?.id);

  return (
    <Tooltip content="扣子罗盘暂不支持技能引用与调试" theme="dark">
      <span className={classNames(styles['skill-widget'])}>
        <img
          src={defaultLibraryBlockInfo[dataInfo?.type ?? '']?.icon}
          className="w-3 h-3"
        />
        <span>{library?.name || dataInfo?.uuid}</span>
      </span>
    </Tooltip>
  );
};

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
