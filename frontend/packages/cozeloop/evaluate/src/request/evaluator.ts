import {
  type GetEvaluatorVersionRequest,
  type GetEvaluatorVersionResponse,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

export async function getEvaluatorVersion(
  params: GetEvaluatorVersionRequest,
): Promise<GetEvaluatorVersionResponse> {
  return StoneEvaluationApi.GetEvaluatorVersion(params);
}
