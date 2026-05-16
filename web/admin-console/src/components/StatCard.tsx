import type { DashboardMetric } from "../types/platform";

export function StatCard({ metric }: { metric: DashboardMetric }) {
  return (
    <article className={`stat-card stat-${metric.tone}`}>
      <p>{metric.label}</p>
      <strong>{metric.value}</strong>
      <span>{metric.delta}</span>
    </article>
  );
}
