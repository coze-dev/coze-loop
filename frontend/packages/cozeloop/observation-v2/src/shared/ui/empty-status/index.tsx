import {
  IconCozIllusEmpty,
  IconCozIllusError,
} from '@coze-arch/coze-design/illustrations';
import { IconCozEmpty } from '@coze-arch/coze-design/icons';
import { Empty, Typography } from '@coze-arch/coze-design';

import { i18nService, useLocale } from '@/i18n';

export const NodeDetailEmpty = () => (
  <Empty
    className="w-full h-full flex items-center justify-center"
    image={<IconCozIllusEmpty style={{ width: 150, height: 150 }} />}
    title={i18nService.t('observation_empty_node_unselected')}
    description={i18nService.t('observation_empty_to_select_node')}
  />
);

export const RunTreeEmpty = () => (
  <Empty
    className="w-full h-full flex items-center justify-center"
    image={<IconCozIllusError style={{ width: 150, height: 150 }} />}
    title={i18nService.t('observation_empty_run_tree_failure')}
    description={i18nService.t('observation_empty_data_abnormal')}
  />
);

export const SearchEmptyComponent = ({ onClear }: { onClear?: () => void }) => {
  const { t } = useLocale();

  return (
    <div className="px-6 py-[24px] flex-col gap-2 flex items-center justify-center">
      <IconCozEmpty className="w-[20px] h-[20px] coz-fg-secondary" />
      <Typography.Text className="text-[14px] !font-normal">
        {t('reported_data_not_found')}
      </Typography.Text>
      <div className="flex flex-col items-center">
        <Typography.Text className="text-[12px] !coz-fg-secondary text-center">
          {t('search_support_quick_search')}
          {t('filter_more_fields_through_filter')}
        </Typography.Text>
      </div>
      <Typography.Text className="text-[12px] " link={true} onClick={onClear}>
        {t('clear_filter')}
      </Typography.Text>
    </div>
  );
};
