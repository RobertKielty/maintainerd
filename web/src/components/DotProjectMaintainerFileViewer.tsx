"use client";

import { useMemo } from "react";
import { isMap, isScalar, isSeq, parseDocument, type Node } from "yaml";
import styles from "./ProjectReconciliationCard.module.css";

type DotProjectMaintainerFileViewerProps = {
  source: string;
  filename: string;
};

type MaintainerTeam = {
  name: string;
  members: string[];
};

type SourceToken = {
  text: string;
  tone?: "comment" | "key" | "punctuation" | "scalar";
};

const scalarText = (node: unknown): string => {
  if (!isScalar(node)) {
    return "";
  }
  return String(node.value ?? "").trim();
};

const mapValue = (node: unknown, key: string): unknown => {
  if (!isMap(node)) {
    return undefined;
  }
  const pair = node.items.find((item) => scalarText(item.key) === key);
  return pair?.value;
};

const scalarSeqValues = (node: unknown): string[] => {
  if (!isSeq(node)) {
    return [];
  }
  return node.items.map(scalarText).filter(Boolean);
};

const parseMaintainerTeams = (source: string): { teams: MaintainerTeam[]; errors: string[] } => {
  const document = parseDocument<Node>(source, { keepSourceTokens: true });
  const maintainers = mapValue(document.contents, "maintainers");
  const teams: MaintainerTeam[] = [];

  if (isSeq(maintainers)) {
    for (const maintainerGroup of maintainers.items) {
      const groupTeams = mapValue(maintainerGroup, "teams");
      if (!isSeq(groupTeams)) {
        continue;
      }
      for (const team of groupTeams.items) {
        const name = scalarText(mapValue(team, "name"));
        if (!name) {
          continue;
        }
        teams.push({
          name,
          members: scalarSeqValues(mapValue(team, "members")),
        });
      }
    }
  }

  return {
    teams,
    errors: document.errors.map((error) => error.message),
  };
};

const findCommentStart = (line: string): number => {
  let quote: "'" | "\"" | null = null;

  for (let index = 0; index < line.length; index += 1) {
    const char = line[index];
    const previous = index > 0 ? line[index - 1] : "";

    if (char === "\"" && quote !== "'" && previous !== "\\") {
      quote = quote === "\"" ? null : "\"";
      continue;
    }
    if (char === "'" && quote !== "\"") {
      quote = quote === "'" ? null : "'";
      continue;
    }
    if (char === "#" && !quote && (index === 0 || /\s/.test(previous))) {
      return index;
    }
  }

  return -1;
};

const tokenizeYamlCode = (code: string): SourceToken[] => {
  if (!code) {
    return [];
  }

  const indentMatch = code.match(/^(\s*)(-\s*)?/);
  const indent = indentMatch?.[1] ?? "";
  const dash = indentMatch?.[2] ?? "";
  const contentStart = indent.length + dash.length;
  const content = code.slice(contentStart);
  const tokens: SourceToken[] = [];

  if (indent) {
    tokens.push({ text: indent });
  }
  if (dash) {
    tokens.push({ text: dash, tone: "punctuation" });
  }

  const keyMatch = content.match(/^([^:\s][^:]*)(:)(\s*)(.*)$/);
  if (keyMatch) {
    tokens.push({ text: keyMatch[1], tone: "key" });
    tokens.push({ text: keyMatch[2], tone: "punctuation" });
    if (keyMatch[3]) {
      tokens.push({ text: keyMatch[3] });
    }
    if (keyMatch[4]) {
      tokens.push({ text: keyMatch[4], tone: "scalar" });
    }
    return tokens;
  }

  if (content) {
    tokens.push({ text: content, tone: "scalar" });
  }

  return tokens;
};

const tokenizeYamlLine = (line: string): SourceToken[] => {
  const commentStart = findCommentStart(line);
  if (commentStart < 0) {
    return tokenizeYamlCode(line);
  }

  return [
    ...tokenizeYamlCode(line.slice(0, commentStart)),
    { text: line.slice(commentStart), tone: "comment" },
  ];
};

const tokenClassName = (token: SourceToken): string | undefined => {
  switch (token.tone) {
    case "comment":
      return styles.yamlTokenComment;
    case "key":
      return styles.yamlTokenKey;
    case "punctuation":
      return styles.yamlTokenPunctuation;
    case "scalar":
      return styles.yamlTokenScalar;
    default:
      return undefined;
  }
};

export default function DotProjectMaintainerFileViewer({
  source,
  filename,
}: DotProjectMaintainerFileViewerProps) {
  const parsed = useMemo(() => parseMaintainerTeams(source), [source]);
  const lines = useMemo(() => source.replace(/\r\n/g, "\n").replace(/\r/g, "\n").split("\n"), [source]);
  const memberCount = parsed.teams.reduce((total, team) => total + team.members.length, 0);

  return (
    <div className={styles.dotProjectYamlViewer}>
      <div className={styles.dotProjectYamlHeader}>
        <div>
          <h4 className={styles.dotProjectYamlTitle}>Formatted maintainer file</h4>
          <p className={styles.dotProjectYamlSubtitle}>
            Parsed from <code>{filename}</code>; source text remains read-only and whitespace-preserving.
          </p>
        </div>
        <div className={styles.dotProjectYamlStats}>
          <span>{parsed.teams.length} teams</span>
          <span>{memberCount} members</span>
        </div>
      </div>

      {parsed.errors.length > 0 ? (
        <div className={styles.dotProjectYamlParseError}>
          YAML parse issue: <strong>{parsed.errors[0]}</strong>
        </div>
      ) : null}

      {parsed.teams.length > 0 ? (
        <div className={styles.dotProjectTeamGrid}>
          {parsed.teams.map((team) => (
            <section className={styles.dotProjectTeamCard} key={team.name}>
              <div className={styles.dotProjectTeamHeader}>
                <span className={styles.dotProjectTeamName}>{team.name}</span>
                <span className={styles.dotProjectTeamMeta}>{team.members.length} members</span>
              </div>
              <div className={styles.dotProjectMemberList}>
                {team.members.map((member) => (
                  <span className={styles.dotProjectMemberChip} key={`${team.name}-${member}`}>
                    {member}
                  </span>
                ))}
              </div>
            </section>
          ))}
        </div>
      ) : (
        <div className={styles.dotProjectYamlParseError}>No maintainer teams were parsed from the cached file.</div>
      )}

      <div className={styles.dotProjectSource} aria-label={`${filename} source`}>
        {lines.map((line, lineIndex) => {
          const tokens = tokenizeYamlLine(line);
          return (
            <div className={styles.dotProjectSourceLine} key={`${lineIndex}-${line}`}>
              <span className={styles.dotProjectLineNumber}>{lineIndex + 1}</span>
              <code className={styles.dotProjectLineCode}>
                {tokens.length > 0
                  ? tokens.map((token, tokenIndex) => (
                      <span className={tokenClassName(token)} key={`${tokenIndex}-${token.text}`}>
                        {token.text}
                      </span>
                    ))
                  : "\u00a0"}
              </code>
            </div>
          );
        })}
      </div>
    </div>
  );
}
