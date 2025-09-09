// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export default function TableHeader({
  filters,
  actions,
  className,
  style,
}: {
  filters?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
}) {
  return (
    <div className={`flex items-center ${className}`} style={style}>
      <div className="flex items-center gap-2 grow">{filters}</div>
      <div className="flex items-center gap-2 ml-auto">{actions}</div>
    </div>
  );
}
