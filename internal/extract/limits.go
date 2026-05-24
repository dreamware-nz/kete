package extract

// max_tokens budgets per call site. Conservative — Haiku will rarely
// need this much for our structured outputs, but a wrong-shape response
// is cheaper to truncate than to retry.
const (
	MaxTokensExtractTask      = 1024
	MaxTokensExtractDecisions = 1024
	MaxTokensDriftScore       = 256
	MaxTokensDriftCorrect     = 256
	MaxTokensCompactSummary   = 4096
)
