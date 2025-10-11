/* eslint-disable max-lines */
/* eslint-disable complexity */
/* eslint-disable max-lines-per-function */
/* eslint-disable @coze-arch/max-line-per-function */
import { useLocation, useParams } from 'react-router-dom';
import { useState, useRef, useCallback, useEffect } from 'react';

import { nanoid } from 'nanoid';
import { Guard, GuardPoint } from '@cozeloop/guard';
import { sourceNameRuleValidator } from '@cozeloop/evaluate-components';
import { useNavigateModule, useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  EvaluatorType,
  TemplateType,
  LanguageType,
  type evaluator,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  IconCozTemplate,
  IconCozPlayFill,
  IconCozExpand,
} from '@coze-arch/coze-design/icons';
import {
  Form,
  FormInput,
  Button,
  Toast,
  Divider,
  Banner,
  FormTextArea,
} from '@coze-arch/coze-design';

import { SCROLL_DELAY, SCROLL_OFFSET } from '@/utils/evaluator';
import {
  CodeEvaluatorLanguageFE,
  codeEvaluatorLanguageMap,
  defaultTestData,
} from '@/constants';
import {
  type BaseFuncExecutorValue,
  type IFormValues,
  TestDataSource,
} from '@/components/evaluator-code/types';
import CodeEvaluatorConfig from '@/components/evaluator-code';

import SubmitCheckModal from './submit-check-modal';
import { CodeCreateHeader } from './header';
import { FullScreenEditorConfigModal } from './full-screen-editor-config-modal';
import { CodeTemplateModal } from './code-template-modal';

import styles from './index.module.less';
import { I18n } from '@cozeloop/i18n-adapter';

const CODE_EVALUATOR_DOCUMENT_LINK =
  'https://loop.coze.cn/open/docs/cozeloop/create_evaluators';

const CodeEvaluatorCreatePage = () => {
  const { spaceID } = useSpace();
  const { id } = useParams<{ id: string }>();
  const navigateModule = useNavigateModule();
  const location = useLocation();
  // 使用特定类型作为formRef类型
  const formRef = useRef<Form<IFormValues>>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isRunning, setIsRunning] = useState(false);
  const [templateModalVisible, setTemplateModalVisible] = useState(false);
  const [submitCheckModalVisible, setSubmitCheckModalVisible] = useState(false);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [templateInfo, setTemplateInfo] = useState<{
    key: string;
    name: string;
    lang: string;
  } | null>(null);

  const handleSubmit = useCallback(async () => {
    try {
      // 验证表单
      const validation = await formRef.current?.formApi.validate();
      if (!validation) {
        return;
      }

      // 获取表单数据
      const formValues = formRef.current?.formApi.getValues();
      if (!formValues) {
        return;
      }

      // 验证配置
      if (!formValues.config?.funcExecutor?.code?.trim()) {
        Toast.error({
          content: I18n.t('evaluate_please_write_function_body'),
          top: 80,
        });
        return;
      }

      setIsSubmitting(true);

      // 这里应该调用实际的API保存数据
      const submitResult = await StoneEvaluationApi.CreateEvaluator({
        cid: nanoid(),
        evaluator: {
          evaluator_type: EvaluatorType.Code,
          name: formValues.name,
          description: formValues.description,
          workspace_id: spaceID,
          current_version: {
            version: '0.0.1',
            evaluator_content: {
              code_evaluator: {
                code_content: formValues.config.funcExecutor.code,
                language_type:
                  formValues.config.funcExecutor.language ===
                  CodeEvaluatorLanguageFE.Javascript
                    ? LanguageType.JS
                    : LanguageType.Python,
              },
            },
          },
        },
      });

      // 模拟保存
      const SAVE_DELAY = 1000;
      await new Promise(resolve => setTimeout(resolve, SAVE_DELAY));

      Toast.success({
        content: I18n.t('evaluate_code_evaluator_created_successfully'),
        top: 80,
      });

      // 跳转到详情页面
      navigateModule(
        `evaluation/evaluators/code/${submitResult.evaluator_id}`,
        { replace: true },
      );
    } catch (error) {
      console.error(I18n.t('evaluate_save_failed'), error);
      Toast.error({
        content: I18n.t('evaluate_save_failed_please_retry'),
        top: 80,
      });
    } finally {
      setIsSubmitting(false);
    }
  }, [spaceID, navigateModule]);

  // 处理提交检查弹窗
  const handleSubmitCheck = useCallback(() => {
    setSubmitCheckModalVisible(true);
  }, []);

  // 处理提交检查弹窗取消
  const handleSubmitCheckCancel = useCallback(() => {
    setSubmitCheckModalVisible(false);
  }, []);

  // 处理提交检查弹窗确认创建
  const handleSubmitCheckConfirm = useCallback(async () => {
    setSubmitCheckModalVisible(false);
    await handleSubmit();
  }, [handleSubmit]);

  // 处理表单值变更（用于同步到提交检查弹窗）
  const handleSubmitCheckChange = useCallback(
    (newValue: BaseFuncExecutorValue) => {
      formRef.current?.formApi.setValue('config.funcExecutor', newValue);
    },
    [],
  );

  // 处理全屏切换
  const handleFullscreenToggle = useCallback(() => {
    setIsFullscreen(prev => !prev);
  }, []);

  // 处理试运行
  const handleRun = async (ref: React.RefObject<Form<IFormValues>>) => {
    // 试运行不需要校验表单
    try {
      // 获取表单数据
      const formValues = ref.current?.formApi.getValues();
      if (!formValues) {
        return;
      }
      const { config } = formValues;
      const { source, customData, setData } = config?.testData || {};

      // 验证配置
      if (!config?.funcExecutor?.code?.trim()) {
        Toast.info({
          content: I18n.t('evaluate_please_write_function_body'),
          top: 80,
        });
        return;
      }
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

      // 发起 debug 请求
      setIsRunning(true);
      try {
        // 平滑滚动到容器底部
        if (scrollContainerRef.current && !isFullscreen) {
          setTimeout(() => {
            scrollContainerRef.current?.scrollTo({
              top: scrollContainerRef.current?.scrollHeight + SCROLL_OFFSET,
              behavior: 'smooth',
            });
          }, SCROLL_DELAY);
        }
        // 构建调试请求参数
        const res = await StoneEvaluationApi.BatchDebugEvaluator({
          workspace_id: spaceID,
          evaluator_type: EvaluatorType.Code,
          evaluator_content: {
            code_evaluator: {
              code_content: config.funcExecutor.code,
              language_type:
                config.funcExecutor.language ===
                CodeEvaluatorLanguageFE.Javascript
                  ? LanguageType.JS
                  : LanguageType.Python, // 1表示JS，2表示Python
            },
          },
          input_data:
            config.testData?.source === TestDataSource.Custom
              ? [config?.testData?.customData as evaluator.EvaluatorInputData]
              : (config?.testData
                  ?.setData as unknown as evaluator.EvaluatorInputData[]),
        });

        // 处理调试结果
        if (
          !res.evaluator_output_data ||
          res.evaluator_output_data.length === 0
        ) {
          Toast.error({
            content: I18n.t('evaluate_debug_failed_no_result'),
            top: 80,
          });
          return;
        }

        // 收集所有结果
        const allResults = res.evaluator_output_data || [];

        if (allResults.length > 0) {
          // 直接通过表单API更新runResults
          ref.current?.formApi.setValue('config.runResults', allResults);

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
      } finally {
        setIsRunning(false);
      }
    } catch (error) {
      console.error(I18n.t('evaluate_form_validation_failed'), error);
      Toast.error({
        content: `表单验证失败: ${(error as Error)?.message || ''}`,
        top: 80,
      });
    }
  };

  // 处理模板选择
  const handleTemplateSelect = useCallback(
    template => {
      const { code_evaluator } = template || {};
      if (code_evaluator) {
        const { code_template_key, code_template_name, language_type } =
          code_evaluator;

        // 更新URL参数
        const searchParams = new URLSearchParams(location.search);
        searchParams.set('templateKey', code_template_key || '');
        searchParams.set('templateLang', language_type || '');
        window.history.replaceState(
          null,
          '',
          `${location.pathname}?${searchParams.toString()}`,
        );

        // 设置模板信息
        setTemplateInfo({
          key: code_template_key || '',
          name: code_template_name || '',
          lang: language_type || '',
        });

        // 这里可以根据模板内容更新表单值
        if (code_evaluator.code_content && formRef.current) {
          formRef.current.formApi.setValue('config.funcExecutor', {
            code: code_evaluator.code_content,
            language:
              codeEvaluatorLanguageMap[language_type] ||
              CodeEvaluatorLanguageFE.Javascript,
          });
        }

        // 关闭模板选择弹窗
        setTemplateModalVisible(false);
      }
    },
    [location.pathname, location.search],
  );

  // 从URL查询参数中获取模板信息
  useEffect(() => {
    const searchParams = new URLSearchParams(location.search);
    const templateKey = searchParams.get('templateKey');
    const templateLang = searchParams.get('templateLang');
    // 复制评估器
    if (id) {
      StoneEvaluationApi.GetEvaluator({
        workspace_id: spaceID,
        evaluator_id: id,
      }).then(res => {
        const { evaluator } = res;
        if (evaluator) {
          const sourceName = res.evaluator?.name || '';
          const copySubfix = '_Copy';
          const newName = sourceName
            .slice(0, 50 - copySubfix.length)
            .concat(copySubfix);
          const { code_evaluator } =
            evaluator.current_version?.evaluator_content || {};
          formRef.current?.formApi.setValues({
            name: newName,
            description: evaluator.description,
            config: {
              funcExecutor: {
                code: code_evaluator?.code_content || '',
                language:
                  codeEvaluatorLanguageMap[
                    code_evaluator?.language_type as LanguageType
                  ] || CodeEvaluatorLanguageFE.Javascript,
              },
              testData: {
                source: TestDataSource.Custom,
                customData: defaultTestData[0],
              },
            },
          });
        }
      });
    } else if (templateKey) {
      // 获取模板信息
      StoneEvaluationApi.GetTemplateInfo({
        builtin_template_key: templateKey,
        builtin_template_type: TemplateType.Code,
        language_type: (templateLang ||
          CodeEvaluatorLanguageFE.Python) as LanguageType,
      }).then(res => {
        const { code_evaluator } = res.builtin_template || {};
        if (res.builtin_template?.code_evaluator && code_evaluator) {
          setTemplateInfo({
            key: templateKey,
            name:
              code_evaluator.code_template_name ||
              I18n.t('evaluate_unnamed_template'),
            lang: templateLang || '',
          });
          const formApi = formRef.current?.formApi;
          if (!formApi) {
            return;
          }
          const funcExecutorValue = formApi.getValue('config.funcExecutor');
          const name = formApi.getValue('name');

          const extraPayload: Record<string, unknown> = {
            ...funcExecutorValue,
          };

          // 使用表单API更新表单值
          if (code_evaluator.code_content && formApi) {
            extraPayload.code = code_evaluator.code_content;
          }

          // 更新语言选择
          if (code_evaluator.language_type && formApi) {
            extraPayload.language =
              codeEvaluatorLanguageMap[
                code_evaluator.language_type as LanguageType
              ];
          }
          if (!name) {
            formApi.setValue(
              'name',
              code_evaluator.code_template_name ||
                I18n.t('evaluate_unnamed_template'),
            );
          }

          formApi.setValue('config.funcExecutor', extraPayload);
        }
      });
    }
  }, [location.search]);

  return (
    <div className={styles['code-create-page-container']}>
      <CodeCreateHeader />

      <div
        ref={scrollContainerRef}
        className="p-6 pt-[12px] flex-1 overflow-y-auto styled-scrollbar pr-[18px]"
      >
        <Form<IFormValues>
          ref={formRef}
          initValues={{
            config: {
              funcExecutor: {},
              testData: {
                source: TestDataSource.Custom,
                customData: defaultTestData[0],
              },
              runResults: [],
            },
          }}
          className="flex-1 w-[1000px] mx-auto form-default"
        >
          <div className="h-[28px] mb-3 text-[16px] leading-7 font-medium coz-fg-plus">
            {I18n.t('evaluate_basic_info')}
          </div>
          <FormInput
            label={I18n.t('evaluate_name')}
            field="name"
            placeholder={I18n.t('please_input_name')}
            required={true}
            maxLength={50}
            trigger="blur"
            rules={[
              { required: true, message: I18n.t('please_input_name') },
              { max: 50 },
              { validator: sourceNameRuleValidator },
              {
                asyncValidator: async (_, value: string) => {
                  if (value) {
                    const { pass } =
                      await StoneEvaluationApi.CheckEvaluatorName({
                        workspace_id: spaceID,
                        name: value,
                      });
                    if (pass === false) {
                      throw new Error(I18n.t('name_already_exists'));
                    }
                  }
                },
              },
            ]}
          />
          <FormTextArea
            label={I18n.t('description')}
            placeholder={I18n.t('please_input_description')}
            autosize={{ minRows: 1 }}
            maxCount={200}
            maxLength={200}
            rules={[{ max: 200 }]}
            field="description"
          />

          <Divider className="mb-6 mt-[14px]" />

          <div className="h-[28px] mb-3 text-[16px] leading-7 font-medium coz-fg-plus flex flex-row items-center justify-between">
            <span>{I18n.t('evaluate_config')}</span>
            <div className="flex items-center gap-2">
              <Button
                size="mini"
                color="secondary"
                className="!coz-fg-hglt !px-[3px] !h-5"
                icon={<IconCozExpand />}
                onClick={handleFullscreenToggle}
              >
                {I18n.t('evaluate_full_screen')}
              </Button>
              <Button
                size="mini"
                color="secondary"
                className="!coz-fg-hglt !px-[3px] !h-5"
                icon={<IconCozTemplate />}
                onClick={() => setTemplateModalVisible(true)}
              >
                {`${I18n.t('evaluate_template_select')}${
                  templateInfo?.name ? `(${templateInfo.name})` : ''
                }`}
              </Button>
            </div>
          </div>

          {/* Header Banner */}
          <Banner
            justify="start"
            className="!bg-[var(--coz-mg-secondary)] text-[12px] font-normal !text-[color:var(--coz-fg-primary)] rounded-lg"
            description={
              <div>
                {I18n.t('evaluate_test_data_tutorial_tip')}
                <a
                  href={CODE_EVALUATOR_DOCUMENT_LINK}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {I18n.t('evaluate_test_data_tutorial_link')}
                </a>
              </div>
            }
          />
          {/* 代码编辑器 */}
          <div className="h-[500px]">
            <CodeEvaluatorConfig
              disabled={isRunning}
              debugLoading={isRunning}
              field="config"
              fieldPath="config"
              noLabel={true}
              editorHeight="100%"
            />
          </div>
        </Form>
      </div>

      <div className="flex-shrink-0 p-6">
        <div className="w-[1000px] mx-auto flex flex-row justify-end gap-2">
          <Guard point={GuardPoint['eval.evaluator_create.debug']} realtime>
            <Button
              icon={<IconCozPlayFill />}
              color="highlight"
              onClick={() => handleRun(formRef)}
              loading={isRunning}
              disabled={isSubmitting || isRunning}
            >
              {isRunning
                ? I18n.t('evaluate_running')
                : I18n.t('evaluate_trial_run')}
            </Button>
          </Guard>
          <Guard point={GuardPoint['eval.evaluator_create.create']} realtime>
            <Button
              type="primary"
              onClick={handleSubmitCheck}
              loading={isSubmitting}
              disabled={isSubmitting || isRunning}
            >
              {I18n.t('create')}
            </Button>
          </Guard>
        </div>
      </div>

      <CodeTemplateModal
        visible={templateModalVisible}
        isShowCustom={false}
        onCancel={() => setTemplateModalVisible(false)}
        onSelect={handleTemplateSelect}
      />

      <SubmitCheckModal
        formRef={formRef}
        visible={submitCheckModalVisible}
        onCancel={handleSubmitCheckCancel}
        onSubmit={handleSubmitCheckConfirm}
        onChange={handleSubmitCheckChange}
      />

      <FullScreenEditorConfigModal
        formRef={formRef}
        visible={isFullscreen}
        debugLoading={isRunning}
        setVisible={setIsFullscreen}
        onCancel={handleFullscreenToggle}
        onRun={handleRun}
        onChange={vs => {
          const currVs = formRef.current?.formApi.getValues();
          formRef.current?.formApi.setValues({
            ...currVs,
            ...vs,
          });
        }}
      />
    </div>
  );
};

export default CodeEvaluatorCreatePage;
