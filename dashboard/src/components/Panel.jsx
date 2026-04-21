export function Panel({ title, subtitle, children, action }) {
  return (
    <section className="panel">
      <header className="panel__header">
        <div>
          <h2>{title}</h2>
          {subtitle ? <p>{subtitle}</p> : null}
        </div>
        {action ? <div>{action}</div> : null}
      </header>
      <div className="panel__body">{children}</div>
    </section>
  )
}
