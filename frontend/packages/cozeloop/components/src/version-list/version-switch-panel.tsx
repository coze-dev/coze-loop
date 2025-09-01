import { IconCozCross } from '@coze-arch/coze-design/icons';

import VersionList, { type VersionListProps } from './version-list';

export default function VersionSwitchPanel({
  visible = true,
  onClose,
  ...versionListProps
}: VersionListProps & {
  visible?: boolean;
  onClose?: () => void;
}) {
  if (!visible) {
    return null;
  }
  return (
    <div className="h-full w-[360px] flex flex-col border-0 border-l border-gray-200 border-solid overflow-hidden">
      <div className="shrink-0 flex items-center px-6 h-12 border-0 border-b border-gray-200 border-solid bg-gray-100">
        <div className="text-sm font-medium">版本记录</div>
        <IconCozCross
          className="ml-auto cursor-pointer text-gray-400 hover:text-gray-900"
          onClick={onClose}
        />
      </div>
      <div className="grow py-6 pl-6 pr-[18px] overflow-auto styled-scrollbar">
        <VersionList {...versionListProps} />
      </div>
      <div className="shrink-0"></div>
    </div>
  );
}
