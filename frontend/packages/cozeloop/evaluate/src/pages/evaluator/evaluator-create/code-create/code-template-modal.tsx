/*
 * Copyright 2025 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/* eslint-disable @coze-arch/max-line-per-function */
import { useEffect, useState } from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { CodeEditor } from '@cozeloop/components';
import {
  TemplateType,
  type EvaluatorContent,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozCrossFill } from '@coze-arch/coze-design/icons';
import {
  Button,
  IconButton,
  Modal,
  Select,
  Spin,
} from '@coze-arch/coze-design';

import {
  codeEvaluatorLanguageMapReverse,
  CodeEvaluatorLanguageFE,
} from '@/constants';

import styles from '../template-modal.module.less';

interface CodeTemplateModalProps {
  visible: boolean;
  isShowCustom?: boolean;
  disabled?: boolean;
  onCancel: () => void;
  onSelect: (template?: EvaluatorContent) => void;
}

export function CodeTemplateModal({
  visible,
  isShowCustom = true,
  disabled,
  onCancel,
  onSelect,
}: CodeTemplateModalProps) {
  const [selected, setSelected] = useState<EvaluatorContent>();
  const [selectedLanguage, setSelectedLanguage] =
    useState<CodeEvaluatorLanguageFE>(CodeEvaluatorLanguageFE.Python);
  const [currentData, setCurrentData] = useState<EvaluatorContent>();
  const [codeValue, setCodeValue] = useState<string>();

  const listService = useRequest(
    async () =>
      StoneEvaluationApi.ListTemplates({
        builtin_template_type: TemplateType.Code,
      }),
    {
      manual: true,
      onSuccess: data => {
        const firstItem = data?.builtin_template_keys?.[0];
        if (firstItem) {
          setSelected(firstItem);
        }
      },
    },
  );

  const detailService = useRequest(
    async (lang?: CodeEvaluatorLanguageFE) => {
      const language = lang || selectedLanguage;

      const key = selected?.code_evaluator?.code_template_key;
      if (key) {
        const res = await StoneEvaluationApi.GetTemplateInfo({
          builtin_template_key: key,
          builtin_template_type: TemplateType.Code,
          language_type: codeEvaluatorLanguageMapReverse[language],
        });

        if (res.builtin_template) {
          setCurrentData(res.builtin_template);
          setCodeValue(res.builtin_template.code_evaluator?.code_content || '');
        }
      }
    },
    {
      ready: Boolean(selected),
      refreshDeps: [selected],
    },
  );

  const handleLanguageChange = (value: CodeEvaluatorLanguageFE) => {
    setSelectedLanguage(value);
    detailService.run(value);
  };

  const handleCustomCreate = () => {
    const payload = {
      code_evaluator: {
        code_template_key: 'custom',
        language_type: codeEvaluatorLanguageMapReverse[selectedLanguage],
      },
    };
    onSelect(payload);
  };

  useEffect(() => {
    if (visible && !listService.data) {
      listService.run();
    }
  }, [visible]);

  return (
    <Modal
      className={styles.modal}
      width={1040}
      height="fill"
      visible={visible}
      hasScroll={false}
      header={null}
      footer={null}
    >
      <div className="overflow-hidden w-full h-full flex flex-row">
        <div className="coz-bg-primary w-60 flex flex-col pb-4">
          <div className="m-4 pl-2 h-10 flex items-center text-[20px] coz-fg-plus font-medium">
            {I18n.t('evaluate_select_template')}
          </div>
          <div className="px-4 overflow-y-auto styled-scrollbar pr-[10px]">
            {listService.loading ? (
              <Spin
                spinning={true}
                style={{
                  width: '100%',
                }}
              />
            ) : (
              <>
                <div className="p-2 text-sm leading-4 font-medium coz-fg-secondary mb-1">
                  {I18n.t('preset_evaluator')}
                </div>
                {listService.data?.builtin_template_keys?.map((t, idx) => (
                  <div
                    key={idx}
                    className={classNames(
                      'p-2 text-sm leading-4 font-medium coz-fg-primary rounded-[6px] mb-1 cursor-pointer',
                      selected === t
                        ? 'bg-[#ABB5FF4D]'
                        : 'hover:coz-mg-primary',
                    )}
                    onClick={() => {
                      setSelected(t);
                    }}
                  >
                    {t.code_evaluator?.code_template_name}
                  </div>
                ))}
              </>
            )}
          </div>
        </div>
        <div className="w-0 flex-1 flex flex-col">
          <div className="flex-shrink-0 mx-6 my-4 h-10 flex items-center justify-between text-[20px] coz-fg-plus font-medium">
            {I18n.t('preview')}
            <IconButton
              size="small"
              icon={<IconCozCrossFill className="!w-4 !h-4 coz-fg-secondary" />}
              className="!max-w-[24px] !w-6 !h-6 !p-1"
              color="secondary"
              onClick={onCancel}
            />
          </div>
          <div className="flex-1 px-6 pb-4 pt-0 overflow-y-auto styled-scrollbar pr-[18px]">
            {listService.loading || detailService.loading ? (
              <Spin
                spinning={true}
                style={{
                  width: '100%',
                }}
              />
            ) : (
              <div className="flex flex-col h-full">
                <div className="flex flex-row justify-between mb-3">
                  <div className="font-medium text-xxl coz-fg-plus content-center">
                    {currentData?.code_evaluator?.code_template_name || ''}
                  </div>
                  <Select
                    value={selectedLanguage}
                    onChange={value =>
                      handleLanguageChange(value as CodeEvaluatorLanguageFE)
                    }
                    size="small"
                    className="w-[182px]"
                  >
                    <Select.Option value={CodeEvaluatorLanguageFE.Python}>
                      {CodeEvaluatorLanguageFE.Python}
                    </Select.Option>
                    <Select.Option value={CodeEvaluatorLanguageFE.Javascript}>
                      {CodeEvaluatorLanguageFE.Javascript}
                    </Select.Option>
                  </Select>
                </div>
                <div className="flex-1 min-h-0">
                  <CodeEditor
                    height="100%"
                    language={selectedLanguage}
                    value={codeValue}
                    onChange={value => setCodeValue((value as string) || '')}
                    options={{
                      readOnly: true,
                      minimap: { enabled: false },
                      scrollBeyondLastLine: false,
                    }}
                  />
                </div>
              </div>
            )}
          </div>
          <div className="flex flex-row justify-end gap-2 px-6 pt-2 pb-4">
            {isShowCustom ? (
              <Button color="primary" onClick={handleCustomCreate}>
                {I18n.t('evaluate_create_with_custom')}
              </Button>
            ) : null}
            <Button
              color="brand"
              disabled={!currentData}
              onClick={() => currentData && onSelect(currentData)}
            >
              {I18n.t('evaluate_confirm')}
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
}
