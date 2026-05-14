"use client";

import styles from "./ProjectReconciliationCard.module.css";

type GitHubHandleProps = {
  id: number;
  github: string;
  name: string;
  prefixName?: boolean;
};

export default function GitHubHandle({ id, github, name, prefixName = false }: GitHubHandleProps) {
  const displayHandle = github.startsWith("@") ? github : `@${github}`;
  const label = prefixName ? `${name} ${displayHandle}` : displayHandle;

  return (
    <a className={styles.githubHandle} href={`/maintainers/${id}`} title={name}>
      <span>{label}</span>
      <span className={styles.githubHandleTooltip} role="tooltip">
        {name}
      </span>
    </a>
  );
}
