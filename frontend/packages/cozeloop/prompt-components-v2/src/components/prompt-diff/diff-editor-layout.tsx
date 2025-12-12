import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

const DEFAULT_EDITOR_HEIGHT = 400;

interface DiffEditorLayoutProps {
  preVersion?: React.ReactNode;
  currentVersion?: React.ReactNode;
  currentHeaderExtraActions?: React.ReactNode;
  diffEditorHeight?: string | number;
  children?: React.ReactNode;
  className?: string;
}

export function DiffEditorLayout({
  preVersion,
  currentVersion,
  diffEditorHeight,
  children,
  className,
  currentHeaderExtraActions,
}: DiffEditorLayoutProps) {
  return (
    <div
      className={classNames(styles['diff-editor-container'], className)}
      style={{ height: diffEditorHeight ?? DEFAULT_EDITOR_HEIGHT }}
    >
      <div className={classNames(styles['diff-header-wrap'], '!bg-[#F7F7FC]')}>
        <div className={styles['diff-header']}>
          <Typography.Text strong>{preVersion || ''}</Typography.Text>
        </div>
        <div className="w-[1px] h-full bg-[#fcfcff] border-0 border-solid !border-r coz-stroke-primary absolute top-0 bottom-0 left-[50%]"></div>
        <div className={styles['diff-header']}>
          <Typography.Text strong>
            {currentVersion || I18n.t('draft_version')}
          </Typography.Text>
          {currentHeaderExtraActions}
        </div>
      </div>
      <div className="flex-1 flex w-full h-full overflow-y-auto styled-scrollbar relative">
        {children}
      </div>
    </div>
  );
}
