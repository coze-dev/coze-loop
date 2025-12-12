import cls from 'classnames';
import { SpanType } from '@cozeloop/api-schema/observation';
import { IconCozCrossFill } from '@coze-arch/coze-design/icons';
import { Button, Typography } from '@coze-arch/coze-design';

import { getNodeConfig } from '@/features/trace-detail/utils/span';
import {
  BROKEN_ROOT_SPAN_ID,
  NORMAL_BROKEN_SPAN_ID,
} from '@/features/trace-detail/constants/span';

import { type TraceHeaderProps } from '../typing';

import styles from './index.module.less';

export const VerticalTraceHeader = ({
  rootSpan,
  className: propsClassName,
  showClose,
  onClose,
}: TraceHeaderProps) => {
  const { type, span_name = '', span_id, span_type } = rootSpan || {};
  const isBroken = [NORMAL_BROKEN_SPAN_ID, BROKEN_ROOT_SPAN_ID].includes(
    span_id || '',
  );
  const traceName = isBroken ? 'Unknown Trace' : span_name;
  const nodeConfig = getNodeConfig({
    spanTypeEnum: type ?? SpanType.Unknown,
    spanType: span_type ?? SpanType.Unknown,
  });

  return (
    <div className={cls(styles['vertical-header'], propsClassName)}>
      <div className="flex min-w-0 items-center mb-2 gap-2">
        {showClose ? (
          <Button
            type="primary"
            color="secondary"
            icon={<IconCozCrossFill />}
            onClick={onClose}
            size="small"
          />
        ) : null}
        <div className="flex flex-1 gap-2 items-center">
          <span className={styles['icon-wrapper']}>
            {nodeConfig.icon?.({ className: '!w-[16px] !h-[16px]' })}
          </span>
          <div className={styles.desc}>
            <div className={cls(styles.name, 'flex items-center flex-1 gap-1')}>
              <Typography.Text
                ellipsis={{
                  showTooltip: {
                    type: 'tooltip',
                    opts: {
                      position: 'bottom',
                      theme: 'dark',
                    },
                  },
                }}
                className="coz-fg-plus"
              >
                {traceName}
              </Typography.Text>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
