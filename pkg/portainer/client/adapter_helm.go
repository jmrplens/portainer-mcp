package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/helm"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// ListHelmRepositories lists helm repositories for a user.
func (a *portainerAPIAdapter) ListHelmRepositories(userId int64) (*apimodels.UsersHelmUserRepositoryResponse, error) {
	params := helm.NewHelmUserRepositoriesListParams().WithID(userId)
	resp, err := a.swagger.Helm.HelmUserRepositoriesList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list helm repositories: %w", err)
	}
	return resp.Payload, nil
}

// CreateHelmRepository creates a helm repository for a user.
func (a *portainerAPIAdapter) CreateHelmRepository(userId int64, url string) (*apimodels.PortainerHelmUserRepository, error) {
	params := helm.NewHelmUserRepositoryCreateParams().WithID(userId).WithPayload(&apimodels.UsersAddHelmRepoURLPayload{URL: url})
	resp, err := a.swagger.Helm.HelmUserRepositoryCreate(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create helm repository: %w", err)
	}
	return resp.Payload, nil
}

// DeleteHelmRepository deletes a helm repository for a user.
func (a *portainerAPIAdapter) DeleteHelmRepository(userId int64, repositoryId int64) error {
	params := helm.NewHelmUserRepositoryDeleteParams().WithID(userId).WithRepositoryID(repositoryId)
	_, err := a.swagger.Helm.HelmUserRepositoryDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete helm repository: %w", err)
	}
	return nil
}

// SearchHelmCharts searches for helm charts in a repository.
func (a *portainerAPIAdapter) SearchHelmCharts(repo string, chart *string) (string, error) {
	params := helm.NewHelmRepoSearchParams().WithRepo(repo)
	if chart != nil {
		params = params.WithChart(chart)
	}
	resp, err := a.swagger.Helm.HelmRepoSearch(params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to search helm charts: %w", err)
	}
	return resp.Payload, nil
}

// InstallHelmChart installs a helm chart on an environment.
func (a *portainerAPIAdapter) InstallHelmChart(environmentId int64, payload *apimodels.HelmInstallChartPayload) (*apimodels.ReleaseRelease, error) {
	params := helm.NewHelmInstallParams().WithID(environmentId).WithPayload(payload)
	resp, err := a.swagger.Helm.HelmInstall(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to install helm chart: %w", err)
	}
	return resp.Payload, nil
}

// ListHelmReleases lists helm releases on an environment.
func (a *portainerAPIAdapter) ListHelmReleases(environmentId int64, namespace *string, filter *string, selector *string) ([]*apimodels.ReleaseReleaseElement, error) {
	params := helm.NewHelmListParams().WithID(environmentId)
	if namespace != nil {
		params = params.WithNamespace(namespace)
	}
	if filter != nil {
		params = params.WithFilter(filter)
	}
	if selector != nil {
		params = params.WithSelector(selector)
	}
	resp, err := a.swagger.Helm.HelmList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list helm releases: %w", err)
	}
	return resp.Payload, nil
}

// DeleteHelmRelease deletes a helm release from an environment.
func (a *portainerAPIAdapter) DeleteHelmRelease(environmentId int64, release string, namespace *string) error {
	params := helm.NewHelmDeleteParams().WithID(environmentId).WithRelease(release)
	if namespace != nil {
		params = params.WithNamespace(namespace)
	}
	_, err := a.swagger.Helm.HelmDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete helm release: %w", err)
	}
	return nil
}

// GetHelmReleaseHistory gets the history of a helm release.
func (a *portainerAPIAdapter) GetHelmReleaseHistory(environmentId int64, name string, namespace *string) ([]*apimodels.ReleaseRelease, error) {
	params := helm.NewHelmGetHistoryParams().WithID(environmentId).WithName(name)
	if namespace != nil {
		params = params.WithNamespace(namespace)
	}
	resp, err := a.swagger.Helm.HelmGetHistory(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get helm release history: %w", err)
	}
	return resp.Payload, nil
}
