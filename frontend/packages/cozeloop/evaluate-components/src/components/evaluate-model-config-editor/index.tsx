import classNames from 'classnames';
import { PopoverModelConfigEditorQuery } from '@cozeloop/prompt-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { type Scenario, type Model } from '@cozeloop/api-schema/llm-manage';
import { type ModelConfig } from '@cozeloop/api-schema/evaluation';
import { IconCozSetting } from '@coze-arch/coze-design/icons';
import { type PopoverProps } from '@coze-arch/coze-design';

export interface ModelConfigEditorProps {
  value?: ModelConfig;
  onChange?: (value?: ModelConfig) => void;
  onModelChange?: (value?: Model) => void;
  /** 刷新模型数据 */
  refreshModelKey?: number;
  disabled?: boolean;
  popoverProps?: PopoverProps;
  /** 使用场景 */
  scenario?: Scenario;
  [k: string]: unknown;
}

export function EvaluateModelConfigEditor(props: ModelConfigEditorProps) {
  const renderDisplayContent = (
    selectModel?: Model,
    isPopoverVisible?: boolean,
  ) => (
    <div
      className={classNames(
        'flex flex-row items-center h-8 border border-solid coz-stroke-plus rounded-[6px] px-2 hover:coz-mg-primary-hovered active:coz-mg-primary-pressed active:coz-stroke-hglt cursor-pointer',
        {
          '!coz-stroke-hglt': isPopoverVisible,
        },
      )}
    >
      {(selectModel as { icon?: string })?.icon ? (
        <img
          className="w-5 h-5 flex-shrink-0 rounded-sm mr-2"
          src={(selectModel as { icon?: string })?.icon}
        />
      ) : null}
      <div className="flex-1 text-sm coz-fg-primary font-normal">
        {selectModel ? (
          selectModel?.name
        ) : props.value?.model_name ? (
          props.value?.model_name
        ) : (
          <span className="coz-fg-dim">{I18n.t('choose_model')}</span>
        )}
      </div>
      {props.disabled ? (
        <div className="flex-shrink-0 text-sm text-brand-9 font-normal cursor-pointer">
          {I18n.t('view_parameters')}
        </div>
      ) : (
        <IconCozSetting className="flex-shrink-0 w-4 h-4 ml-6px coz-fg-secondary" />
      )}
    </div>
  );

  return (
    <PopoverModelConfigEditorQuery
      defaultActiveFirstModel={true}
      {...props}
      key={props.refreshModelKey}
      value={props.value}
      onChange={v => {
        props.onChange?.(v);
      }}
      renderDisplayContent={renderDisplayContent}
      popoverProps={
        props?.popoverProps ?? {
          position: 'bottomLeft',
        }
      }
    />
  );
}
