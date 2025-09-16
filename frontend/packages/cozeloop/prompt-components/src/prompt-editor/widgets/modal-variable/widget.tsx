// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type Root } from 'react-dom/client';
import classNames from 'classnames';
import { TooltipWhenDisabled } from '@cozeloop/components';
import { IconCozCrossCircleFill } from '@coze-arch/coze-design/icons';
import { Icon } from '@coze-arch/coze-design';
import { WidgetType, type EditorView } from '@codemirror/view';

import { ReactComponent as ModalVariableIcon } from '@/assets/modal-variable.svg';

import { renderDom } from '../render-dom';

import styles from './index.module.less';

export interface ModalVariableDataInfo {
  variableKey?: string;
  uuid?: string;
}

interface ModalVariableDisplayProps {
  dataInfo?: ModalVariableDataInfo;
  readonly?: boolean;
  isMultimodal?: boolean;
  disabled?: boolean;
  disabledTip?: string;
  onDelete?: () => void;
}

const ModalVariableDisplay: React.FC<ModalVariableDisplayProps> = ({
  dataInfo,
  readonly,
  isMultimodal,
  onDelete,
  disabled,
  disabledTip = '当前 Message 不支持多模态，请调整变量类型或更换 Message 类型',
}) => (
  <TooltipWhenDisabled
    content={
      disabled ? disabledTip : '所选模型不支持多模态，请调整变量类型或更换模型'
    }
    theme="dark"
    disabled={!isMultimodal || disabled}
  >
    <div
      className={classNames(styles['modal-variable-widget'], {
        [styles['modal-variable-widget-disabled']]: !isMultimodal || disabled,
      })}
    >
      <Icon svg={<ModalVariableIcon fontSize={13} />} size="extra-small" />
      {readonly ? null : (
        <IconCozCrossCircleFill
          fontSize={12}
          className={styles['modal-variable-widget-delete']}
          onClick={onDelete}
        />
      )}
      <span className="text-[13px]">{dataInfo?.variableKey}</span>
    </div>
  </TooltipWhenDisabled>
);

interface ModalVariableWidgetOptions extends ModalVariableDisplayProps {
  from: number;
  to: number;
}

export class ModalVariableWidget extends WidgetType {
  root?: Root;

  constructor(public options: ModalVariableWidgetOptions) {
    super();
  }

  toDOM(view: EditorView): HTMLElement {
    const { root, dom } = renderDom<ModalVariableDisplayProps>(
      ModalVariableDisplay,
      {
        disabled: this.options.disabled,
        dataInfo: this.options.dataInfo,
        readonly: this.options.readonly,
        onDelete: this.options.onDelete,
        isMultimodal: this.options.isMultimodal,
        disabledTip: this.options.disabledTip,
      },
    );

    this.root = root;
    return dom;
  }

  getEqKey() {
    return [
      this.options.dataInfo?.variableKey,
      this.options.dataInfo?.uuid,
      this.options.isMultimodal,
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
