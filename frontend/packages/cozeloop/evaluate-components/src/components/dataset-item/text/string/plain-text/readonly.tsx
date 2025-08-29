import cn from 'classnames';
import { Typography } from '@coze-arch/coze-design';

import styles from '../index.module.less';
import { type DatasetItemProps } from '../../../type';
export const PlainTextDatasetItemReadOnly = ({
  fieldContent,
  expand,
  className,
  displayFormat,
}: DatasetItemProps) =>
  expand ? (
    <Typography.Text
      style={{ color: 'inherit' }}
      className={cn(
        'block !text-[13px] max-h-[292px] overflow-y-auto break-all',
        displayFormat && styles.border,
        className,
      )}
    >
      {fieldContent?.text}
    </Typography.Text>
  ) : (
    <Typography.Text
      style={{ color: 'inherit' }}
      className={cn('!text-[13px] overflow-hidden break-all', className)}
      ellipsis={{
        showTooltip: {
          opts: {
            // theme: 'dark',
            content: (
              <div className="max-h-[200px] overflow-auto">
                {fieldContent?.text}
              </div>
            ),
          },
        },
        rows: 1,
      }}
    >
      {fieldContent?.text}
    </Typography.Text>
  );
