function NotFound({ onBackHome }) {
  return (
    <main className="join-screen">
      <section className="join-card join-card--narrow">
        <div className="topbar">
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
