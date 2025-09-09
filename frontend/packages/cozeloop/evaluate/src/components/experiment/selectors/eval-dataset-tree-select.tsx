// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { TypographyText } from '@cozeloop/evaluate-components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type EvaluationSetVersion,
  type EvaluationSet,
} from '@cozeloop/api-schema/evaluation';
import { Tag, type TreeNodeData, TreeSelect } from '@coze-arch/coze-design';

import { updateTreeData } from './utils';
import { listEvaluationSets, listEvaluationSetVersions } from './requery';

type TreeNode = TreeNodeData & {
  evalSet?: EvaluationSet;
  evalSetVersion?: EvaluationSetVersion;
};

export default function EvalDatasetTreeSelect({
  value,
  onChange,
}: {
  value?: Int64[];
  onChange?: (val: Int64[]) => void;
}) {
  const { spaceID } = useSpace();
  const [treeData, setTreeData] = useState<TreeNode[]>([]);

  const onLoadChildren = async (node: TreeNode) => {
    const res = await listEvaluationSetVersions({
      workspace_id: spaceID,
      page_number: 1,
      page_size: 100,
      evaluation_set_id: node?.evalSet?.id ?? '',
    });
    const newChildren = (res.versions ?? [])?.map(item => {
      const child: TreeNode = {
        label: (
          <span className="flex gap-1 overflow-hidden">
            <span>v{item.version ?? ''}</span>
            <TypographyText style={{ color: 'var(--coz-fg-secondary)' }}>
              {item.description}
            </TypographyText>
          </span>
        ),
        key: item.id?.toString() ?? '',
        value: item.id?.toString() ?? '',
        isLeaf: true,
        evalSet: node?.evalSet,
        evalSetVersion: item,
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
    const res = await listEvaluationSets({
      workspace_id: spaceID,
      page_number: 1,
      page_size: 100,
      name: searchText,
    });
    const newTreeData: TreeNode[] = (res.evaluation_sets ?? [])?.map(item => {
      const node: TreeNode = {
        label: item.name ?? '-',
        key: item.id?.toString() ?? '',
        value: item.id?.toString() ?? '',
        isLeaf: false,
        evalSet: item,
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
      onChange={keys => onChange?.(keys as Int64[])}
      onSearch={searchText => searchService.run(searchText)}
      renderSelectedItem={(item: TreeNode) => {
        const evalSetVersion =
          item?.evalSetVersion ?? item?.evalSet?.evaluation_set_version;
        const name = item.evalSet?.name ?? '-';
        const version = evalSetVersion?.version ?? '';
        return {
          content: (
            <Tag
              color="primary"
              className="inline-flex item-center overflow-hidden"
            >
              {item.isLeaf ? `${name} v${version}` : name}
            </Tag>
          ),
          isRenderInTag: false,
        };
      }}
    />
  );
}
