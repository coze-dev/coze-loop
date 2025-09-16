// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { TypographyText } from '@cozeloop/evaluate-components';
import {
  type EvalTarget,
  EvalTargetType,
  type EvalTargetVersion,
} from '@cozeloop/api-schema/evaluation';
import { type TreeNodeData, TreeSelect } from '@coze-arch/coze-design';

import { updateTreeData } from '../utils';
import { listSourceEvalTarget, listSourceEvalTargetVersion } from '../requery';

type TreeNode = TreeNodeData & {
  evalTarget?: EvalTarget;
  evalTargetVersion?: EvalTargetVersion;
};

export default function CozeBotEvalTargetTreeSelect({
  spaceID,
  value,
  onChange,
}: {
  spaceID: string;
  value?: string[];
  onChange?: (val: string[]) => void;
}) {
  const [treeData, setTreeData] = useState<TreeNode[]>([]);

  const onLoadChildren = async (node: TreeNode) => {
    const res = await listSourceEvalTargetVersion({
      workspace_id: spaceID,
      source_target_id: node?.evalTarget?.source_target_id?.toString() ?? '',
      target_type: EvalTargetType.CozeBot,
    });
    const newChildren = (res.versions ?? [])?.map(item => {
      const child: TreeNode = {
        label: (
          <span className="flex gap-1">
            <span>v{item.source_target_version ?? ''}</span>
            <TypographyText style={{ color: 'var(--coz-fg-secondary)' }}>
              {item.eval_target_content?.coze_bot?.description}
            </TypographyText>
          </span>
        ),
        key: item.id?.toString() ?? '',
        value: item.id?.toString() ?? '',
        isLeaf: true,
        evalTargetVersion: item,
      };
      return child;
    });
    setTreeData(originTreeData => {
      const newTreeData = updateTreeData(
        originTreeData,
        node.key ?? '',
        newChildren,
      );
      return newTreeData;
    });
  };

  const searchService = useRequest(async (searchText: string | undefined) => {
    const res = await listSourceEvalTarget({
      workspace_id: spaceID,
      target_type: EvalTargetType.CozeBot,
      page_token: '1',
      page_size: 100,
      name: searchText,
    });

    const newTreeData: TreeNode[] = (res.eval_targets ?? [])?.map(item => {
      const node: TreeNode = {
        label:
          item.eval_target_version?.eval_target_content?.coze_bot?.bot_name ??
          '-',
        key: item.source_target_id?.toString() ?? '',
        value: item.source_target_id?.toString() ?? '',
        isLeaf: true,
        evalTarget: item,
      };
      return node;
    });
    setTreeData(newTreeData);
  });

  return (
    <TreeSelect
      loadData={onLoadChildren}
      treeData={treeData}
      style={{ width: '100%' }}
      placeholder={I18n.t('please_select', { field: '' })}
      multiple={true}
      filterTreeNode={true}
      dropdownStyle={{ maxHeight: 400, overflow: 'auto' }}
      maxTagCount={1}
      value={value}
      onChange={keys => onChange?.(keys as string[])}
      onSearch={searchText => searchService.run(searchText)}
      renderSelectedItem={(item: TreeNode) => {
        const evalTargetVersion =
          item?.evalTargetVersion ?? item?.evalTarget?.eval_target_version;
        const name =
          evalTargetVersion?.eval_target_content?.coze_bot?.bot_name ?? '';
        const version = evalTargetVersion?.source_target_version ?? '';
        return {
          content: (
            <div className="flex item-center">
              {item.isLeaf ? `${name} - v${version}` : name}
            </div>
          ),
          isRenderInTag: false,
        };
      }}
    />
  );
}
