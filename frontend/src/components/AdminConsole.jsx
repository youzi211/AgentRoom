import { clearStoredAdminKey } from '../api/roomClient'
import { ADMIN_SECTIONS } from '../routing'
import AgentAdmin from './AgentAdmin'
import MeetingAdmin from './MeetingAdmin'

// AdminConsole is the shared shell for the admin backend. It renders one app
// bar with section tabs and switches between meeting management and agent
// configuration. AgentAdmin renders embedded (without its own app bar).
function AdminConsole({ section, onNavigateSection, onBackHome, onSignOut }) {
  const activeSection = section === ADMIN_SECTIONS.agents ? ADMIN_SECTIONS.agents : ADMIN_SECTIONS.meetings

  const handleSignOut = () => {
    clearStoredAdminKey()
    if (onSignOut) {
      onSignOut()
    }
  }

  return (
    <main className="workbench workbench--admin">
      <header className="app-bar">
        <div className="brand-lockup">
          <span className="brand-mark">AR</span>
          <div>
            <strong>管理后台</strong>
            <span>会议记录与 Agent 管理</span>
          </div>
        </div>
        <nav className="app-nav" aria-label="管理导航">
          <button
            type="button"
            className={`app-nav-item${activeSection === ADMIN_SECTIONS.meetings ? ' app-nav-item--active' : ''}`}
            onClick={() => onNavigateSection(ADMIN_SECTIONS.meetings)}
          >
            会议管理
          </button>
          <button
            type="button"
            className={`app-nav-item${activeSection === ADMIN_SECTIONS.agents ? ' app-nav-item--active' : ''}`}
            onClick={() => onNavigateSection(ADMIN_SECTIONS.agents)}
          >
            Agent 管理
          </button>
          <button className="app-nav-item" type="button" onClick={onBackHome}>
            会议入口
          </button>
          <button className="app-nav-item" type="button" onClick={handleSignOut}>
            退出后台
          </button>
        </nav>
      </header>

      {activeSection === ADMIN_SECTIONS.agents ? <AgentAdmin embedded onBack={onBackHome} /> : <MeetingAdmin />}
    </main>
  )
}

export default AdminConsole
