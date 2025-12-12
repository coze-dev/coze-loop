import { useLocalStorageState } from 'ahooks';
import {
  IconCozInfoCircleFill,
  IconCozCross,
} from '@coze-arch/coze-design/icons';
import { Typography, IconButton } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';
import { useTraceStore } from '@/features/trace-list/stores/trace';

export const CozeLoopTraceBanner = () => {
  const { customParams } = useTraceStore();
  const { t } = useLocale();
  const [visible, setVisible] = useLocalStorageState(
    `${customParams?.user?.user_id_str ?? ''}_${customParams?.spaceID ?? ''}_coze_up_banner_trace`,
    {
      defaultValue: true,
    },
  );

  if (!visible) {
    return null;
  }
  return (
    <div className="h-[36px] w-full bg-brand-3 text-left px-4 py-2 box-border justify-between flex items-center">
      <div className="flex items-center gap-x-1">
        <IconCozInfoCircleFill className="w-[14px] h-[14px] text-brand-9" />
        <span className="text-[var(--coz-fg-primary)] text-[13px] inline-flex items-center">
          {t('using_cozeloop_sdk_tip', {
            sdk: (
              <Typography.Text
                link={{
                  href: 'https://loop.coze.cn/open/docs/cozeloop/sdk',
                  target: '_blank',
                }}
                className="text-brand-9"
              >
                <span className="text-brand-9">
                  &nbsp;{t('cozeloop_sdk')}&nbsp;
                </span>
              </Typography.Text>
            ),
          })}
        </span>
      </div>
      <IconButton
        className="!w-[20px] !h-[20px]"
        icon={<IconCozCross />}
        onClick={() => setVisible(false)}
        size="mini"
        color="secondary"
      />
    </div>
  );
};
