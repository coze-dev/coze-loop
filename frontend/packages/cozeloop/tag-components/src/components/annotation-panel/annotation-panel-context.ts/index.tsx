import React from 'react';

import { PlatformType } from '@cozeloop/api-schema/observation';

interface AnnotationPanelContextProps {
  platformType?: string | number;
  saveLoading?: boolean;
  setSaveLoading?: (loading: boolean) => void;
  editChanged?: boolean;
  setEditChanged?: (changed: boolean) => void;
}

export const AnnotationPanelContext =
  React.createContext<AnnotationPanelContextProps>({
    platformType: PlatformType.Cozeloop,
    saveLoading: false,
  });

export const useAnnotationPanelContext = () =>
  React.useContext(AnnotationPanelContext);
