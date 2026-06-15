package dotproject

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
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

func WriteGistReportCSV(w io.Writer, rows []GistReportRow) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{
		"Project Name",
		"project.yaml",
		"maintainers.yaml",
		"Maintainer Count",
		"SECURITY.md",
		"CONTRIBUTING.md",
		"GOVERNANCE.md",
		"Warning",
	}); err != nil {
		return err
	}
	for _, row := range rows {
		count := ""
		if row.MaintainerCount != nil {
			count = strconv.FormatUint(uint64(*row.MaintainerCount), 10)
		}
		if err := writer.Write([]string{
			row.ProjectName,
			row.ProjectFileURL,
			row.MaintainersFileURL,
			count,
			row.SecurityFileURL,
			row.ContributingFileURL,
			row.GovernanceFileURL,
			row.Warning,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func GistReportCSV(rows []GistReportRow) (string, error) {
	var buf bytes.Buffer
	if err := WriteGistReportCSV(&buf, rows); err != nil {
		return "", fmt.Errorf("write dot-project gist csv: %w", err)
	}
	return buf.String(), nil
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
