// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { Select } from '@coze-arch/coze-design';

interface PlatformSelectProps {
  onChange: (value: string | number) => void;
  value: string | number;
  optionList: { label: string; value: string | number }[];
}

export const PlatformSelect = ({
  onChange,
  value,
  optionList,
}: PlatformSelectProps) => {
  const { spaceID, space: { name: spaceName } = {} } = useSpace();

  return (
    <Select
      className="w-[144px] box-border"
      value={value}
      defaultValue={optionList[0]}
      optionList={optionList}
      onSelect={event => {
        onChange(event as string | number);
        sendEvent(EVENT_NAMES.cozeloop_observation_trace_switch_observe_obj, {
          space_id: spaceID,
          space_name: spaceName ?? '',
          platform: event as string | number,
          module: 'Trace',
        });
      }}
    />
  );
};
