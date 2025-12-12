/* eslint-disable complexity */

import { type Root } from 'react-dom/client';
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Prompt } from '@cozeloop/api-schema/prompt';
import {
  IconCozArrowDown,
  IconCozArrowRight,
  IconCozTrashCan,
} from '@coze-arch/coze-design/icons';
import { Popconfirm, Popover, Spin, Typography } from '@coze-arch/coze-design';
import { type EditorView, WidgetType } from '@codemirror/view';

import { BasicPromptEditor } from '@/components/basic-prompt-editor';

import { renderDom } from '../render-dom';
import { useSegmentWidget } from './segment-widget-hooks';
import { SegmentVersionChange } from './segment-version-change';
import SegmentCompletion from './segment-completion';

import css from './segment-widget.module.less';

interface SegmentDisplayProps {
  segment?: Prompt;
  onDelete?: (id?: Int64) => void;
  onItemClick?: (info?: Prompt) => void;
  readonly?: boolean;
  stateType?: string;
}

const SegmentDisplay: React.FC<SegmentDisplayProps> = ({
  segment,
  onDelete,
  onItemClick,
  readonly,
  stateType,
}) => {
  const { isActive, toggleActive, segmentInfo, ladoingSegment } =
    useSegmentWidget({
      spaceID: `${segment?.workspace_id || ''}`,
      segmengId: segment?.id,
      sgementVersion: segment?.prompt_commit?.commit_info?.version,
      hasSubPrompt:
        segment?.prompt_commit?.detail?.prompt_template?.has_snippet ||
        !segment?.prompt_commit?.detail?.prompt_template?.messages,
    });
  const snippet = segmentInfo || segment;

  const handleItemClick = (info?: Prompt) => {
    if (info) {
      onItemClick?.(info);
    }
  };

  const handleDelete = (e: React.MouseEvent) => {
    e.stopPropagation();
    onDelete?.(snippet?.id);
  };

  return (
    <div
      className={classNames('fornax-sgement inline-flex gap-y-1 m-0.5', {
        'max-w-[200px]': !isActive,
        'w-full': isActive,
        'flex-col': isActive,
        'mx-0': isActive,
        'bg-[rgba(238,68,51,0.3)]': stateType === 'delete',
        'bg-[rgba(34,184,24,0.3)]': stateType === 'add',
      })}
    >
      <div
        className={classNames(css['segment-widget'], {
          [css.active]: isActive,
        })}
      >
        <div className="flex items-center gap-1 flex-1 overflow-hidden">
          <IconCozArrowRight
            onClick={toggleActive}
            className={classNames('cursor-pointer', css['left-icon'])}
          />

          <Typography.Text
            className={css.name}
            size="small"
            ellipsis={{ showTooltip: true }}
          >
            {snippet?.prompt_basic?.display_name}
          </Typography.Text>
          {!readonly ? (
            <Popover
              content={
                <SegmentVersionChange
                  segmentInfo={snippet}
                  onItemClick={handleItemClick}
                />
              }
              trigger={readonly ? 'hover' : 'click'}
              stopPropagation
            >
              <span className={css.version}>
                {snippet?.prompt_commit?.commit_info?.version}
                <IconCozArrowDown />
              </span>
            </Popover>
          ) : (
            <span className={css.version}>
              {snippet?.prompt_commit?.commit_info?.version}
            </span>
          )}
        </div>

        {readonly ? null : (
          <Popconfirm
            title={I18n.t('prompt_confirm_delete_section')}
            onConfirm={handleDelete}
            okText={I18n.t('confirm')}
            cancelText={I18n.t('cancel')}
            okButtonColor="red"
          >
            <IconCozTrashCan
              className={classNames(css.icon, 'flex-shrink-0')}
              onClick={e => {
                e.stopPropagation();
              }}
            />
          </Popconfirm>
        )}
      </div>
      {isActive ? (
        <div className={css['segment-info']}>
          {ladoingSegment ? (
            <Spin wrapperClassName="text-center w-full" />
          ) : (
            <BasicPromptEditor
              key={
                snippet?.prompt_commit?.detail?.prompt_template?.messages?.[0]
                  ?.content
              }
              defaultValue={
                snippet?.prompt_commit?.detail?.prompt_template?.messages?.[0]
                  ?.content || ''
              }
              readOnly
              linePlaceholder=""
            >
              <SegmentCompletion />
            </BasicPromptEditor>
          )}
        </div>
      ) : null}
    </div>
  );
};

interface SegmentWidgetOptions {
  segment?: Prompt;
  onDelete?: (id?: Int64) => void;
  onItemClick?: (info?: Prompt) => void;
  stateType?: string;
  readonly?: boolean;
  from: number;
  to: number;
}

export class SgementWidget extends WidgetType {
  root?: Root;

  constructor(public options: SegmentWidgetOptions) {
    super();
  }

  toDOM(view: EditorView): HTMLElement {
    const { root, dom } = renderDom<SegmentDisplayProps>(SegmentDisplay, {
      segment: this.options.segment,
      onDelete: this.options.onDelete,
      onItemClick: this.options.onItemClick,
      readonly: this.options.readonly,
      stateType: this.options.stateType,
    });
    this.root = root;
    return dom;
  }

  getEqKey() {
    return [
      this.options.segment?.id,
      this.options.segment?.prompt_commit?.commit_info?.version,
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
