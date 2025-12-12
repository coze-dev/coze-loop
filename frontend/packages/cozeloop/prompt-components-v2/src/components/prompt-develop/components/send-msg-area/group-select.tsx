import { useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  IconCozArrowDown,
  IconCozInfinity,
  IconCozSingleOperate,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Popover,
  Typography,
  CozInputNumber,
} from '@coze-arch/coze-design';

import { useBasicStore } from '@/store/use-basic-store';
import { MessageListGroupType } from '@/consts';

import { usePromptDevProviderContext } from '../prompt-provider';

interface GroupSelectProps {
  streaming?: boolean;
}

export function GroupSelect({ streaming }: GroupSelectProps) {
  const ctx = usePromptDevProviderContext();
  const [dropVisible, setDropVisible] = useState(false);
  const { groupType, setGroupType } = useBasicStore(
    useShallow(state => ({
      groupType: state.groupType,
      setGroupType: state.setGroupType,
    })),
  );

  const isMultiGroup = groupType === MessageListGroupType.Multi;
  return (
    <div className="flex items-center gap-3">
      <Popover
        trigger="custom"
        visible={dropVisible}
        content={
          <div className="px-4 pt-3 pb-4 w-[350px]">
            <Typography.Text strong className="mb-3 block">
              {I18n.t('prompt_run_mode')}
            </Typography.Text>

            <div className="flex flex-col gap-2">
              <div
                className={classNames(
                  '!h-fit !px-3 !pt-1.5 !pb-3 border border-solid coz-stroke-primary rounded-lg cursor-pointer hover:bg-[#969fff26]',
                  {
                    'coz-stroke-hglt':
                      groupType === MessageListGroupType.Single,
                    'bg-[#969fff26]': groupType === MessageListGroupType.Single,
                  },
                )}
                onClick={() => {
                  setGroupType(MessageListGroupType.Single);
                  setDropVisible(false);
                  ctx.setGroupNum?.(1);
                }}
              >
                <div className="flex items-center gap-3">
                  <IconCozSingleOperate className="flex-shrink-0" />
                  <div className="flex flex-col items-start">
                    <Typography.Text strong style={{ lineHeight: '32px' }}>
                      {I18n.t('prompt_single_run')}
                    </Typography.Text>
                    <Typography.Text
                      size="small"
                      className="!text-[13px] !leading-[20px] !coz-fg-secondary"
                    >
                      {I18n.t('prompt_model_output_single_response')}
                    </Typography.Text>
                  </div>
                </div>
              </div>

              <div
                className={classNames(
                  'items-start !h-fit !px-3 !pt-1.5 !pb-3 border border-solid coz-stroke-primary rounded-lg cursor-pointer hover:bg-[#969fff26]',
                  {
                    'coz-stroke-hglt': isMultiGroup,
                    'bg-[#969fff26]': isMultiGroup,
                  },
                )}
                onClick={() => {
                  setGroupType(MessageListGroupType.Multi);
                  setDropVisible(false);
                  ctx.setGroupNum?.(1);
                }}
              >
                <div className="flex items-center gap-3">
                  <IconCozInfinity className="flex-shrink-0" />
                  <div className="flex flex-col items-start">
                    <Typography.Text strong style={{ lineHeight: '32px' }}>
                      {I18n.t('prompt_multiple_runs')}
                    </Typography.Text>
                    <Typography.Text
                      size="small"
                      className="!text-[13px] !leading-[20px] !coz-fg-secondary"
                    >
                      {I18n.t(
                        'prompt_model_output_multi_response_for_stability_test',
                      )}
                    </Typography.Text>
                  </div>
                </div>
              </div>
            </div>
          </div>
        }
        position="topLeft"
        onClickOutSide={() => setDropVisible(false)}
      >
        <Button
          icon={<IconCozArrowDown />}
          iconPosition="right"
          color="primary"
          className="!border border-solid coz-stroke-primary"
          onClick={() => !streaming && setDropVisible(true)}
          disabled={streaming}
          size="small"
        >
          <Typography.Text
            icon={isMultiGroup ? <IconCozInfinity /> : <IconCozSingleOperate />}
          >
            {isMultiGroup
              ? I18n.t('prompt_multiple_runs')
              : I18n.t('prompt_single_run')}
          </Typography.Text>
        </Button>
      </Popover>
      {isMultiGroup ? (
        <div className="flex items-center gap-2">
          <Typography.Text>{I18n.t('prompt_run_group_count')}</Typography.Text>
          <CozInputNumber
            min={2}
            max={10}
            style={{ width: 50 }}
            value={ctx.groupNum}
            size="small"
            onNumberChange={v => {
              ctx.setGroupNum?.(v);
            }}
            disabled={streaming}
          />
        </div>
      ) : null}
    </div>
  );
}
