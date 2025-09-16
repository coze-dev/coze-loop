// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useBaseURL } from '../use-navigate-module';

export function useResourcePageJump() {
  const { baseURL } = useBaseURL();
  const getPromptDetailURL = (promptID: string, version?: string) => {
    const url = `${baseURL}/pe/prompts/${promptID}${version ? `?version=${version}` : ''}`;
    return url;
  };

  const getTagDetailURL = (tagID: string) => {
    const url = `${baseURL}/tag/tag/${tagID}`;
    return url;
  };

  const getTagCreateURL = () => {
    const url = `${baseURL}/tag/tag/create`;
    return url;
  };
  return { getPromptDetailURL, getTagDetailURL, getTagCreateURL };
}
