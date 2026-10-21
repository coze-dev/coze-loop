package experiment

import (
	"fmt"
	"github.com/bytedance/gg/gptr"
	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	apiExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
)

// A single fixed dataset selection is enough to create a result-set experiment.
func NormalizeResultSetCreateRequest(req *apiExpt.CreateExperimentRequest) error {
	if req == nil || req.GetRunModeConfig().GetRunMode() != domainExpt.ExptRunMode_ResultSet {
		return nil
	}
	switch req.GetEvalSetSourceType() {
	case 0, domainExpt.ExptEvalSetSourceType_SingleSet:
		if len(req.EvalSetConfigs) > 0 {
			return fmt.Errorf("result_set single-set selection conflicts with eval_set_configs")
		}
		if req.GetEvalSetID() <= 0 || req.GetEvalSetVersionID() <= 0 {
			return fmt.Errorf("result_set requires a fixed evaluation set version")
		}
		req.EvalSetConfigs = []*domainExpt.EvalSetConfig{{EvalSetID: req.GetEvalSetID(), EvalSetVersionID: req.GetEvalSetVersionID(), SharedOption: req.EvalSetSharedOption}}
		req.EvalSetSourceType = gptr.Of(domainExpt.ExptEvalSetSourceType_MultiSetConfig)
	case domainExpt.ExptEvalSetSourceType_MultiSetConfig:
		if len(req.EvalSetConfigs) == 0 {
			return fmt.Errorf("result_set requires eval_set_configs")
		}
	default:
		return fmt.Errorf("invalid result_set eval_set_source_type")
	}
	first := req.EvalSetConfigs[0]
	if first == nil || first.GetEvalSetID() <= 0 || first.GetEvalSetVersionID() <= 0 {
		return fmt.Errorf("result_set requires a fixed evaluation set version")
	}
	if (req.GetEvalSetID() != 0 && req.GetEvalSetID() != first.GetEvalSetID()) || (req.GetEvalSetVersionID() != 0 && req.GetEvalSetVersionID() != first.GetEvalSetVersionID()) {
		return fmt.Errorf("result_set top-level dataset selection conflicts with eval_set_configs")
	}
	req.EvalSetID = gptr.Of(first.GetEvalSetID())
	req.EvalSetVersionID = gptr.Of(first.GetEvalSetVersionID())
	if req.EvalSetSharedOption == nil {
		req.EvalSetSharedOption = first.SharedOption
	}
	return nil
}
