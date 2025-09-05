/* eslint-disable @typescript-eslint/no-explicit-any */
import { Facet } from '@codemirror/state';

export const cunstomFacet = Facet.define<
  Record<string, any>,
  Record<string, any>
>({
  combine(values) {
    return values[values.length - 1];
  },
});
