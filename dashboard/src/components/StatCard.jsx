export function StatCard({ label, value, tone = 'default', meta }) {
  return (
    <article className={`stat-card stat-card--${tone}`}>
      <span className="stat-card__label">{label}</span>
      <strong className="stat-card__value">{value}</strong>
      {meta ? <span className="stat-card__meta">{meta}</span> : null}
    </article>
  )
}
