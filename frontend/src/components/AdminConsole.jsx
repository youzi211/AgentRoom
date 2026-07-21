import { Avatar, Button, Group } from '@mantine/core'
import { clearStoredAdminKey } from '../api/roomClient'
import { ADMIN_SECTIONS } from '../routing'
import { BarChart3, Bot, LogIn, LogOut, Sparkles, Users } from 'lucide-react'
import AgentAdmin from './AgentAdmin'
import MeetingAdmin from './MeetingAdmin'
import ModelProfileAdmin from './ModelProfileAdmin'

// AdminConsole is the shared shell for the admin backend. It renders one app
// bar with section tabs and switches between meeting management and agent
// configuration. AgentAdmin renders embedded (without its own app bar).
function AdminConsole({ section, onNavigateSection, onBackHome, onSignOut }) {
  const activeSection = Object.values(ADMIN_SECTIONS).includes(section) ? section : ADMIN_SECTIONS.meetings

  const handleSignOut = () => {
    clearStoredAdminKey()
    if (onSignOut) {
      onSignOut()
    }
  }

  return (
    <main className="entry-dashboard admin-dashboard">
      <header className="entry-dashboard-header admin-dashboard-header">
        <div className="entry-dashboard-brand admin-dashboard-brand">
          <Avatar className="entry-brand-symbol" radius="sm" color="teal" aria-hidden="true">
            <Sparkles size={24} />
          </Avatar>
          <strong>AgentRoom</strong>
        </div>
        <Group component="nav" className="entry-dashboard-nav admin-dashboard-nav" gap="xs" aria-label="管理导航">
          <Button
            type="button"
            className={`entry-dashboard-nav-item${activeSection === ADMIN_SECTIONS.meetings ? ' entry-dashboard-nav-item--active' : ''}`}
            variant={activeSection === ADMIN_SECTIONS.meetings ? 'light' : 'subtle'}
            color={activeSection === ADMIN_SECTIONS.meetings ? 'teal' : 'gray'}
            leftSection={<BarChart3 size={17} />}
            onClick={() => onNavigateSection(ADMIN_SECTIONS.meetings)}
          >
            会议管理
          </Button>
          <Button
            type="button"
            className={`entry-dashboard-nav-item${activeSection === ADMIN_SECTIONS.models ? ' entry-dashboard-nav-item--active' : ''}`}
            variant={activeSection === ADMIN_SECTIONS.models ? 'light' : 'subtle'}
            color={activeSection === ADMIN_SECTIONS.models ? 'teal' : 'gray'}
            leftSection={<Bot size={17} />}
            onClick={() => onNavigateSection(ADMIN_SECTIONS.models)}
          >
            模型配置
          </Button>
          <Button
            type="button"
            className={`entry-dashboard-nav-item${activeSection === ADMIN_SECTIONS.agents ? ' entry-dashboard-nav-item--active' : ''}`}
            variant={activeSection === ADMIN_SECTIONS.agents ? 'light' : 'subtle'}
            color={activeSection === ADMIN_SECTIONS.agents ? 'teal' : 'gray'}
            leftSection={<Users size={17} />}
            onClick={() => onNavigateSection(ADMIN_SECTIONS.agents)}
          >
            Agent 管理
          </Button>
          <Button className="entry-dashboard-nav-item" type="button" variant="subtle" color="gray" leftSection={<LogIn size={17} />} onClick={onBackHome}>
            会议入口
          </Button>
          <Button className="entry-dashboard-nav-item" type="button" variant="subtle" color="gray" leftSection={<LogOut size={17} />} onClick={handleSignOut}>
            退出后台
          </Button>
        </Group>
      </header>

      <div className="admin-dashboard-layout">
        {activeSection === ADMIN_SECTIONS.agents ? <AgentAdmin embedded onBack={onBackHome} /> : activeSection === ADMIN_SECTIONS.models ? <ModelProfileAdmin /> : <MeetingAdmin />}
      </div>
    </main>
  )
}

export default AdminConsole
