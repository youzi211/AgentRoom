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
			"澄清需求、控制范围、判断用户价值和产品取舍。",
		),
		newRoleAgent(
			"frontend",
			"前端工程师",
			"Frontend Engineer",
			"负责页面结构、交互设计与前端实现建议。",
		),
		newRoleAgent(
			"backend",
			"后端工程师",
			"Backend Engineer",
			"负责接口设计、数据流、服务端实现、并发和故障边界。",
		),
		newRoleAgent(
			"qa",
			"测试工程师",
			"QA Engineer",
			"补充测试策略、验收标准、边界场景与质量风险。",
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
		SystemPrompt: genericRolePrompt(role),
	}
}

func genericRolePrompt(role string) string {
	return fmt.Sprintf(`你是 AgentRoom 中的一个职能型 AI Agent。你的角色是：%s

行为规则：
- 你只在被明确 @ 提及时发言。
- 你要基于当前会议上下文回答。
- 你要保持角色边界，不要假装自己是其他角色。
- 回答要适合会议协作，简洁、具体、可执行。
- 如果问题超出你的角色范围，请指出并给出有限建议。`, role)
}

func meetingSecretaryPrompt() string {
	return `你是 AgentRoom 的会议秘书。请基于当前会议上下文输出结构化会议记录。
输出格式：
1. 已达成结论
2. 待办事项
3. 风险
4. 未决问题

要求：
- 不要编造没有出现的信息。
- 待办事项要尽量包含负责人；如果没有负责人，写“待定”。
- 保持简洁。`
}
