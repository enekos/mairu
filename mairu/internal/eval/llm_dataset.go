package eval

// LLMDataset describes a set of replayable LLM test cases. Inspired by
// LangSmith datasets but kept deliberately small: each case carries the
// prompt(s) to send, the expected shape of the response, and a match
// strategy. The runner does NOT verify model identity or temperature — it
// just compares the response against the expectation.
type LLMDataset struct {
	Description string    `json:"description,omitempty"`
	Model       string    `json:"model,omitempty"`
	Cases       []LLMCase `json:"cases"`
}

// LLMCase is one trace replay.
//
// Mode is one of:
//   - "json"  — runs Provider.GenerateJSON with System+User; expects Response to be valid JSON
//   - "text"  — runs Provider.GenerateContent with User
//
// Match selects how Expected is compared to the actual response:
//   - "exact"      — string equality (whitespace trimmed)
//   - "contains"   — Expected is a substring of the response (case-insensitive)
//   - "json_field" — Expected is parsed as JSON object; every key must match
//     (subset semantics) against the response parsed as JSON
//   - "judge"      — uses an LLM to score response vs Expected on a 0.0–1.0
//     scale; passes when score >= JudgeThreshold (defaults to 0.7)
type LLMCase struct {
	ID             string  `json:"id"`
	Mode           string  `json:"mode"`
	Model          string  `json:"model,omitempty"`
	System         string  `json:"system,omitempty"`
	User           string  `json:"user"`
	Expected       string  `json:"expected"`
	Match          string  `json:"match"`
	JudgeThreshold float64 `json:"judge_threshold,omitempty"`
}

// LLMResult is the outcome of a single replayed case.
type LLMResult struct {
	ID       string `json:"id"`
	Passed   bool   `json:"passed"`
	Response string `json:"response"`
	Reason   string `json:"reason,omitempty"`
	Error    string `json:"error,omitempty"`
}

// LLMMetrics aggregates results across a dataset.
type LLMMetrics struct {
	Total  int     `json:"total"`
	Passed int     `json:"passed"`
	Failed int     `json:"failed"`
	Errors int     `json:"errors"`
	Pass   float64 `json:"pass_rate"`
}
