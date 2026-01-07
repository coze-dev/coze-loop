// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable complexity */
/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */
import { useParams } from 'react-router-dom';
import { useRef, useState, useCallback } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { Guard, GuardPoint, useGuard } from '@cozeloop/guard';
import { useDemoSpace, useSpace } from '@cozeloop/biz-hooks-adapter';
import { useBreadcrumb } from '@cozeloop/base-hooks';
import {
  EvaluatorType,
  type EvaluatorVersion,
  type Evaluator,
  type EvaluatorInputData,
  type LanguageType,
  type EvaluatorContent,
  type EvaluatorOutputData,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { Form, Spin, Toast } from '@coze-arch/coze-design';

import { SCROLL_DELAY, SCROLL_OFFSET } from '@/utils/evaluator';
import {
  CodeEvaluatorLanguageFE,
  codeEvaluatorLanguageMapReverse,
  codeEvaluatorLanguageMap,
} from '@/constants';
import {
  TestDataSource,
  type CodeEvaluatorValue,
} from '@/components/evaluator-code/types';

import { VersionListPane } from '../version-list-pane';
import { Header } from '../header';
import { SubmitVersionModal } from '../../evaluator-create/submit-version-modal';
import { FullScreenEditorConfigModal } from '../../evaluator-create/code-create/full-screen-editor-config-modal';
import { CodeTemplateModal } from '../../evaluator-create/code-create/code-template-modal';
import { CodeEvaluatorVersionView } from './code-evaluator-version-view';
import {
  CodeEvaluatorConfigField,
  transformApiToComponent,
} from './code-evaluator-config-field';
import { CodeDebugButton } from './code-debug-button';

import styles from './index.module.less';

interface IFormValue {
  name?: string;
  description?: string;
  config: CodeEvaluatorValue;
}

function CodeEvaluatorDetailPage() {
  const { spaceID } = useSpace();
  const { id } = useParams<{ id: string }>();
  const formRef = useRef<Form<IFormValue>>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const [versionListVisible, setVersionListVisible] = useState(false);
  const [versionListRefreshFlag, setVersionListRefreshFlag] = useState([]);
  const [submitModalVisible, setSubmitModalVisible] = useState(false);
  const { isDemoSpace } = useDemoSpace();
  const initialFlag = useRef<boolean>(true);

  const [selectedVersion, setSelectedVersion] = useState<
    EvaluatorVersion | undefined
  >();

  const [_initialEvaluator, setInitialEvaluator] = useState<
    Evaluator | undefined
  >(undefined);

  const [refreshEditorModelKey, _setRefreshEditorModelKey] = useState(0);
  const [templateModalVisible, setTemplateModalVisible] = useState(false);
  const [templateInfo, setTemplateInfo] = useState<{
    key: string;
    name: string;
    lang: string;
  } | null>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);

  const service = useRequest(
    async () => {
      if (!id) {
        throw new Error('Evaluator ID is required');
      }

      const queryString = window.location.search;
      const urlParams = new URLSearchParams(queryString);
      const versionID = urlParams.get('version');
      if (versionID) {
        setSelectedVersion({
          id: versionID,
        });
      }

      const res = await StoneEvaluationApi.GetEvaluator({
        workspace_id: spaceID,
        evaluator_id: id,
      });

      if (!res.evaluator) {
        throw new Error('Evaluator not found');
      }

      setInitialEvaluator(res.evaluator);

      // 将评估器数据设置到表单
      if (res.evaluator) {
        const formValue: IFormValue = transformApiToComponent(res.evaluator);

        // 使用 setTimeout 确保表单已经渲染完成
        setTimeout(() => {
          formRef.current?.formApi?.setValues(formValue);
        }, 0);
      }

      return res.evaluator;
    },
    {
      refreshDeps: [id, spaceID],
    },
  );
  const evaluator = service.data;

  const guard = useGuard({ point: GuardPoint['eval.evaluator.global'] });
  useBreadcrumb({
    text: evaluator?.name || '',
  });

  const autoSaveService = useRequest(
    async (values: IFormValue) => {
      // 初始化时不自动保存
      if (initialFlag.current) {
        initialFlag.current = false;
        return;
      }
      const { config } = values;
      const { funcExecutor } = config || {};
      const { code, language = CodeEvaluatorLanguageFE.Javascript } =
        funcExecutor || {};

      const res = await StoneEvaluationApi.UpdateEvaluatorDraft({
        workspace_id: spaceID,
        evaluator_id: evaluator?.evaluator_id || '',
        evaluator_content: {
          code_evaluator: {
            code_content: code,
            language_type: codeEvaluatorLanguageMapReverse[language],
          },
        },
        evaluator_type: EvaluatorType.Code,
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

  // 通用的调试执行函数
  const executeDebug = async (
    targetFormRef: React.RefObject<Form<IFormValue>>,
  ) => {
    try {
      const formValues = targetFormRef.current?.formApi.getValues();
      if (!formValues) {
        return;
      }
      const { config } = formValues;
      const { funcExecutor, testData } = config || {};
      const { code, language = CodeEvaluatorLanguageFE.Javascript } =
        funcExecutor || {};

      // 验证配置
      if (!code?.trim()) {
        Toast.info({
          content: I18n.t('evaluate_please_write_function_body'),
          top: 80,
        });
        return;
      }

      const { source, customData, setData } = testData || {};
      if (
        (source === TestDataSource.Custom && !customData) ||
        (source === TestDataSource.Dataset && !setData)
      ) {
        Toast.info({
          content: I18n.t('evaluate_please_configure_test_data'),
          top: 80,
        });
        return;
      }

      // 平滑滚动到第410行容器下方200px位置
      if (scrollContainerRef?.current && !isFullscreen) {
        setTimeout(() => {
          scrollContainerRef.current?.scrollTo({
            top: scrollContainerRef.current?.scrollHeight + SCROLL_OFFSET,
            behavior: 'smooth',
          });
        }, SCROLL_DELAY);
      }

      // 发起 debug 请求
      const res = await StoneEvaluationApi.BatchDebugEvaluator({
        workspace_id: spaceID,
        evaluator_type: EvaluatorType.Code,
        evaluator_content: {
          code_evaluator: {
            code_content: code,
            language_type: codeEvaluatorLanguageMapReverse[language],
          },
        },
        input_data:
          source === TestDataSource.Custom
            ? [customData as EvaluatorInputData]
            : (setData as unknown as EvaluatorInputData[]),
      });

      // 处理调试结果
      if (
        !res.evaluator_output_data ||
        res.evaluator_output_data.length === 0
      ) {
        Toast.error({
          content: I18n.t('evaluate_debug_failed_no_evaluation_result'),
          top: 80,
        });
        return;
      }

      // 收集所有结果
      const allResults = res.evaluator_output_data || [];

      if (allResults.length > 0) {
        // 直接通过表单API更新runResults
        targetFormRef.current?.formApi.setValue(
          'config.runResults',
          allResults,
        );

        return allResults;
      } else {
        Toast.warning({
          content: I18n.t('evaluate_debug_no_evaluation_result'),
          top: 80,
        });
        return;
      }
    } catch (error) {
      console.error(I18n.t('evaluate_debug_failed'), error);
      Toast.error({
        content: `调试失败: ${(error as Error)?.message || I18n.t('evaluate_unknown_error')}`,
        top: 80,
      });
    }
  };

  const debugService = useRequest(
    async (ref: React.RefObject<Form<IFormValue>>) =>
      (await executeDebug(ref)) as EvaluatorOutputData[] | undefined,
    {
      manual: true,
    },
  );

  // 处理模板选择
  const handleTemplateSelect = useCallback((template?: EvaluatorContent) => {
    const { code_evaluator } = template || {};
    if (code_evaluator) {
      const { code_template_key, code_template_name, language_type } =
        code_evaluator;

      // 设置模板信息
      setTemplateInfo({
        key: code_template_key || '',
        name: code_template_name || '',
        lang: language_type || '',
      });

      // 更新表单值
      if (code_evaluator.code_content && formRef.current) {
        formRef.current.formApi.setValue('config.funcExecutor', {
          code: code_evaluator.code_content,
          language:
            codeEvaluatorLanguageMap[language_type as LanguageType] ||
            CodeEvaluatorLanguageFE.Javascript,
        });
      }

      // 关闭模板选择弹窗
      setTemplateModalVisible(false);
    }
  }, []);

  // 处理全屏切换
  const handleFullscreenToggle = useCallback(() => {
    setIsFullscreen(prev => !prev);
  }, []);

  if (service.loading) {
    return (
      <div className="h-full flex items-center justify-center">
        <Spin spinning={true} />
      </div>
    );
  }

  if (service.error) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-center">
          <div className="text-red-500 mb-2">加载失败</div>
          <div className="text-gray-500 text-sm">{service.error.message}</div>
        </div>
      </div>
    );
  }

  if (!evaluator) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-center">
          <div className="text-gray-500">评估器不存在</div>
        </div>
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
          <CodeEvaluatorVersionView version={selectedVersion} />
        </div>
      );
    }
  };

  return (
    <div
      className={`h-full overflow-hidden flex flex-col ${styles['code-detail-page-container']}`}
    >
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
        customDebugButton={
          <Guard point={GuardPoint['eval.evaluator_create.debug']} realtime>
            <CodeDebugButton
              onClick={() => {
                debugService.run(formRef);
              }}
              loading={debugService.loading}
            />
          </Guard>
        }
      />

      <div className="flex-1 overflow-hidden flex flex-row">
        <div
          ref={scrollContainerRef}
          className="flex-1 overflow-y-auto p-6 flex styled-scrollbar pr-[18px]"
        >
          <Form
            className="flex-1 max-w-[1000px] mx-auto"
            ref={formRef}
            onValueChange={values => {
              // Demo 空间且没有管理权限，不保存
              if (!isDemoSpace) {
                autoSaveService.run(values);
              }
            }}
          >
            {renderContent()}
            <div className={`${selectedVersion ? 'hidden' : ''}`}>
              <CodeEvaluatorConfigField
                disabled={guard.data.readonly}
                refreshEditorModelKey={refreshEditorModelKey}
                debugLoading={debugService.loading}
                onOpenTemplateModal={() => setTemplateModalVisible(true)}
                templateInfo={templateInfo}
                onFullscreenToggle={handleFullscreenToggle}
                editorHeight="600px"
              />
            </div>
          </Form>
        </div>
        <div className="h-6" />

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

      <CodeTemplateModal
        visible={templateModalVisible}
        isShowCustom={false}
        onCancel={() => setTemplateModalVisible(false)}
        onSelect={handleTemplateSelect}
      />

      <FullScreenEditorConfigModal
        formRef={formRef}
        visible={isFullscreen}
        debugLoading={debugService.loading}
        setVisible={setIsFullscreen}
        onCancel={handleFullscreenToggle}
        onRun={
          debugService.run as (
            ref: React.RefObject<Form<IFormValue>>,
          ) => Promise<EvaluatorOutputData[] | undefined>
        }
        onChange={vs => {
          formRef.current?.formApi.setValues(vs);
        }}
      />
    </div>
  );
}

export default CodeEvaluatorDetailPage;
