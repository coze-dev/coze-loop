import { ConfigContent } from './config-content';

interface Props {
  disabled?: boolean;
  refreshEditorModelKey?: number;
}

export function PromptConfigField({ disabled, refreshEditorModelKey }: Props) {
  return (
    <>
      <div className="h-[28px] mb-3 text-[16px] leading-7 font-medium coz-fg-plus flex flex-row items-center">
        {'配置信息'}
      </div>
      <ConfigContent
        disabled={disabled}
        refreshEditorModelKey={refreshEditorModelKey}
      />
    </>
  );
}
