function FocusTimeline({ focusPoints = [] }) {
  if (focusPoints.length === 0) {
    return (
      <section className="sidebar-section focus-panel">
        <div className="sidebar-header">
          <h2>会议焦点</h2>
          <span className="sidebar-count">0</span>
        </div>
        <p className="sidebar-empty">发送消息后，AI 将自动分析会议焦点。</p>
      </section>
    )
  }

  const groupedPoints = groupByTime(focusPoints)

  return (
    <section className="sidebar-section focus-panel">
      <div className="sidebar-header">
        <h2>会议焦点</h2>
        <span className="sidebar-count">{focusPoints.length}</span>
      </div>
      <div className="focus-timeline">
        {groupedPoints.map((group, groupIndex) => (
          <div key={groupIndex} className="focus-group">
            <div className="focus-time-label">{group.timeLabel}</div>
            <ul className="focus-list">
              {group.points.map((point, pointIndex) => (
                <li key={point.id || pointIndex} className={`focus-item focus-item--${getCategoryClass(point.category)}`}>
                  <span className="focus-dot" />
                  <div className="focus-content">
                    <span className="focus-text">{point.content}</span>
                    {point.category && <span className="focus-category">{point.category}</span>}
                  </div>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </section>
  )
}

function groupByTime(points) {
  const groups = []
  const now = new Date()

  points.forEach((point) => {
    const pointTime = new Date(point.timestamp)
    const diffMinutes = Math.floor((now - pointTime) / 60000)

    let timeLabel
    if (diffMinutes < 1) {
      timeLabel = '刚刚'
    } else if (diffMinutes < 5) {
      timeLabel = '近5分钟'
    } else if (diffMinutes < 15) {
      timeLabel = '近15分钟'
    } else if (diffMinutes < 30) {
      timeLabel = '近30分钟'
    } else {
      timeLabel = '更早'
    }

    const existingGroup = groups.find((g) => g.timeLabel === timeLabel)
    if (existingGroup) {
      existingGroup.points.push(point)
    } else {
      groups.push({ timeLabel, points: [point] })
    }
  })

  return groups
}

function getCategoryClass(category) {
  if (!category) return 'default'
  const lower = category.toLowerCase()
  if (lower.includes('需求') || lower.includes('requirement')) return 'requirement'
  if (lower.includes('决策') || lower.includes('decision')) return 'decision'
  if (lower.includes('问题') || lower.includes('issue')) return 'issue'
  if (lower.includes('技术') || lower.includes('tech')) return 'tech'
  if (lower.includes('计划') || lower.includes('plan')) return 'plan'
  return 'default'
}

export default FocusTimeline
