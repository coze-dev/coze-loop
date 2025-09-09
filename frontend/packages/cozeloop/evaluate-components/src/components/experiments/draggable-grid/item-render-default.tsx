// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export default function ItemRenderDefault({
  item,
  action,
}: {
  item: { id: string };
  action: React.ReactNode;
}) {
  return (
    <div className="flex items-center p-2">
      <div className="flex items-center space-x-2">Item: {item.id}</div>
      <div className="flex items-center ml-auto">{action}</div>
    </div>
  );
}
