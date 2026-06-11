package agent

import (
	"sort"
	"strings"

	"agentroom/backend/internal/model"
)

type mentionMatch struct {
	agent model.Agent
	index int
	order int
}

func MentionedAgents(message model.Message, agents []model.Agent) []model.Agent {
	if message.SenderType != model.SenderTypeHuman {
		return nil
	}
	return DetectMentions(message.Content, agents)
}

func DetectMentions(content string, agents []model.Agent) []model.Agent {
	text := strings.TrimSpace(content)
	if text == "" {
		return nil
	}

	matches := make([]mentionMatch, 0)
	for order, candidate := range agents {
		idx := strings.Index(text, candidate.Mention)
		if idx == -1 {
			continue
		}
		matches = append(matches, mentionMatch{
			agent: candidate,
			index: idx,
			order: order,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].index == matches[j].index {
			return matches[i].order < matches[j].order
		}
		return matches[i].index < matches[j].index
	})

	result := make([]model.Agent, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if _, ok := seen[match.agent.ID]; ok {
			continue
		}
		seen[match.agent.ID] = struct{}{}
		result = append(result, match.agent)
	}

	return result
}
