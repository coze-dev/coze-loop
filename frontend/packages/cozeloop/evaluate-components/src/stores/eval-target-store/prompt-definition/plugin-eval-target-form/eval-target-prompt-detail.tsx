import { useMemo, useState } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { type prompt as promptDomain } from '@cozeloop/api-schema/prompt';
import { IconCozArrowRight, IconCozEmpty } from '@coze-arch/coze-design/icons';
import { EmptyState, Loading } from '@coze-arch/coze-design';

import { promptMessageToEvalMessage } from '@/utils/parse-prompt-variable';
import { PromptVariablesList } from '@/components/evaluator/prompt-variables-list';
import { PromptMessage } from '@/components/evaluator/prompt-message';
import { EvaluateModelConfigEditor } from '@/components/evaluate-model-config-editor';

import emptyStyles from '../../empty-state.module.less';

export function EvalTargetPromptDetail(props: {
  promptDetail?: promptDomain.Prompt;
  loading?: boolean;
}) {
  const { promptDetail, loading } = props;
  const [open, setOpen] = useState(false);

  const commitDetail = promptDetail?.prompt_commit?.detail;
  const promptTemplate = commitDetail?.prompt_template;

  const messageList = useMemo(() => {
    if (promptTemplate?.messages) {
      return promptTemplate?.messages?.map(promptMessageToEvalMessage);
    }
    return [];
  }, [promptTemplate]);

  const variableList = promptTemplate?.variable_defs;

  if (loading) {
    return (
      <div className="h-[84px] w-full flex items-center justify-center">
        <Loading
          className="!w-full"
          size="large"
          label={I18n.t('loading_prompt_detail')}
          loading={true}
        />
      </div>
    );
  }

  return (
    <div className="rounded-[10px]">
      <div
        className="h-5 flex flex-row items-center cursor-pointer text-sm coz-fg-primary font-semibold"
        onClick={() => setOpen(pre => !pre)}
      >
        {I18n.t('prompt_details')}
        <IconCozArrowRight
          className={classNames(
            'h-4 w-4 ml-2 coz-fg-plus transition-transform',
            open ? 'rotate-90' : '',
          )}
        />
      </div>

      <div className={classNames('mt-4', open ? '' : 'hidden')}>
        {!promptDetail ? (
          <div className="h-[84px] w-full flex items-center justify-center">
            <EmptyState
              size="default"
              icon={<IconCozEmpty className="coz-fg-dim text-32px" />}
              title={I18n.t('no_data')}
              className={emptyStyles['empty-state']}
              description={I18n.t(
                'cozeloop_open_evaluate_select_prompt_key_version_before_view',
              )}
            />
          </div>
        ) : (
          <div className="mt-4">
            <EvaluateModelConfigEditor
              value={commitDetail?.model_config}
              disabled={true}
            />

            <div className="text-sm font-medium coz-fg-primary mt-3 mb-2">
              {'Prompt'}
            </div>
            {messageList.map((m, idx) => (
              <PromptMessage className="mb-2" key={idx} message={m} />
            ))}

            {variableList?.length ? (
              <PromptVariablesList variables={variableList} />
            ) : null}
          </div>
        )}
      </div>
    </div>
  );
}
