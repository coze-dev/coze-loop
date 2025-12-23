// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"github.com/google/wire"
)

var ExperimentMySQLDAOSet = wire.NewSet(
	NewExptDAO,
	NewExptEvaluatorRefDAO,
	NewExptRunLogDAO,
	NewExptStatsDAO,
	NewExptTurnResultDAO,
	NewExptItemResultDAO,
	NewExptTurnEvaluatorResultRefDAO,
	NewExptTurnResultFilterKeyMappingDAO,
	NewExptAggrResultDAO,
	NewExptTurnAnnotateRecordRefDAO,
	NewAnnotateRecordDAO,
	NewExptTurnResultTagRefDAO,
	NewExptResultExportRecordDAO,
	NewExptInsightAnalysisRecordDAO,
	NewExptInsightAnalysisFeedbackVoteDAO,
	NewExptInsightAnalysisFeedbackCommentDAO,
)
