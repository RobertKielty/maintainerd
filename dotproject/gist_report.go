package dotproject

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"

	"maintainerd/model"
)

type GistReportRow struct {
	ProjectName         string `json:"project_name"`
	ProjectFileURL      string `json:"project_file_url"`
	MaintainersFileURL  string `json:"maintainers_file_url"`
	MaintainerCount     *uint  `json:"maintainer_count,omitempty"`
	SecurityFileURL     string `json:"security_file_url"`
	ContributingFileURL string `json:"contributing_file_url"`
	GovernanceFileURL   string `json:"governance_file_url"`
	Warning             string `json:"warning,omitempty"`
}

func BuildGistReportRow(project model.Project, result *DiscoveryResult) (GistReportRow, bool) {
	if result == nil || !result.RepoExists {
		return GistReportRow{}, false
	}
	return GistReportRow{
		ProjectName:         strings.TrimSpace(project.Name),
		ProjectFileURL:      fileURL(result.ProjectFile),
		MaintainersFileURL:  fileURL(result.MaintainersFile),
		MaintainerCount:     result.MaintainerCount,
		SecurityFileURL:     fileURL(result.SecurityFile),
		ContributingFileURL: fileURL(result.ContributingFile),
		GovernanceFileURL:   fileURL(result.GovernanceFile),
		Warning:             gistWarning(result),
	}, true
}

func WriteGistReportMarkdown(w io.Writer, rows []GistReportRow) error {
	if _, err := io.WriteString(w, "| Project Name | project.yaml | maintainers.yaml | Maintainer Count | SECURITY.md | CONTRIBUTING.md | GOVERNANCE.md | Warning |\n"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "| --- | --- | --- | ---: | --- | --- | --- | --- |\n"); err != nil {
		return err
	}
	for _, row := range rows {
		count := ""
		if row.MaintainerCount != nil {
			count = strconv.FormatUint(uint64(*row.MaintainerCount), 10)
		}
		if _, err := fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			markdownTableCell(row.ProjectName),
			markdownLinkForURL(row.ProjectFileURL),
			markdownLinkForURL(row.MaintainersFileURL),
			count,
			markdownLinkForURL(row.SecurityFileURL),
			markdownLinkForURL(row.ContributingFileURL),
			markdownLinkForURL(row.GovernanceFileURL),
			markdownTableCell(row.Warning),
		); err != nil {
			return err
		}
	}
	return nil
}

func GistReportMarkdown(rows []GistReportRow) (string, error) {
	var buf bytes.Buffer
	if err := WriteGistReportMarkdown(&buf, rows); err != nil {
		return "", fmt.Errorf("write dot-project gist markdown: %w", err)
	}
	return buf.String(), nil
}

func markdownLinkForURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	label := markdownLinkLabel(raw)
	return fmt.Sprintf("[%s](%s)", markdownLinkText(label), markdownURL(raw))
}

func markdownLinkLabel(raw string) string {
	if parsed, err := url.Parse(raw); err == nil {
		if base := path.Base(strings.TrimRight(parsed.Path, "/")); base != "." && base != "/" && base != "" {
			return base
		}
	}
	trimmed := strings.TrimRight(raw, "/")
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 && idx+1 < len(trimmed) {
		return trimmed[idx+1:]
	}
	return raw
}

func markdownTableCell(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}

func markdownLinkText(value string) string {
	value = markdownTableCell(value)
	value = strings.ReplaceAll(value, "[", "\\[")
	value = strings.ReplaceAll(value, "]", "\\]")
	return value
}

func markdownURL(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), ")", "%29")
}

func fileURL(file FileDiscovery) string {
	if !file.Exists {
		return ""
	}
	if value := strings.TrimSpace(file.BlobURL); value != "" {
		return value
	}
	return strings.TrimSpace(file.RawURL)
}

func gistWarning(result *DiscoveryResult) string {
	if result == nil || !result.MaintainersFile.Exists || result.MaintainersParseStatus == ParseStatusParsed {
		return ""
	}
	message := strings.TrimSpace(result.MaintainersParseError)
	if message == "" {
		message = string(result.MaintainersParseStatus)
	}
	ref := fileURL(result.MaintainersFile)
	if ref == "" {
		return "maintainers.yaml warning: " + message
	}
	return fmt.Sprintf("maintainers.yaml warning: %s (%s)", message, ref)
}
