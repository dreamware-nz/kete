package extract

import (
	_ "embed"
)

//go:embed prompts/extract_task.txt
var promptExtractTask string

//go:embed prompts/extract_decisions.txt
var promptExtractDecisions string

//go:embed prompts/drift_score.txt
var promptDriftScore string

//go:embed prompts/drift_correct.txt
var promptDriftCorrect string

//go:embed prompts/compact_summary.txt
var promptCompactSummary string

// Prompts exposes the embedded extraction prompts to other packages.
// Test surface only — production code uses the typed functions
// (ExtractTask, ExtractDecisions, etc.) which already wire prompts.
var Prompts = map[string]string{
	"extract_task":      promptExtractTask,
	"extract_decisions": promptExtractDecisions,
	"drift_score":       promptDriftScore,
	"drift_correct":     promptDriftCorrect,
	"compact_summary":   promptCompactSummary,
}
