// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { downloadImageWithCustomName } from '@cozeloop/toolkit';
import { type ContentPartLoop } from '@cozeloop/components';
import { ContentType, Role } from '@cozeloop/api-schema/prompt';
import { IconCozDownload } from '@coze-arch/coze-design/icons';
import {
  ImagePreview,
  Typography,
  Image,
  IconButton,
} from '@coze-arch/coze-design';
import { CalypsoLazy } from '@bytedance/calypso';

import { messageId } from '@/utils/prompt';

import { usePromptDevProviderContext } from '../../prompt-provider';

import styles from './index.module.less';

export function MultiModalItem({
  parts,
  isMarkdown,
  streaming,
  smooth,
}: {
  parts: ContentPartLoop[];
  isMarkdown?: boolean;
  streaming?: boolean;
  smooth?: boolean;
}) {
  const { debugAreaConfig } = usePromptDevProviderContext();

  return (
    <div className={styles['multi-modal-item']}>
      {parts?.map((item, index) => {
        if (item.type === ContentType.Text) {
          return !isMarkdown ? (
            <Typography.Paragraph className="whitespace-break-spaces">
              {item.text}
            </Typography.Paragraph>
          ) : (
            debugAreaConfig?.renderMessageItem?.({
              message: {
                role: Role.Assistant,
                content: item.text,
                id: messageId(),
              },
              streaming,
            }) || (
              <CalypsoLazy
                markDown={item.text ?? (streaming && index !== 0 ? '...' : '')}
                imageOptions={{ forceHttps: true }}
                smooth={smooth}
                autoFixSyntax={{ autoFixEnding: smooth }}
              />
            )
          );
        } else if (item.type === ContentType.ImageURL) {
          return (
            <ImagePreview
              key={item.uid}
              className={styles['multi-modal-item-image']}
            >
              <div className={styles['image-preview-list']}>
                <Image
                  src={item?.image_url?.url}
                  className="w-full h-full"
                  imgStyle={{
                    objectFit: 'contain',
                    width: '100%',
                    height: '100%',
                  }}
                />
                <IconButton
                  className="fornax-small-button absolute right-1 bottom-2 opacity-0"
                  size="small"
                  color="primary"
                  icon={<IconCozDownload />}
                  onClick={() =>
                    item?.image_url?.url &&
                    downloadImageWithCustomName(item?.image_url?.url)
                  }
                />
              </div>
            </ImagePreview>
          );
        }
        return null;
      })}
    </div>
  );
}
