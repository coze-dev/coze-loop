import { useResourcePageJump } from '@cozeloop/biz-hooks-adapter';
import { type EvalTarget } from '@cozeloop/api-schema/evaluation';

import { BaseTargetPreview } from '../base-target-preview';

const PromptIcon = (
  <div className="flex items-center mr-1">
    <svg
      width="20"
      height="20"
      viewBox="0 0 20 20"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <rect width="20" height="20" rx="4" fill="#B0B9FF" />
      <path
        d="M7.71101 5.64889H8.41323V6.42222H7.88879C7.56879 6.42222 7.40879 6.6 7.40879 6.97333V8.68C7.40879 9.24 7.15101 9.61333 6.63545 9.8C7.15101 10.0133 7.40879 10.3867 7.40879 10.92V12.6356C7.40879 12.9911 7.56879 13.1778 7.88879 13.1778H8.41323V13.9511H7.71101C7.30212 13.9511 6.99101 13.8178 6.77767 13.56C6.58212 13.32 6.48434 13 6.48434 12.6V10.9733C6.48434 10.7156 6.43101 10.5289 6.32434 10.4222C6.1999 10.28 5.97767 10.2 5.66656 10.1822V9.41778C5.97767 9.39111 6.1999 9.31111 6.32434 9.18667C6.43101 9.06222 6.48434 8.87556 6.48434 8.62667V7.00889C6.48434 6.6 6.58212 6.28 6.77767 6.04C6.99101 5.77333 7.30212 5.64889 7.71101 5.64889ZM11.5867 5.64889H12.2889C12.6889 5.64889 13 5.77333 13.2223 6.04C13.4178 6.28 13.5156 6.6 13.5156 7.00889V8.62667C13.5156 8.87556 13.5689 9.07111 13.6845 9.19556C13.8 9.32 14.0134 9.39111 14.3334 9.41778V10.1822C14.0134 10.2 13.8 10.28 13.6756 10.4222C13.5689 10.5289 13.5156 10.7156 13.5156 10.9733V12.6C13.5156 13 13.4178 13.32 13.2223 13.56C13 13.8178 12.6889 13.9511 12.2889 13.9511H11.5867V13.1778H12.1111C12.4311 13.1778 12.5911 12.9911 12.5911 12.6356V10.92C12.5911 10.3867 12.8489 10.0133 13.3645 9.8C12.8489 9.61333 12.5911 9.24 12.5911 8.68V6.97333C12.5911 6.6 12.4311 6.42222 12.1111 6.42222H11.5867V5.64889Z"
        fill="white"
      />
    </svg>
  </div>
);

export default function PromptTargetPreview({
  evalTarget,
  enableLinkJump,
  jumpBtnClassName,
  showIcon = false,
}: {
  evalTarget: EvalTarget | undefined;
  enableLinkJump?: boolean;
  jumpBtnClassName?: string;
  showIcon?: boolean;
}) {
  const { name, version, prompt_id } =
    evalTarget?.eval_target_version?.eval_target_content?.prompt ?? {};
  const { getPromptDetailURL } = useResourcePageJump();
  return (
    <div className="flex">
      {showIcon ? PromptIcon : null}
      <BaseTargetPreview
        name={name ?? '-'}
        version={version}
        onClick={e => {
          if (!prompt_id) {
            return;
          }
          e.stopPropagation();
          const url = getPromptDetailURL(prompt_id, version);
          window.open(url);
        }}
        enableLinkJump={enableLinkJump}
        jumpBtnClassName={jumpBtnClassName}
      />
    </div>
  );
}
