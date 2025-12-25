// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { create } from 'zustand';
import {
  type ListUserSpaceResponse,
  type Space,
} from '@cozeloop/api-schema/foundation';

import { spaceService } from '../services/space-service';

type SpaceType = Space & { space_role_type?: number };
interface SpaceState {
  space?: SpaceType;
  spaces: SpaceType[];
  total?: number;
}

interface SpaceAction {
  fetchSpaces: (force?: boolean) => ListUserSpaceResponse;
  patch: (state: Partial<SpaceState>) => void;
  reset: () => void;
}

export const PERSONAL_ENTERPRISE_ID = 'personal';

export const useSpaceStore = create<SpaceState & SpaceAction>((set, _get) => {
  // cache list spaces request
  let fetchSpacesTask: Promise<ListUserSpaceResponse> | undefined;

  return {
    spaces: [],
    fetchSpaces: async (force?: boolean) => {
      const currentTask = force
        ? spaceService.listSpaces()
        : fetchSpacesTask || spaceService.listSpaces();

      if (currentTask !== fetchSpacesTask) {
        fetchSpacesTask = currentTask;
      }

      const resp = await currentTask;

      set({ spaces: resp.spaces, total: resp.total });

      return resp;
    },
    patch: state => set({ ...state }),
    reset: () => {
      fetchSpacesTask = undefined;
      set({ space: undefined, spaces: [], total: 0 });
    },
  };
});

export function setSpace(space?: Space) {
  return useSpaceStore.getState().patch({ space });
}
