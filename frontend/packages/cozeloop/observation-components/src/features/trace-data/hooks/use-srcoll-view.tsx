// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef, useState, createContext, useContext } from 'react';

export const useScrollView = () => {
  const ref = useRef<HTMLDivElement>(null);
  const [isFullScreen, setIsFullScreen] = useState(false);
  const onFullScreenStateChange = (newState: boolean) => {
    setIsFullScreen(newState);
    if (ref.current && newState) {
      setTimeout(() => {
        ref.current?.scrollIntoView(false);
      });
    }
  };
  return {
    containerRef: ref,
    isFullScreen,
    onFullScreenStateChange,
  };
};
export const useFullScreen = () => {
  const { isFullScreen } = useContext(FullscreenContext);
  return { isFullScreen };
};

export const FullscreenContext = createContext<{
  isFullScreen: boolean;
}>({
  isFullScreen: false,
});
