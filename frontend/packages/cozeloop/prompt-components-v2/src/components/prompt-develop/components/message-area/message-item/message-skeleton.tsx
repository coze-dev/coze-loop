/* eslint-disable security/detect-object-injection */

import classNames from 'classnames';
import { Role } from '@cozeloop/api-schema/prompt';
import { Skeleton } from '@coze-arch/coze-design';

import styles from './index.module.less';

interface MessageSkeletonProps {
  messageType?: Role;
  estimatedHeight?: number;
}

export function MessageSkeleton({
  messageType = Role.Assistant,
  estimatedHeight = 120,
}: MessageSkeletonProps) {
  return (
    <div
      className={styles['message-item']}
      style={{ minHeight: estimatedHeight }}
    >
      {/* 头像骨架屏 */}
      <Skeleton.Avatar
        className={styles['message-avatar']}
        size="default"
        style={{
          width: 32,
          height: 32,
        }}
      />

      {/* 消息内容骨架屏 */}
      <div
        className={classNames(styles['message-content'], styles[messageType])}
      >
        <div className={styles['message-info']}>
          <Skeleton.Paragraph rows={1} />
        </div>

        {/* 底部工具栏骨架屏 */}
        <div className={styles['message-footer-tools']}>
          <div className="flex justify-between items-center w-full">
            <div className="flex space-x-2"></div>
            <div className="flex space-x-2">
              <Skeleton.Button style={{ width: 40, height: 24 }} />
              <Skeleton.Button style={{ width: 40, height: 24 }} />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
