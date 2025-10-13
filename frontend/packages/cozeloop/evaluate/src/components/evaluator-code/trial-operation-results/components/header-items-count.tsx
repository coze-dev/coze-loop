/*
 * Copyright 2025 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Divider, Tag } from '@coze-arch/coze-design';

const HeaderItemsCount = ({
  totalCount,
  successCount,
  failedCount,
}: {
  totalCount: number;
  successCount: number;
  failedCount: number;
}) => (
  <Tag color="primary" size="small" className="ml-2">
    总条数 {totalCount || 0}
    <Divider
      layout="vertical"
      style={{ marginLeft: 8, marginRight: 8, height: 12 }}
    />
    成功 {successCount}
    <Divider
      layout="vertical"
      style={{ marginLeft: 8, marginRight: 8, height: 12 }}
    />
    失败 {failedCount}
  </Tag>
);

export default HeaderItemsCount;
