import type { ReactElement } from "react";

export type IconName =
  | "activity"
  | "agent"
  | "connector"
  | "dashboard"
  | "flow"
  | "knowledge"
  | "model"
  | "search"
  | "shield"
  | "skill"
  | "tool";

type IconProps = {
  name: IconName;
  label?: string;
};

export function Icon({ name, label }: IconProps) {
  return (
    <svg
      className="icon"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden={label ? undefined : true}
      role={label ? "img" : undefined}
    >
      {label ? <title>{label}</title> : null}
      {paths[name]}
    </svg>
  );
}

const paths: Record<IconName, ReactElement> = {
  activity: (
    <>
      <path d="M4 13h3l2-6 4 10 2-6h5" />
      <path d="M4 20h16" />
    </>
  ),
  agent: (
    <>
      <path d="M12 3a6 6 0 0 0-6 6v3a6 6 0 0 0 12 0V9a6 6 0 0 0-6-6Z" />
      <path d="M8 21h8" />
      <path d="M9 10h.01" />
      <path d="M15 10h.01" />
      <path d="M9 15c1.8 1 4.2 1 6 0" />
    </>
  ),
  connector: (
    <>
      <path d="M8 7H5a3 3 0 0 0 0 6h3" />
      <path d="M16 7h3a3 3 0 0 1 0 6h-3" />
      <path d="M8 10h8" />
      <path d="M12 14v7" />
      <path d="M8 21h8" />
    </>
  ),
  dashboard: (
    <>
      <path d="M4 5h7v6H4z" />
      <path d="M13 5h7v4h-7z" />
      <path d="M13 11h7v8h-7z" />
      <path d="M4 13h7v6H4z" />
    </>
  ),
  flow: (
    <>
      <circle cx="6" cy="6" r="2.5" />
      <circle cx="18" cy="6" r="2.5" />
      <circle cx="12" cy="18" r="2.5" />
      <path d="M8.5 6h7" />
      <path d="m7.7 8.1 3.2 7" />
      <path d="m16.3 8.1-3.2 7" />
    </>
  ),
  knowledge: (
    <>
      <path d="M5 5.5A2.5 2.5 0 0 1 7.5 3H20v16H7.5A2.5 2.5 0 0 0 5 21.5z" />
      <path d="M5 5.5v16" />
      <path d="M9 7h7" />
      <path d="M9 11h6" />
      <path d="M9 15h4" />
    </>
  ),
  model: (
    <>
      <path d="M12 3 4 7v10l8 4 8-4V7z" />
      <path d="m4 7 8 4 8-4" />
      <path d="M12 11v10" />
      <path d="M8 9v4" />
      <path d="M16 9v4" />
    </>
  ),
  search: (
    <>
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="m16 16 4 4" />
    </>
  ),
  shield: (
    <>
      <path d="M12 3 5 6v5c0 4.8 2.8 8.2 7 10 4.2-1.8 7-5.2 7-10V6z" />
      <path d="m9 12 2 2 4-5" />
    </>
  ),
  skill: (
    <>
      <path d="M12 3v4" />
      <path d="M12 17v4" />
      <path d="M4.2 7.5 7.6 9.5" />
      <path d="m16.4 14.5 3.4 2" />
      <path d="m19.8 7.5-3.4 2" />
      <path d="m7.6 14.5-3.4 2" />
      <circle cx="12" cy="12" r="4" />
    </>
  ),
  tool: (
    <>
      <path d="M14.7 6.3a4 4 0 0 0-5 5L4 17v3h3l5.7-5.7a4 4 0 0 0 5-5l-2.6 2.6-3-3z" />
    </>
  )
};
