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
  onDelete?: () => void;
}

const ModalVariableDisplay: React.FC<ModalVariableDisplayProps> = ({
  dataInfo,
  readonly,
  isMultimodal,
  onDelete,
}) => (
  <TooltipWhenDisabled
    content="所选模型不支持多模态，请调整变量类型或更换模型"
    theme="dark"
    disabled={!isMultimodal}
  >
    <div
      className={classNames(styles['modal-variable-widget'], {
        [styles['modal-variable-widget-disabled']]: !isMultimodal,
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
        dataInfo: this.options.dataInfo,
        readonly: this.options.readonly,
        onDelete: this.options.onDelete,
        isMultimodal: this.options.isMultimodal,
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
