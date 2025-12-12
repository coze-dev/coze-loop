import { type Dispatch, type SetStateAction } from 'react';

import { create } from 'zustand';
import { produce } from 'immer';

import { MessageListGroupType, MessageListRoundType } from '@/consts';
interface BasicState {
  readonly?: boolean;
  saveLock?: boolean;
  autoSaving?: boolean;
  streaming?: boolean;
  versionChangeVisible?: boolean;
  versionChangeLoading?: boolean;
  roundType?: MessageListRoundType;
  groupType?: MessageListGroupType;
  executeDisabled?: boolean;
}

type PromptActionType<S> = Dispatch<SetStateAction<S>>;
interface BasicAction {
  setReadonly: PromptActionType<boolean | undefined>;
  setAutoSaving: PromptActionType<boolean | undefined>;
  setStreaming: PromptActionType<boolean | undefined>;
  setVersionChangeVisible: PromptActionType<boolean | undefined>;
  setVersionChangeLoading: PromptActionType<boolean | undefined>;
  setSaveLock: PromptActionType<boolean | undefined>;
  setRoundType: PromptActionType<MessageListRoundType | undefined>;
  setGroupType: PromptActionType<MessageListGroupType | undefined>;
  setExecuteDisabled: PromptActionType<boolean | undefined>;
  clearBasicStore: () => void;
}

export const useBasicStore = create<BasicState & BasicAction>()((set, get) => ({
  autoSaving: false,
  saveLock: true,
  setReadonly: (val: SetStateAction<boolean | undefined>) =>
    set(
      produce((state: BasicState) => {
        state.readonly = val instanceof Function ? val(get().readonly) : val;
      }),
    ),
  setAutoSaving: (val: SetStateAction<boolean | undefined>) =>
    set(
      produce((state: BasicState) => {
        state.autoSaving =
          val instanceof Function ? val(get().autoSaving) : val;
      }),
    ),
  streaming: false,
  setStreaming: (val: SetStateAction<boolean | undefined>) =>
    set(
      produce((state: BasicState) => {
        state.streaming = val instanceof Function ? val(get().streaming) : val;
      }),
    ),
  versionChangeVisible: false,
  setVersionChangeVisible: (val: SetStateAction<boolean | undefined>) =>
    set(
      produce((state: BasicState) => {
        state.versionChangeVisible =
          val instanceof Function ? val(get().versionChangeVisible) : val;
      }),
    ),
  versionChangeLoading: false,
  setVersionChangeLoading: (val: SetStateAction<boolean | undefined>) =>
    set(
      produce((state: BasicState) => {
        state.versionChangeLoading =
          val instanceof Function ? val(get().versionChangeLoading) : val;
      }),
    ),
  setSaveLock: (val: SetStateAction<boolean | undefined>) =>
    set(
      produce((state: BasicState) => {
        state.saveLock = val instanceof Function ? val(get().saveLock) : val;
      }),
    ),
  roundType: MessageListRoundType.Multi,
  setRoundType: (val: SetStateAction<MessageListRoundType | undefined>) => {
    set(
      produce((state: BasicState) => {
        state.roundType = val instanceof Function ? val(get().roundType) : val;
      }),
    );
  },
  groupType: MessageListGroupType.Single,
  setGroupType: (val: SetStateAction<MessageListGroupType | undefined>) =>
    set(
      produce((state: BasicState) => {
        state.groupType = val instanceof Function ? val(get().groupType) : val;
      }),
    ),
  executeDisabled: false,
  setExecuteDisabled: (val: SetStateAction<boolean | undefined>) =>
    set(
      produce((state: BasicState) => {
        state.executeDisabled =
          val instanceof Function ? val(get().executeDisabled) : val;
      }),
    ),
  clearBasicStore: () =>
    set({
      autoSaving: false,
      streaming: false,
      versionChangeVisible: false,
      versionChangeLoading: false,
      saveLock: true,
      roundType: MessageListRoundType.Multi,
      groupType: MessageListGroupType.Single,
      executeDisabled: false,
    }),
}));
