// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export const useUserListApi = () => {
  const getUserInfoByUserIds = async ({
    user_ids,
  }: {
    user_ids: string[];
  }): Promise<
    { user_name?: string; user_avatar?: string; user_id?: string }[]
  > => Promise.resolve([]);

  const searchSpaceMemberList = async ({
    search,
    space_id,
  }: {
    search: string;
    space_id: string;
  }): Promise<{ user_name: string; user_avatar: string; user_id: string }[]> =>
    Promise.resolve([]);
  return { getUserInfoByUserIds, searchSpaceMemberList };
};
