// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isEmpty } from 'lodash-es';
import { IconCozLink } from '@coze-arch/coze-design/icons';
import { Tag, Typography } from '@coze-arch/coze-design';

import { getPartUrl } from '../utils/span';
import { type Span, type RawMessage } from '../types';
import { TraceImage } from './image';

import styles from './index.module.less';

interface MessagePartsProps {
  raw: RawMessage;
  attrTos?: Span['attr_tos'];
}

function IconImageVariable() {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 14 14"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path
        d="M9.89258 7.79004C10.3431 7.79004 10.7448 8.07283 10.8965 8.49707L11.1123 9.10059L11.8213 8.19727C12.0234 7.93999 12.333 7.79006 12.6602 7.79004H13.4678C13.7619 7.79022 14.0007 8.02813 14.001 8.32227C14.001 8.61661 13.7621 8.85528 13.4678 8.85547H12.6602L11.5361 10.2871L11.9775 11.5215H12.668C12.9622 11.5216 13.201 11.7604 13.2012 12.0547C13.2012 12.3491 12.9623 12.5877 12.668 12.5879H11.9775C11.527 12.5879 11.1243 12.3051 10.9727 11.8809L10.7578 11.2773L10.0479 12.1807C9.84578 12.4378 9.53695 12.5878 9.20996 12.5879H8.40234C8.10788 12.5879 7.86914 12.3491 7.86914 12.0547C7.86926 11.7603 8.10796 11.5215 8.40234 11.5215H9.20996L10.334 10.0908L9.89258 8.85547H9.20215C8.90769 8.85547 8.66895 8.61673 8.66895 8.32227C8.66919 8.02801 8.90784 7.79004 9.20215 7.79004H9.89258ZM11.668 1.75C12.3122 1.75018 12.834 2.27277 12.834 2.91699V6.94922C12.6003 6.95604 12.3606 6.97252 12.1553 7.01465C11.9686 7.05301 11.8098 7.12577 11.668 7.21387V2.91699H2.33398V8.16699L4.46094 6.04004C4.57484 5.92617 4.76012 5.92617 4.87402 6.04004L7.64062 8.80664C7.8661 9.05396 8.43506 9.63965 8.76953 9.63965H9.23047L9.32617 9.88477L8.68848 10.6768H8.3457C7.61657 10.6768 7.0164 11.2313 6.94434 11.9414L6.9375 12.085L6.94434 12.2285C6.94506 12.2357 6.94643 12.2429 6.94727 12.25H2.33398C1.6898 12.2498 1.16797 11.7272 1.16797 11.083V2.91699C1.16797 2.27277 1.6898 1.75018 2.33398 1.75H11.668ZM10.209 4.375C10.37 4.375 10.501 4.50591 10.501 4.66699V5.54199C10.5008 5.70292 10.3699 5.83301 10.209 5.83301H9.33398C9.17318 5.83283 9.04314 5.70282 9.04297 5.54199V4.66699C9.04297 4.50602 9.17308 4.37518 9.33398 4.375H10.209Z"
        fill="currentColor"
        fill-opacity="0.82"
      />
    </svg>
  );
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
        if (part.type === 'multi_part_variable') {
          return (
            <div key={ind} className="mb-2">
              <div className={styles['tool-title']}>
                <Tag color="primary" prefixIcon={<IconImageVariable />}>
                  {part.text}
                </Tag>
              </div>
            </div>
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
