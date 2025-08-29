import { type PEPromptsAdapters } from './prompts';
import { type PEPlaygroundAdapters } from './playground';

export interface PEAdapters {
  'pe.prompts': PEPromptsAdapters;
  'pe.playground': PEPlaygroundAdapters;
}
