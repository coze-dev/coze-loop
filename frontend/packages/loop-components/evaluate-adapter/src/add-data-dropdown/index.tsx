// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type AddDataDropdownType } from '@cozeloop/adapter-interfaces/evaluate';
import { Dropdown, Typography } from '@coze-arch/coze-design';
/**
 * 评测集详情页「添加数据」下拉菜单 适配器
 *
 * 为了使评测集详情页在不同环境下能有不同「添加数据」下拉菜单，因此引入该适配器
 *
 * 理想情况应该将所有菜单配置写在不同适配器内部，根据不同环境调用不同适配器即可，但这种方案需要对现有代码（主要是 pkg 分包/依赖）造成巨大改动，在目前极限倒排条件下难以确保质量。
 * 因此先将原先共有的菜单配置从外部传入，适配器内部仅添加不同环境下新增的菜单。
 */
export const EvaluationAddDataDropdownMenus: AddDataDropdownType = ({
  menuConfigs,
}) => (
  <Dropdown.Menu mode="menu">
    {menuConfigs.map((menu, index) => (
      <Dropdown.Item
        key={index}
        onClick={() => {
          menu.onClick();
        }}
        className="min-w-[90px] !p-0 !pl-2"
      >
        <Typography.Text size="small" className="!text-[13px]">
          {menu.label}
        </Typography.Text>
      </Dropdown.Item>
    ))}
  </Dropdown.Menu>
);
