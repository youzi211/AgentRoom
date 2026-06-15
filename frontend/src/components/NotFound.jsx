function NotFound({ onBackHome }) {
  return (
    <main className="workbench workbench--center">
      <section className="panel direct-entry-panel">
        <div className="panel-header panel-header--horizontal">
          <div>
            <p className="eyebrow">404</p>
            <h1>这个页面不存在</h1>
            <p className="section-copy">请检查链接是否完整，或返回会议入口重新创建、加入房间。</p>
          </div>
          <button className="button button--primary" type="button" onClick={onBackHome}>
            返回入口
          </button>
        </div>
      </section>
    </main>
  )
}

export default NotFound
