/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */

import { useEffect, useRef, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  FooterActions,
  sleep,
  StepNav,
  useReportEvent,
} from '@cozeloop/components';
import { StonePromptApi } from '@cozeloop/api-schema';
import {
  Form,
  type FormApi,
  Spin,
  Modal,
  FormInput,
  Skeleton,
  FormTextArea,
} from '@coze-arch/coze-design';

import { versionValidate } from '@/utils/prompt';
import { usePromptStore } from '@/store/use-prompt-store';
import { CALL_SLEEP_TIME, EVENT_NAMES } from '@/consts';

import {
  VersionLabelTitle,
  FormVersionLabelSelect,
  type LabelWithPromptVersion,
  checkLabelDuplicate,
} from '../version-label';
import { PromptDiff } from '../prompt-diff';
import { usePromptDevProviderContext } from '../prompt-develop/components/prompt-provider';

interface PromptSubmitProps {
  visible: boolean;
  initVersion?: string;
  initDescription?: string;
  onCancel?: () => void;
  onOk?: (version: { version?: string }) => void;
}

export function PromptSubmit({
  visible,
  onOk,
  onCancel,
  initVersion,
  initDescription,
}: PromptSubmitProps) {
  const sendEvent = useReportEvent();
  const { spaceID, onSubmitSuccess, submitConfig } =
    usePromptDevProviderContext();
  const formApi = useRef<
    FormApi<{
      version?: string;
      description?: string;
      labels?: LabelWithPromptVersion[];
    }>
  >();

  const { promptInfo, totalReferenceCount } = usePromptStore(
    useShallow(state => ({
      promptInfo: state.promptInfo,
      totalReferenceCount: state.totalReferenceCount,
    })),
  );

  const [okButtonText, setOkButtonText] = useState(I18n.t('continue'));

  const getPromptService = useRequest(
    (version?: string) =>
      StonePromptApi.GetPrompt({
        prompt_id: promptInfo?.id ?? '',
        with_draft: !version,
        with_commit: Boolean(version),
        workspace_id: spaceID,
      }),
    {
      manual: false,
      ready: Boolean(spaceID && visible),
    },
  );

  const isFirstSubmit = !promptInfo?.prompt_basic?.latest_version;

  const submitService = useRequest(
    async () => {
      const values = await formApi.current
        ?.validate()
        ?.catch(e => console.error(e));
      if (!values) {
        return;
      }

      await checkLabelDuplicate(values.labels);

      try {
        await StonePromptApi.CommitDraft({
          prompt_id: promptInfo?.id || '',
          commit_version: values?.version || '',
          commit_description: values?.description,
          label_keys: values?.labels?.map(item => item.key),
        });

        sendEvent?.(EVENT_NAMES.prompt_submit_info, {
          prompt_id: `${promptInfo?.id || 'playground'}`,
          prompt_key: promptInfo?.prompt_key || 'playground',
          version: values?.version || '',
        });
        await sleep(CALL_SLEEP_TIME);

        await onOk?.({ version: values?.version });

        onSubmitSuccess?.({
          prompt: promptInfo,
          version: values?.version,
          totalReferenceCount,
        });
      } catch (e) {
        console.error(e);
      }
    },
    {
      manual: true,
      ready: Boolean(spaceID && promptInfo?.id),
      refreshDeps: [spaceID, promptInfo?.id, totalReferenceCount],
    },
  );

  const submitForm = (
    <Form
      className="w-full max-w-[600px]"
      initValues={{ version: initVersion, description: initDescription }}
      getFormApi={api => (formApi.current = api)}
    >
      <FormInput
        label={{
          text: I18n.t('version'),
          required: true,
        }}
        field="version"
        required
        validate={val => versionValidate(val, initVersion)}
        placeholder={I18n.t('input_version_number')}
      />

      {submitConfig?.hideVersionLabel ? null : (
        <FormVersionLabelSelect
          label={<VersionLabelTitle />}
          field="labels"
          promptID={promptInfo?.id || ''}
        />
      )}
      <FormTextArea
        label={I18n.t('version_description')}
        field="description"
        placeholder={I18n.t('please_input_version_description')}
        maxCount={200}
        maxLength={200}
        rows={5}
      />
    </Form>
  );

  const handleSubmit = () => {
    if (okButtonText === I18n.t('continue')) {
      setOkButtonText(I18n.t('submit'));
    } else {
      submitService.runAsync().catch(e => console.error(e));
    }
  };

  const handleCancel = () => {
    if (!isFirstSubmit && okButtonText === I18n.t('submit')) {
      setOkButtonText(I18n.t('continue'));
    } else {
      onCancel?.();
    }
  };

  useEffect(() => {
    if (!visible) {
      setOkButtonText(I18n.t('continue'));
      formApi.current?.reset();
    } else if (visible && isFirstSubmit) {
      setOkButtonText(I18n.t('submit'));
    }
  }, [visible, isFirstSubmit, promptInfo]);

  return (
    <Modal
      data-btm="c58401"
      className="min-h-[calc(100vh - 140px)]"
      width={!isFirstSubmit ? 900 : 640}
      visible={visible}
      title={I18n.t('submit_new_version')}
      onCancel={onCancel}
      okButtonProps={{ loading: submitService.loading }}
      height="fit-content"
      footer={
        <FooterActions
          confirmBtnProps={{
            text: okButtonText,
            loading: submitService.loading,
            onClick: handleSubmit,
            'data-btm': okButtonText === I18n.t('submit') ? 'd87263' : '',
            'data-btm-title':
              okButtonText === I18n.t('submit')
                ? I18n.t('prompt_prompt_submit')
                : '',
          }}
          cancelBtnProps={{
            onClick: handleCancel,
            text:
              isFirstSubmit || okButtonText === I18n.t('continue')
                ? I18n.t('cancel')
                : I18n.t('prev_step'),
          }}
        />
      }
    >
      <Skeleton
        loading={getPromptService.loading || !getPromptService.data?.prompt}
        placeholder={
          <div className="w-full flex items-center justify-center h-[470px]">
            <Spin />
          </div>
        }
      >
        <div className="w-full  overflow-y-auto">
          {!isFirstSubmit ? (
            <div className="flex flex-col gap-2">
              <StepNav
                currentStep={okButtonText}
                stepItems={[
                  {
                    key: I18n.t('continue'),
                    label: I18n.t('confirm_version_difference'),
                  },
                  {
                    key: I18n.t('submit'),
                    label: I18n.t('confirm_version_info'),
                  },
                ]}
              />

              <div className="flex-1">
                {okButtonText === I18n.t('continue') ? (
                  <PromptDiff
                    onlyShowContent
                    visible
                    data={promptInfo}
                    contentHeight="calc(100vh - 340px)"
                    currentVersionTitle={I18n.t(
                      'prompt_current_submission_draft',
                    )}
                    sameDesc={I18n.t('prompt_submit_no_version_diff_confirm')}
                  />
                ) : (
                  <div className="flex justify-center w-full">{submitForm}</div>
                )}
              </div>
            </div>
          ) : (
            submitForm
          )}
        </div>
      </Skeleton>
    </Modal>
  );
}
