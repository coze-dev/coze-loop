import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozIllusAdd } from '@coze-arch/coze-design/illustrations';
import { EmptyState } from '@coze-arch/coze-design';

export function ExperimentListEmptyState({
  hasFilterCondition,
}: {
  hasFilterCondition: boolean;
}) {
  return (
    <EmptyState
      size="full_screen"
      icon={<IconCozIllusAdd />}
      title={
        hasFilterCondition
          ? I18n.t('no_results_found')
          : I18n.t('no_experiment_yet')
      }
      description={
        hasFilterCondition
          ? I18n.t('try_other_keywords')
          : I18n.t('click_to_create_experiment')
      }
    />
  );
}
