// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useNavigate } from 'react-router-dom';

export const useSearchParam = () => {
  const navigate = useNavigate();
  const getSearchParams = (key: string) => {
    const queryString = window.location.search;
    const urlParams = new URLSearchParams(queryString);
    const value = urlParams.get(key);
    urlParams.delete(key);
    if (value) {
      navigate({
        search: `?${urlParams.toString()}`,
      });
    }
    return value;
  };

  return {
    getSearchParams,
  };
};
