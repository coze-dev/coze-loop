// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export function usePerformance() {
  const checkMarkExists = (markName: string) => {
    const entries = window.performance?.getEntriesByName(markName, 'mark');
    return entries.length > 0;
  };

  const markStart = (mark: string) => {
    const startMark = `${mark}_start`;
    window.performance?.mark(startMark);
  };

  const markEnd = (mark: string) => {
    const startMark = `${mark}_start`;
    const endMark = `${mark}_end`;
    const measureName = `${mark}_measure`;

    if (!checkMarkExists(startMark)) {
      return;
    }

    window.performance?.mark(endMark);
    window.performance?.measure(measureName, startMark, endMark);
    const entry = window.performance?.getEntriesByName(measureName)?.[0];

    if (entry) {
      window.performance?.clearMarks(startMark);
      window.performance?.clearMarks(endMark);
      window.performance?.clearMeasures(measureName);

      return entry.duration;
    }
  };

  return {
    markStart,
    markEnd,
  };
}
