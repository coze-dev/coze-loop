import { isEmpty } from 'lodash-es';
import { IconCozLink } from '@coze-arch/coze-design/icons';
import { Typography } from '@coze-arch/coze-design';

import { getPartUrl } from '@/features/trace-data/utils/span';
import { type Span, type RawMessage } from '@/features/trace-data/types';

import { TraceImage } from './image';

import styles from './index.module.less';

interface MessagePartsProps {
  raw: RawMessage;
  attrTos?: Span['attr_tos'];
}

export const MessageParts = (props: MessagePartsProps) => {
  const { raw, attrTos } = props;
  if (isEmpty(raw.parts)) {
    return null;
  }
  return (
    <>
      {raw.parts?.map((part, ind) => {
        const fileUrl =
          part.type === 'file_url'
            ? getPartUrl(part?.file_url?.url, attrTos)
            : null;
        const imageUrl =
          part.type === 'image_url'
            ? getPartUrl(part?.image_url?.url, attrTos)
            : null;

        if (imageUrl) {
          return (
            <div key={ind} className="mb-2">
              <div className={styles['tool-title']}>
                <TraceImage url={imageUrl} />
              </div>
            </div>
          );
        }
        if (fileUrl) {
          return (
            <>
              <Typography.Text
                link={{
                  href: fileUrl,
                  target: '_blank',
                }}
              >
                <span className="flex items-center gap-x-1">
                  <IconCozLink className="text-brand-9 !w-[14px] !h-[14px]" />
                  <span>{part?.file_url?.name ?? '-'}</span>
                </span>
              </Typography.Text>
            </>
          );
        }

        return (
          <div key={ind} className="mb-2">
            <div className={styles['tool-title']}>{part.text}</div>
          </div>
        );
      })}
    </>
  );
};
