type BadgeProps = {
  tone?: "blue" | "green" | "amber" | "red" | "gray";
  children: string;
};

export function Badge({ tone = "gray", children }: BadgeProps) {
  return <span className={`badge badge-${tone}`}>{children}</span>;
}
