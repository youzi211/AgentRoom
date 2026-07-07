import { clearStoredAdminKey } from '../api/roomClient'
import { ADMIN_SECTIONS } from '../routing'
import { BarChart3, LogIn, LogOut, Sparkles, Users } from 'lucide-react'
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
    <main className="entry-dashboard admin-dashboard">
      <header className="entry-dashboard-header admin-dashboard-header">
        <div className="entry-dashboard-brand admin-dashboard-brand">
          <span className="entry-brand-symbol" aria-hidden="true">
            <Sparkles size={24} />
          </span>
          <strong>AgentRoom</strong>
        </div>
        <nav className="entry-dashboard-nav admin-dashboard-nav" aria-label="管理导航">
          <button
            type="button"
            className={`entry-dashboard-nav-item${activeSection === ADMIN_SECTIONS.meetings ? ' entry-dashboard-nav-item--active' : ''}`}
            onClick={() => onNavigateSection(ADMIN_SECTIONS.meetings)}
          >
            <BarChart3 size={17} />
            会议管理
          </button>
          <button
            type="button"
            className={`entry-dashboard-nav-item${activeSection === ADMIN_SECTIONS.agents ? ' entry-dashboard-nav-item--active' : ''}`}
            onClick={() => onNavigateSection(ADMIN_SECTIONS.agents)}
          >
            <Users size={17} />
            Agent 管理
          </button>
          <button className="entry-dashboard-nav-item" type="button" onClick={onBackHome}>
            <LogIn size={17} />
            会议入口
          </button>
          <button className="entry-dashboard-nav-item" type="button" onClick={handleSignOut}>
            <LogOut size={17} />
            退出后台
          </button>
        </nav>
      </header>

      <div className="admin-dashboard-layout">
        {activeSection === ADMIN_SECTIONS.agents ? <AgentAdmin embedded onBack={onBackHome} /> : <MeetingAdmin />}
      </div>
    </main>
  )
}

export default AdminConsole
