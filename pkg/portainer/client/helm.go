package client

import (
	"fmt"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// GetHelmRepositories retrieves all Helm repositories for a user.
func (c *PortainerClient) GetHelmRepositories(userId int) (models.HelmRepositoryList, error) {
	raw, err := c.cli.ListHelmRepositories(int64(userId))
	if err != nil {
		return models.HelmRepositoryList{}, fmt.Errorf("failed to list helm repositories: %w", err)
	}

	return models.ConvertToHelmRepositoryList(raw), nil
}

// CreateHelmRepository creates a Helm repository for a user.
func (c *PortainerClient) CreateHelmRepository(userId int, url string) (models.HelmRepository, error) {
	raw, err := c.cli.CreateHelmRepository(int64(userId), url)
	if err != nil {
		return models.HelmRepository{}, fmt.Errorf("failed to create helm repository: %w", err)
	}

	return models.ConvertToHelmRepository(raw), nil
}

// DeleteHelmRepository deletes a Helm repository for a user.
func (c *PortainerClient) DeleteHelmRepository(userId int, repositoryId int) error {
	err := c.cli.DeleteHelmRepository(int64(userId), int64(repositoryId))
	if err != nil {
		return fmt.Errorf("failed to delete helm repository: %w", err)
	}

	return nil
}

// SearchHelmCharts searches for Helm charts in a repository.
func (c *PortainerClient) SearchHelmCharts(repo string, chart string) (string, error) {
	var chartPtr *string
	if chart != "" {
		chartPtr = &chart
	}

	result, err := c.cli.SearchHelmCharts(repo, chartPtr)
	if err != nil {
		return "", fmt.Errorf("failed to search helm charts: %w", err)
	}

	return result, nil
}

// InstallHelmChart installs a Helm chart on an environment.
func (c *PortainerClient) InstallHelmChart(environmentId int, chart, name, namespace, repo, values, version string) (models.HelmReleaseDetails, error) {
	payload := &apimodels.HelmInstallChartPayload{
		Chart:     chart,
		Name:      name,
		Namespace: namespace,
		Repo:      repo,
		Values:    values,
		Version:   version,
	}

	raw, err := c.cli.InstallHelmChart(int64(environmentId), payload)
	if err != nil {
		return models.HelmReleaseDetails{}, fmt.Errorf("failed to install helm chart: %w", err)
	}

	return models.ConvertToHelmReleaseDetails(raw), nil
}

// GetHelmReleases retrieves all Helm releases on an environment.
func (c *PortainerClient) GetHelmReleases(environmentId int, namespace, filter, selector string) ([]models.HelmRelease, error) {
	var nsPtr, filterPtr, selectorPtr *string
	if namespace != "" {
		nsPtr = &namespace
	}
	if filter != "" {
		filterPtr = &filter
	}
	if selector != "" {
		selectorPtr = &selector
	}

	raw, err := c.cli.ListHelmReleases(int64(environmentId), nsPtr, filterPtr, selectorPtr)
	if err != nil {
		return nil, fmt.Errorf("failed to list helm releases: %w", err)
	}

	releases := make([]models.HelmRelease, len(raw))
	for i, r := range raw {
		releases[i] = models.ConvertToHelmRelease(r)
	}

	return releases, nil
}

// DeleteHelmRelease deletes a Helm release from an environment.
func (c *PortainerClient) DeleteHelmRelease(environmentId int, release, namespace string) error {
	var nsPtr *string
	if namespace != "" {
		nsPtr = &namespace
	}

	err := c.cli.DeleteHelmRelease(int64(environmentId), release, nsPtr)
	if err != nil {
		return fmt.Errorf("failed to delete helm release: %w", err)
	}

	return nil
}

// GetHelmReleaseHistory retrieves the history of a Helm release.
func (c *PortainerClient) GetHelmReleaseHistory(environmentId int, name, namespace string) ([]models.HelmReleaseDetails, error) {
	var nsPtr *string
	if namespace != "" {
		nsPtr = &namespace
	}

	raw, err := c.cli.GetHelmReleaseHistory(int64(environmentId), name, nsPtr)
	if err != nil {
		return nil, fmt.Errorf("failed to get helm release history: %w", err)
	}

	details := make([]models.HelmReleaseDetails, len(raw))
	for i, r := range raw {
		details[i] = models.ConvertToHelmReleaseDetails(r)
	}

	return details, nil
}
