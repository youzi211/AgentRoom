package agent

import (
	"fmt"

	"agentroom/backend/internal/model"
)

type RoleTemplate struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
}

type RoleSet struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	TemplateIDs []string `json:"templateIDs"`
}

func RoleTemplates() []RoleTemplate {
	templates := []RoleTemplate{
		{
			ID:          "product_manager",
			Name:        "产品经理",
			Role:        "Product Manager",
			Description: "澄清目标、范围、用户价值和产品取舍。",
		},
		{
			ID:          "architect",
			Name:        "架构师",
			Role:        "Architect",
			Description: "关注系统边界、接口契约、数据流和长期演进风险。",
		},
		{
			ID:          "qa_reviewer",
			Name:        "质量评审",
			Role:        "QA Reviewer",
			Description: "补充验收标准、测试策略、边界场景和回归风险。",
		},
		{
			ID:          "risk_reviewer",
			Name:        "风险评审",
			Role:        "Risk Reviewer",
			Description: "识别交付、依赖、安全、合规和运维风险。",
		},
		{
			ID:          "meeting_scribe",
			Name:        "会议纪要",
			Role:        "Meeting Scribe",
			Description: "整理结论、待办、风险和未决问题。",
		},
	}

	for i := range templates {
		if templates[i].SystemPrompt == "" {
			templates[i].SystemPrompt = roleTemplatePrompt(templates[i])
		}
	}
	return templates
}

func RoleSets() []RoleSet {
	return []RoleSet{
		{
			ID:          "product_review",
			Name:        "产品评审",
			Description: "适合需求、方案和发布风险评审。",
			TemplateIDs: []string{"product_manager", "architect", "qa_reviewer", "risk_reviewer"},
		},
		{
			ID:          "delivery_review",
			Name:        "交付复盘",
			Description: "适合整理结论、待办、质量风险和后续推进。",
			TemplateIDs: []string{"meeting_scribe", "qa_reviewer", "risk_reviewer"},
		},
	}
}

func AgentFromTemplate(template RoleTemplate) model.Agent {
	return model.Agent{
		ID:           predefinedAgentID(template.ID),
		Name:         template.Name,
		Mention:      "@" + template.Name,
		Role:         template.Role,
		Runtime:      model.AgentRuntimeLLM,
		Source:       model.AgentSourceBuiltin,
		Description:  template.Description,
		SystemPrompt: template.SystemPrompt,
		Enabled:      true,
	}
}

func predefinedAgentID(templateID string) string {
	switch templateID {
	case "product_manager":
		return "pm"
	case "qa_reviewer":
		return "qa"
	case "risk_reviewer":
		return "risk"
	case "meeting_scribe":
		return "secretary"
	default:
		return templateID
	}
}

func roleTemplatePrompt(template RoleTemplate) string {
	if template.ID == "meeting_scribe" {
		return meetingSecretaryPrompt()
	}
	return fmt.Sprintf(`你是 AgentRoom 会议中的%s。
重点关注：%s
请给出简洁、具体、适合协作推进的建议。如果上下文不足，请明确说明不确定之处，不要编造事实。`, template.Role, template.Description)
}
