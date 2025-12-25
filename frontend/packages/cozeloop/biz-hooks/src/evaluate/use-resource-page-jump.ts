// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export function useResourcePageJump() {
  const getPromptDetailURL = (promptID: string, version?: string) => {
    const url = `pe/prompts/${promptID}${version ? `?version=${version}` : ''}`;
    return url;
  };

  const getTagDetailURL = (tagID: string) => `tag/tag/${tagID}`;

  const getTagCreateURL = () => 'tag/tag/create';

  return { getPromptDetailURL, getTagDetailURL, getTagCreateURL };
}
