// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useParams } from 'react-router-dom';
import { useRef, useState } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import { ModelConfigInfo, TemplateInfo } from '@cozeloop/evaluate-components';
import { useDemoSpace, useSpace } from '@cozeloop/biz-hooks-adapter';
import { useBreadcrumb } from '@cozeloop/base-hooks';
import {
  EvaluatorType,
  type EvaluatorVersion,
  type Evaluator,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Form, Spin, Toast } from '@coze-arch/coze-design';

import { SubmitVersionModal } from '../evaluator-create/submit-version-modal';
import { generateInputSchemas } from '../evaluator-create/prompt-field';
import { PromptConfigField } from '../evaluator-create/prompt-config-field';
import { VersionListPane } from './version-list-pane';
import { Header } from './header';

function EvaluatorDetailPage() {
  const { spaceID } = useSpace();
  const { id } = useParams<{ id: string }>();
  const formRef = useRef<Form<Evaluator>>(null);

  const [versionListVisible, setVersionListVisible] = useState(false);
  const [versionListRefreshFlag, setVersionListRefreshFlag] = useState([]);
  const [submitModalVisible, setSubmitModalVisible] = useState(false);
  const { isDemoSpace } = useDemoSpace();

  const [selectedVersion, setSelectedVersion] = useState<
    EvaluatorVersion | undefined
  >();

  const [refreshEditorModelKey, setRefreshEditorModelKey] = useState(0);

  const service = useRequest(async () => {
    const queryString = window.location.search;
    const urlParams = new URLSearchParams(queryString);
    const versionID = urlParams.get('version');
    if (versionID) {
      setSelectedVersion({
        id: versionID,
      });
    }

    return StoneEvaluationApi.GetEvaluator({
      workspace_id: spaceID,
      evaluator_id: id,
    }).then(res => res.evaluator);
  });
  const evaluator = service.data;

  const guard = useGuard({ point: GuardPoint['eval.evaluator.global'] });
  useBreadcrumb({
    text: evaluator?.name || '',
  });

  const autoSaveService = useRequest(
    async (values: Evaluator) => {
      const evaluatorContent = values.current_version?.evaluator_content || {};
      evaluatorContent.input_schemas = generateInputSchemas(
        evaluatorContent?.prompt_evaluator?.message_list,
      );

      const res = await StoneEvaluationApi.UpdateEvaluatorDraft({
        workspace_id: spaceID,
        evaluator_id: evaluator?.evaluator_id || '',
        evaluator_content: evaluatorContent,
        evaluator_type: values.evaluator_type || EvaluatorType.Prompt,
      });
      if (res.evaluator) {
        service.mutate(res.evaluator);
        return { lastSaveTime: res.evaluator?.base_info?.updated_at };
      }
    },
    {
      manual: true,
      debounceWait: 800,
    },
  );

  const versionService = useRequest(
    async () => {
      if (selectedVersion?.id) {
        const res = await StoneEvaluationApi.GetEvaluatorVersion({
          workspace_id: spaceID,
          evaluator_version_id: selectedVersion.id,
        });
        const versionDetail = res.evaluator?.current_version;
        if (versionDetail) {
          setSelectedVersion(pre => {
            if (pre?.id === versionDetail.id) {
              return versionDetail;
            }
            return pre;
          });
        }
      }
    },
    {
      refreshDeps: [selectedVersion?.id],
    },
  );

  if (service.loading) {
    return (
      <div className="h-full flex items-center justify-center">
        <Spin spinning={true} />
      </div>
    );
  }

  const renderContent = () => {
    if (selectedVersion) {
      if (versionService.loading) {
        return (
          <div className="h-full w-full flex items-center justify-center">
            <Spin spinning={true} />
          </div>
        );
      }
      return (
        <div className="flex-1 max-w-[800px] mx-auto">
          <div className="h-[28px] mb-3 text-[16px] leading-7 font-medium coz-fg-plus">
            {I18n.t('config_info')}
          </div>
          <ModelConfigInfo
            data={
              selectedVersion.evaluator_content?.prompt_evaluator?.model_config
            }
          />
          <div className="h-2" />
          <TemplateInfo
            notTemplate={true}
            data={selectedVersion.evaluator_content}
          />
          <div className="h-6" />
        </div>
      );
    }

    return (
      <Form
        initValues={evaluator}
        className="flex-1 max-w-[800px] mx-auto"
        ref={formRef}
        onValueChange={values => {
          // Demo 空间且没有管理权限，不保存
          if (!isDemoSpace) {
            autoSaveService.run(values);
          }
        }}
      >
        <PromptConfigField
          disabled={guard.data.readonly}
          refreshEditorModelKey={refreshEditorModelKey}
        />
        <div className="h-6" />
      </Form>
    );
  };
  return (
    <div className="h-full overflow-hidden flex flex-col">
      <Header
        evaluator={evaluator}
        autoSaveService={autoSaveService}
        selectedVersion={selectedVersion}
        onChangeBaseInfo={baseInfo =>
          service.mutate(old => ({
            ...old,
            ...baseInfo,
          }))
        }
        onOpenVersionList={() => setVersionListVisible(true)}
        onSubmitVersion={() =>
          formRef?.current?.formApi
            ?.validate()
            .then(() => {
              setSubmitModalVisible(true);
            })
            .catch(e => console.warn(e))
        }
        debugButtonProps={{
          formApi: formRef,
          onApplyValue: () => {
            setRefreshEditorModelKey(pre => pre + 1);
          },
        }}
      />

      <div className="flex-1 overflow-hidden flex flex-row">
        <div className="flex-1 overflow-y-auto p-6 flex styled-scrollbar pr-[18px]">
          {renderContent()}
        </div>
        {versionListVisible && evaluator ? (
          <VersionListPane
            evaluator={evaluator}
            onClose={() => setVersionListVisible(false)}
            selectedVersion={selectedVersion}
            onSelectVersion={setSelectedVersion}
            refreshFlag={versionListRefreshFlag}
          />
        ) : null}
      </div>

      <SubmitVersionModal
        type="append"
        visible={submitModalVisible}
        evaluator={evaluator}
        onCancel={() => setSubmitModalVisible(false)}
        onSuccess={(_, newEvaluator) => {
          setSubmitModalVisible(false);
          Toast.success(I18n.t('version_submit_success'));
          service.mutate(() => newEvaluator);
          if (versionListVisible) {
            setVersionListRefreshFlag([]);
          }
        }}
      />
    </div>
  );
}

export default EvaluatorDetailPage;
