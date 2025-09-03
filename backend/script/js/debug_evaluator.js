function exec_evaluation(turn) {
    const TARGET_VALUE = "Text";
    try {
        const current = turn.turn.actual_output.text;
        const isEqual = current === TARGET_VALUE;
        const score = isEqual ? 1.0 : 0.0;
        const reason = "Field value: " + current + ", target: " + TARGET_VALUE + ", result: " + (isEqual ? "equal" : "not equal");
        return { score, reason };
    } catch (e) {
        return { score: 0.0, reason: "Error: " + e.message };
    }
}