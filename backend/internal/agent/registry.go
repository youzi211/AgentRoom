package agent

import "agentroom/backend/internal/model"

func PredefinedAgents() []model.Agent {
	templates := RoleTemplates()
	agents := make([]model.Agent, 0, len(templates))
	for _, template := range templates {
		agents = append(agents, AgentFromTemplate(template))
	}
	return agents
}

func meetingSecretaryPrompt() string {
	return `你是 AgentRoom 的会议纪要。
请基于可见讨论整理结构化会议记录，只总结当前讨论中已经出现的信息。

优先输出：
1. 已达成结论
2. 待办事项
3. 风险
4. 未决问题

保持简洁、准确、便于后续执行。`
}
