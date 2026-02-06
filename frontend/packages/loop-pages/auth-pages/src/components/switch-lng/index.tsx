// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cls from 'classnames';
import { useI18nStore } from '@cozeloop/stores';
import { I18n } from '@cozeloop/i18n-adapter';
import { Tooltip } from '@coze-arch/coze-design';

import s from './index.module.less';

interface Props {
  className?: string;
}

export function SwitchLang({ className }: Props) {
  const toggleLangState = useI18nStore(state => state.toggleLng);
  const lang = useI18nStore(state => state.lng);
  const isEn = lang === 'en-US';
  const toggleLang = () => {
    toggleLangState();
    location.reload();
  };

  return (
    <Tooltip content={I18n.t('toggle_lng_tip')} clickTriggerToHide={true}>
      <div className={cls(s.container, className)} onClick={toggleLang}>
        <svg
          viewBox="0 0 1024 1024"
          xmlns="http://www.w3.org/2000/svg"
          width="18"
          height="18"
        >
          {/* A */}
          <path
            d="M832 744H638.4l-48 131.2h-57.6l166.4-440h75.2l166.4 440H880L832 744zm-17.6-51.2L740.8 488h-8L656 692.8h158.4z"
            fill={isEn ? 'var(--coz-fg)' : 'var(--coz-fg-dim)'}
          />
          {/* 文 */}
          <path
            d="M532.8 697.6s-8-3.2-46.4-27.2c-44.8-27.2-86.4-52.8-121.6-83.2-35.2 28.8-75.2 56-121.6 81.6-46.4 25.6-100.8 49.6-163.2 73.6l-28.8-52.8c60.8-20.8 113.6-43.2 158.4-65.6 44.8-22.4 83.2-46.4 115.2-73.6-35.2-35.2-64-75.2-88-120S192 336 177.6 280h-104v-49.6h264c-12.8-40-22.4-72-30.4-92.8L360 128c3.2 6.4 6.4 19.2 12.8 35.2 4.8 16 9.6 30.4 12.8 40 1.6 6.4 4.8 16 9.6 28.8h249.6v49.6H537.6c-14.4 57.6-32 108.8-54.4 153.6-22.4 43.2-49.6 83.2-83.2 116.8 32 25.6 72 49.6 115.2 72 25.6 12.8 41.6 19.2 41.6 19.2l-24 54.4zM232 281.6c12.8 49.6 30.4 94.4 52.8 132.8 20.8 38.4 46.4 73.6 76.8 104 28.8-30.4 54.4-64 72-102.4 19.2-38.4 35.2-83.2 46.4-132.8H232z"
            fill={isEn ? 'var(--coz-fg-dim)' : 'var(--coz-fg)'}
          />
        </svg>
        <div className={s.text}>{isEn ? 'English' : '中文'}</div>
      </div>
    </Tooltip>
  );
}
