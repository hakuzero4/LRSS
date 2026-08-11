package llm

import (
	"fmt"
	"strings"
)

// ErrCompletionTruncated is returned when the model hit a token/length limit
// (or equivalent incomplete status). Callers must not treat the partial text
// as a successful feature result for cache or persistence.
var ErrCompletionTruncated = fmt.Errorf("llm: completion truncated (finish_reason=length)")

// IsIncompleteCompletion reports whether finish_reason means the model stopped
// early due to length/token limits rather than a natural stop.
func IsIncompleteCompletion(finishReason string) bool {
	switch strings.ToLower(strings.TrimSpace(finishReason)) {
	case "length", "max_tokens", "max_output_tokens":
		return true
	default:
		return false
	}
}

// RejectIfIncomplete returns ErrCompletionTruncated when the completion is
// truncated; otherwise nil. Empty finish reasons are treated as complete
// (providers often omit the field on success).
func RejectIfIncomplete(finishReason string) error {
	if IsIncompleteCompletion(finishReason) {
		return ErrCompletionTruncated
	}
	return nil
}
