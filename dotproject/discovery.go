package dotproject

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"maintainerd/model"

	"github.com/google/go-github/v55/github"
	"gopkg.in/yaml.v3"
)

const (
	DefaultRepoName        = ".project"
	ImporterVersion        = "dot-project-discovery/v1"
	SupportedSchemaVersion = "1.0.0"
	maintainersFileName    = "maintainers.yaml"
)

var (
	ErrDotProjectRepoNotFound = errors.New("dot-project repo not found")
	ErrDotProjectFileNotFound = errors.New("dot-project file not found")
)

type ParseStatus string

const (
	ParseStatusMissing           ParseStatus = "missing"
	ParseStatusParsed            ParseStatus = "parsed"
	ParseStatusUnsupportedSchema ParseStatus = "unsupported_schema"
	ParseStatusInvalidYAML       ParseStatus = "invalid_yaml"
	ParseStatusInvalidShape      ParseStatus = "invalid_shape"
)

type RepositoryClient interface {
	GetDefaultBranch(ctx context.Context, owner, repo string) (string, error)
	GetFile(ctx context.Context, owner, repo, ref, path string) (*FetchedFile, error)
	ListFiles(ctx context.Context, owner, repo, ref, path string) ([]ListedFile, error)
}

type FetchedFile struct {
	Path    string
	BlobURL string
	RawURL  string
	ETag    string
	Body    string
}

type ListedFile struct {
	Path string
}

type FileDiscovery struct {
	Path     string
	Exists   bool
	BlobURL  string
	RawURL   string
	ETag     string
	BodyHash string
	Body     string
}

type DiscoveryResult struct {
	Org      string
	RepoName string

	RepoExists    bool
	RepoRef       string
	DefaultBranch string

	ProjectFile      FileDiscovery
	MaintainersFile  FileDiscovery
	SecurityFile     FileDiscovery
	ContributingFile FileDiscovery
	GovernanceFile   FileDiscovery

	MaintainersFilename    string
	SchemaVersion          string
	SchemaSupported        bool
	MaintainerCount        *uint
	ProjectParseStatus     ParseStatus
	MaintainersParseStatus ParseStatus
	ProjectParseError      string
	MaintainersParseError  string
}

type Discoverer struct {
	Client RepositoryClient
}

func (d *Discoverer) Discover(ctx context.Context, project model.Project) (*DiscoveryResult, error) {
	if d == nil || d.Client == nil {
		return nil, errors.New("dot-project discoverer client is required")
	}

	org := strings.TrimSpace(project.GitHubOrg)
	if org == "" {
		return nil, errors.New("project github org is required")
	}

	result := &DiscoveryResult{
		Org:                    org,
		RepoName:               DefaultRepoName,
		RepoRef:                repoRef(org),
		ProjectParseStatus:     ParseStatusMissing,
		MaintainersParseStatus: ParseStatusMissing,
	}

	defaultBranch, err := d.Client.GetDefaultBranch(ctx, org, DefaultRepoName)
	if err != nil {
		if errors.Is(err, ErrDotProjectRepoNotFound) {
			return result, nil
		}
		return nil, fmt.Errorf("get default branch for %s/%s: %w", org, DefaultRepoName, err)
	}

	result.RepoExists = true
	result.DefaultBranch = defaultBranch

	projectFile, err := d.fetchOptionalFile(ctx, org, defaultBranch, "project.yaml")
	if err != nil {
		return nil, err
	}
	result.ProjectFile = projectFile
	if result.ProjectFile.Exists {
		result.SchemaVersion, result.SchemaSupported, result.ProjectParseStatus, result.ProjectParseError =
			parseProjectYAML(result.ProjectFile)
	}

	maintainersFile, err := d.fetchMaintainersFile(ctx, org, defaultBranch)
	if err != nil {
		return nil, err
	}
	result.MaintainersFile = maintainersFile
	if result.MaintainersFile.Exists {
		result.MaintainersFilename = result.MaintainersFile.Path
		count, status, parseErr := parseMaintainersYAML(result.MaintainersFile)
		result.MaintainerCount = count
		result.MaintainersParseStatus = status
		result.MaintainersParseError = parseErr
	}

	securityFile, err := d.fetchOptionalFile(ctx, org, defaultBranch, "SECURITY.md")
	if err != nil {
		return nil, err
	}
	result.SecurityFile = securityFile

	contributingFile, err := d.fetchOptionalFile(ctx, org, defaultBranch, "CONTRIBUTING.md")
	if err != nil {
		return nil, err
	}
	result.ContributingFile = contributingFile

	governanceFile, err := d.fetchOptionalFile(ctx, org, defaultBranch, "GOVERNANCE.md")
	if err != nil {
		return nil, err
	}
	result.GovernanceFile = governanceFile

	return result, nil
}

func (d *Discoverer) fetchMaintainersFile(ctx context.Context, org, ref string) (FileDiscovery, error) {
	path, ok, err := d.findMaintainersPath(ctx, org, ref)
	if err != nil {
		return FileDiscovery{}, err
	}
	if !ok {
		return FileDiscovery{}, nil
	}
	return d.fetchOptionalFile(ctx, org, ref, path)
}

func (d *Discoverer) findMaintainersPath(ctx context.Context, org, ref string) (string, bool, error) {
	files, err := d.Client.ListFiles(ctx, org, DefaultRepoName, ref, "")
	if err != nil {
		return "", false, fmt.Errorf("list %s/%s root: %w", org, DefaultRepoName, err)
	}
	for _, file := range files {
		path := strings.TrimSpace(file.Path)
		if strings.EqualFold(path, maintainersFileName) {
			return path, true, nil
		}
	}
	return "", false, nil
}

func (d *Discoverer) fetchOptionalFile(ctx context.Context, org, ref, path string) (FileDiscovery, error) {
	fetched, err := d.Client.GetFile(ctx, org, DefaultRepoName, ref, path)
	if err != nil {
		if errors.Is(err, ErrDotProjectFileNotFound) {
			return FileDiscovery{}, nil
		}
		return FileDiscovery{}, fmt.Errorf("fetch %s/%s %s: %w", org, DefaultRepoName, path, err)
	}

	return FileDiscovery{
		Path:     fetched.Path,
		Exists:   true,
		BlobURL:  fetched.BlobURL,
		RawURL:   fetched.RawURL,
		ETag:     fetched.ETag,
		BodyHash: hashBody(fetched.Body),
		Body:     fetched.Body,
	}, nil
}

type projectYAML struct {
	SchemaVersion string `yaml:"schema_version"`
}

func parseProjectYAML(file FileDiscovery) (string, bool, ParseStatus, string) {
	raw := projectYAML{}
	if err := yaml.Unmarshal([]byte(file.Body), &raw); err != nil {
		return "", false, ParseStatusInvalidYAML, err.Error()
	}
	version := strings.TrimSpace(raw.SchemaVersion)
	if version == "" {
		return "", false, ParseStatusInvalidShape, "schema_version is required"
	}
	if version != SupportedSchemaVersion {
		return version, false, ParseStatusUnsupportedSchema, fmt.Sprintf("unsupported schema version %q", version)
	}
	return version, true, ParseStatusParsed, ""
}

func parseMaintainersYAML(file FileDiscovery) (*uint, ParseStatus, string) {
	raw := struct {
		Maintainers []struct {
			Teams []struct {
				Members []string `yaml:"members"`
			} `yaml:"teams"`
		} `yaml:"maintainers"`
	}{}

	if err := yaml.Unmarshal([]byte(file.Body), &raw); err != nil {
		return nil, ParseStatusInvalidYAML, err.Error()
	}
	if len(raw.Maintainers) == 0 {
		return nil, ParseStatusInvalidShape, "maintainers must contain at least one entry"
	}

	handles := make(map[string]struct{})
	for _, maintainerGroup := range raw.Maintainers {
		for _, team := range maintainerGroup.Teams {
			for _, member := range team.Members {
				normalized := normalizeHandle(member)
				if normalized == "" {
					continue
				}
				handles[normalized] = struct{}{}
			}
		}
	}

	count := uint(len(handles))
	return &count, ParseStatusParsed, ""
}

func normalizeHandle(raw string) string {
	handle := strings.TrimSpace(raw)
	handle = strings.TrimPrefix(handle, "@")
	handle = strings.ToLower(handle)
	return handle
}

func hashBody(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func repoRef(org string) string {
	return fmt.Sprintf("https://github.com/%s/%s", org, DefaultRepoName)
}

func blobRef(org, branch, path string) string {
	return fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", org, DefaultRepoName, branch, path)
}

func rawRef(org, branch, path string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", org, DefaultRepoName, branch, path)
}

type GitHubRepositoryClient struct {
	Client *github.Client
}

func (c *GitHubRepositoryClient) GetDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	if c == nil || c.Client == nil {
		return "", errors.New("github client is required")
	}
	repository, resp, err := c.Client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		if isGitHubNotFound(resp, err) {
			return "", ErrDotProjectRepoNotFound
		}
		return "", err
	}
	return strings.TrimSpace(repository.GetDefaultBranch()), nil
}

func (c *GitHubRepositoryClient) GetFile(ctx context.Context, owner, repo, ref, path string) (*FetchedFile, error) {
	if c == nil || c.Client == nil {
		return nil, errors.New("github client is required")
	}
	file, _, resp, err := c.Client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		if isGitHubNotFound(resp, err) {
			return nil, ErrDotProjectFileNotFound
		}
		return nil, err
	}
	if file == nil {
		return nil, ErrDotProjectFileNotFound
	}

	body, err := file.GetContent()
	if err != nil {
		return nil, err
	}

	etag := ""
	if resp != nil {
		etag = resp.Header.Get("ETag")
	}

	fetched := &FetchedFile{
		Path:    path,
		BlobURL: strings.TrimSpace(file.GetHTMLURL()),
		RawURL:  strings.TrimSpace(file.GetDownloadURL()),
		ETag:    etag,
		Body:    body,
	}
	if fetched.BlobURL == "" {
		fetched.BlobURL = blobRef(owner, ref, path)
	}
	if fetched.RawURL == "" {
		fetched.RawURL = rawRef(owner, ref, path)
	}
	return fetched, nil
}

func (c *GitHubRepositoryClient) ListFiles(ctx context.Context, owner, repo, ref, path string) ([]ListedFile, error) {
	if c == nil || c.Client == nil {
		return nil, errors.New("github client is required")
	}
	_, dir, resp, err := c.Client.Repositories.GetContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		if isGitHubNotFound(resp, err) {
			return nil, ErrDotProjectFileNotFound
		}
		return nil, err
	}
	files := make([]ListedFile, 0, len(dir))
	for _, entry := range dir {
		if entry == nil {
			continue
		}
		files = append(files, ListedFile{Path: strings.TrimSpace(entry.GetPath())})
	}
	return files, nil
}

func isGitHubNotFound(resp *github.Response, err error) bool {
	if resp != nil && resp.StatusCode == 404 {
		return true
	}
	var ghErr *github.ErrorResponse
	if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == 404 {
		return true
	}
	return false
}
