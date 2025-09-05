import { useState } from 'react';

import dayjs from 'dayjs';
import { type Result } from 'ahooks/lib/useRequest/src/types';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, Guard } from '@cozeloop/guard';
import { CozeUser } from '@cozeloop/evaluate-components';
import { RouteBackAction, EditIconButton } from '@cozeloop/components';
import {
  type EvaluatorVersion,
  type Evaluator,
} from '@cozeloop/api-schema/evaluation';
import { IconCozLoading } from '@coze-arch/coze-design/icons';
import { Button, Tag, Typography } from '@coze-arch/coze-design';

import {
  DebugButton,
  type DebugButtonProps,
} from '../evaluator-create/debug-button';
import { type BaseInfo, BaseInfoModal } from './base-info-modal';

export function Header({
  evaluator,
  selectedVersion,
  autoSaveService,
  onChangeBaseInfo,
  onOpenVersionList,
  onSubmitVersion,
  debugButtonProps,
}: {
  evaluator?: Evaluator;
  selectedVersion?: EvaluatorVersion;
  autoSaveService: Result<
    | {
        lastSaveTime: string | undefined;
      }
    | undefined,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    any
  >;
  onChangeBaseInfo: (values: BaseInfo) => void;
  onOpenVersionList: () => void;
  onSubmitVersion: () => void;

  debugButtonProps: DebugButtonProps;
}) {
  const [editVisible, setEditVisible] = useState(false);

  const renderAutoSave = () => {
    let tagContent: React.ReactNode = null;
    if (autoSaveService.loading) {
      tagContent = (
        <>
          <IconCozLoading className="w-3 h-3 animate-spin mr-1" />
          {I18n.t('draft_auto_saving')}
        </>
      );
    } else if (autoSaveService.error) {
      tagContent = I18n.t('draft_auto_save_failed');
    } else if (autoSaveService.data?.lastSaveTime) {
      tagContent = `${I18n.t('draft_auto_saved_in')} ${dayjs(Number(autoSaveService.data.lastSaveTime)).format('YYYY-MM-DD HH:mm:ss')}`;
    }

    if (tagContent) {
      return (
        <Tag
          color="primary"
          className="!h-5 !px-2 !py-[2px] rounded-[3px] mr-1"
        >
          {tagContent}
        </Tag>
      );
    }
    return null;
  };

  const renderExtra = () => {
    if (selectedVersion) {
      return (
        <>
          <Tag
            color="green"
            className="!h-5 !px-2 !py-[2px] rounded-[3px] mr-1"
          >
            {selectedVersion.version}
          </Tag>
          <div className="mx-3 h-3 w-0 border-0 border-l border-solid coz-stroke-primary" />
          <div className="text-xs coz-fg-secondary font-normal">
            {I18n.t('submission_time')}
            {dayjs(Number(selectedVersion.base_info?.created_at)).format(
              'YYYY-MM-DD HH:mm:ss',
            )}
          </div>
          <div className="mx-3 h-3 w-0 border-0 border-l border-solid coz-stroke-primary" />
          <div className="text-xs coz-fg-secondary font-normal flex items-center">
            <span className="shrink-0">{I18n.t('submitter')}</span>
            <CozeUser
              user={selectedVersion.base_info?.created_by}
              size="small"
            />
          </div>
        </>
      );
    }

    return (
      <>
        {evaluator?.draft_submitted === false ? (
          <Tag
            color="yellow"
            className="!h-5 !px-2 !py-[2px] rounded-[3px] mr-1"
          >
            {I18n.t('changes_not_submitted')}
          </Tag>
        ) : null}

        {renderAutoSave()}
      </>
    );
  };

  return (
    <>
      <div className="px-6 py-2 h-[64px] flex-shrink-0 flex flex-row items-center border-0 border-b border-solid coz-stroke-primary">
        <RouteBackAction defaultModuleRoute="evaluation/evaluators" />
        <div className="ml-2 flex-1">
          <div className="text-[14px] leading-5 font-medium coz-fg-plus flex items-center gap-x-1">
            <Typography.Text className="!coz-fg-plus !font-medium !text-[14px] !leading-[20px]">
              {evaluator?.name}
            </Typography.Text>
            <Guard point={GuardPoint['eval.evaluator.edit_meta']}>
              <EditIconButton onClick={() => setEditVisible(true)} />
            </Guard>
          </div>
          <div className="h-6 flex flex-row items-center">
            <div className="text-xs font-normal !coz-fg-secondary max-w-[240px] overflow-hidden text-ellipsis whitespace-nowrap leading-4">
              {I18n.t('description')}：{evaluator?.description || '-'}
            </div>
            <div className="mx-3 h-3 w-0 border-0 border-l border-solid coz-stroke-primary" />
            {renderExtra()}
          </div>
        </div>

        <div className="flex-shrink-0 flex flex-row gap-2">
          <Button color="primary" onClick={onOpenVersionList}>
            {I18n.t('version_record')}
          </Button>
          {selectedVersion ? null : <DebugButton {...debugButtonProps} />}
          {selectedVersion ? null : (
            <Guard point={GuardPoint['eval.evaluator.commit']}>
              <Button color="brand" onClick={onSubmitVersion}>
                {I18n.t('submit_new_version')}
              </Button>
            </Guard>
          )}
        </div>
      </div>
      <BaseInfoModal
        evaluator={evaluator}
        visible={editVisible}
        onCancel={() => setEditVisible(false)}
        onSubmit={onChangeBaseInfo}
      />
    </>
  );
}
