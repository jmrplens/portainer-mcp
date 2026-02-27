package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// HelmRepository represents a user's Helm repository.
type HelmRepository struct {
	ID     int    `json:"id"`
	URL    string `json:"url"`
	UserID int    `json:"userId"`
}

// HelmRepositoryList represents the response from listing Helm repositories.
type HelmRepositoryList struct {
	GlobalRepository string           `json:"globalRepository"`
	UserRepositories []HelmRepository `json:"userRepositories"`
}

// HelmRelease represents a Helm release (from list endpoint).
type HelmRelease struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Revision   string `json:"revision"`
	Status     string `json:"status"`
	Chart      string `json:"chart"`
	AppVersion string `json:"appVersion"`
	Updated    string `json:"updated"`
}

// HelmReleaseDetails represents a detailed Helm release (from install/history).
type HelmReleaseDetails struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Version    int    `json:"version"`
	AppVersion string `json:"appVersion"`
	Status     string `json:"status"`
}

// ConvertToHelmRepository converts a raw PortainerHelmUserRepository to a local HelmRepository.
func ConvertToHelmRepository(raw *apimodels.PortainerHelmUserRepository) HelmRepository {
	if raw == nil {
		return HelmRepository{}
	}

	return HelmRepository{
		ID:     int(raw.ID),
		URL:    raw.URL,
		UserID: int(raw.UserID),
	}
}

// ConvertToHelmRepositoryList converts a raw UsersHelmUserRepositoryResponse to a local HelmRepositoryList.
func ConvertToHelmRepositoryList(raw *apimodels.UsersHelmUserRepositoryResponse) HelmRepositoryList {
	if raw == nil {
		return HelmRepositoryList{}
	}

	repos := make([]HelmRepository, len(raw.UserRepositories))
	for i, r := range raw.UserRepositories {
		repos[i] = ConvertToHelmRepository(r)
	}

	return HelmRepositoryList{
		GlobalRepository: raw.GlobalRepository,
		UserRepositories: repos,
	}
}

// ConvertToHelmRelease converts a raw ReleaseReleaseElement to a local HelmRelease.
func ConvertToHelmRelease(raw *apimodels.ReleaseReleaseElement) HelmRelease {
	if raw == nil {
		return HelmRelease{}
	}

	return HelmRelease{
		Name:       raw.Name,
		Namespace:  raw.Namespace,
		Revision:   raw.Revision,
		Status:     raw.Status,
		Chart:      raw.Chart,
		AppVersion: raw.AppVersion,
		Updated:    raw.Updated,
	}
}

// ConvertToHelmReleaseDetails converts a raw ReleaseRelease to a local HelmReleaseDetails.
func ConvertToHelmReleaseDetails(raw *apimodels.ReleaseRelease) HelmReleaseDetails {
	if raw == nil {
		return HelmReleaseDetails{}
	}

	status := ""
	if raw.Info != nil {
		status = raw.Info.Status
	}

	return HelmReleaseDetails{
		Name:       raw.Name,
		Namespace:  raw.Namespace,
		Version:    int(raw.Version),
		AppVersion: raw.AppVersion,
		Status:     status,
	}
}
