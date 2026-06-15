package agent

import (
	"fmt"

	"agentroom/backend/internal/model"
)

func PredefinedAgents() []model.Agent {
	return []model.Agent{
		newRoleAgent(
			"pm",
			"产品经理",
			"Product Manager",
			"澄清目标、范围、用户价值和产品取舍。",
		),
		newRoleAgent(
			"frontend",
			"前端工程师",
			"Frontend Engineer",
			"关注界面结构、交互细节和前端实现方案。",
		),
		newRoleAgent(
			"backend",
			"后端工程师",
			"Backend Engineer",
			"关注接口设计、数据流、服务边界和稳定性。",
		),
		newRoleAgent(
			"qa",
			"测试工程师",
			"QA Engineer",
			"补充测试策略、验收标准、边界场景和质量风险。",
		),
		{
			ID:           "secretary",
			Name:         "会议秘书",
			Mention:      "@会议秘书",
			Role:         "Meeting Secretary",
			Description:  "整理结论、待办、风险和未决问题。",
			Enabled:      true,
			SystemPrompt: meetingSecretaryPrompt(),
		},
	}
}

func newRoleAgent(id string, name string, role string, description string) model.Agent {
	return model.Agent{
		ID:           id,
		Name:         name,
		Mention:      "@" + name,
		Role:         role,
		Description:  description,
		Enabled:      true,
		SystemPrompt: genericRolePrompt(role, description),
	}
}

func genericRolePrompt(role string, description string) string {
	return fmt.Sprintf(`你是 AgentRoom 会议中的%s。
重点关注：%s
请给出简洁、具体、适合协作推进的建议。
如果上下文不足，请明确说明不确定之处，不要编造事实。`, role, description)
}

func meetingSecretaryPrompt() string {
	return `你是 AgentRoom 的会议秘书。
请基于可见讨论整理结构化会议记录，只总结当前讨论中已经出现的信息。

优先输出：
1. 已达成结论
2. 待办事项
3. 风险
4. 未决问题

保持简洁、准确、便于后续执行。`
}
