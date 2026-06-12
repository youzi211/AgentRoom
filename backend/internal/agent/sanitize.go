package agent

import (
	"errors"
	"regexp"
	"strings"
)

var (
	thinkTagRe         = regexp.MustCompile("(?is)<think>.*?</think>")
	thinkingTagRe      = regexp.MustCompile("(?is)<thinking>.*?</thinking>")
	unclosedThinkRe    = regexp.MustCompile("(?is)<think>.*")
	unclosedThinkingRe = regexp.MustCompile("(?is)<thinking>.*")
)

// StripThinkBlocks removes private reasoning tags before an agent reply is persisted or broadcast.
func StripThinkBlocks(response string) (string, error) {
	cleaned := thinkTagRe.ReplaceAllString(response, "")
	cleaned = thinkingTagRe.ReplaceAllString(cleaned, "")
	cleaned = unclosedThinkRe.ReplaceAllString(cleaned, "")
	cleaned = unclosedThinkingRe.ReplaceAllString(cleaned, "")

	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "", errors.New("empty response after removing private reasoning")
	}
	return cleaned, nil
}
