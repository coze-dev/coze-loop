// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import VersionDescriptions, { type Version } from './version-descriptions';

interface Props {
  version: Version | undefined;
  active?: boolean;
  className?: string;
  onClick?: () => void;
}

export default function VersionItem({
  version,
  active,
  className,
  onClick,
}: Props) {
  return (
    <div className={`group flex cursor-pointer ${className}`} onClick={onClick}>
      <div className="w-6 h-10 flex items-center shrink-0">
        <div
          className={`w-2 h-2 rounded-full ${active ? 'bg-green-700' : 'bg-gray-300'} `}
        />
      </div>
      <div
        className={`grow px-2 pt-2 rounded-m ${active ? 'bg-gray-100' : ''} group-hover:bg-gray-100`}
      >
        <VersionDescriptions version={version} />
      </div>
    </div>
  );
}
