package client

import (
	"fmt"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/portainer/client-api-go/v2/pkg/client/roles"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// ListRoles lists all roles.
func (a *portainerAPIAdapter) ListRoles() ([]*apimodels.PortainereeRole, error) {
	params := roles.NewRoleListParams()
	resp, err := a.swagger.Roles.RoleList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	return resp.Payload, nil
}

// GetMOTD retrieves the message of the day.
func (a *portainerAPIAdapter) GetMOTD() (map[string]any, error) {
	// Use raw HTTP to avoid SDK Hash type mismatch
	// (SDK defines Hash as []int64, but newer API versions return a string).
	op := &runtime.ClientOperation{
		ID:                 "MOTD",
		Method:             "GET",
		PathPattern:        "/motd",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{a.scheme},
		Params: runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, reg strfmt.Registry) error {
			return nil
		}),
		AuthInfo: a.httpTransport.DefaultAuthentication,
		Reader: runtime.ClientResponseReaderFunc(func(resp runtime.ClientResponse, consumer runtime.Consumer) (any, error) {
			var result map[string]any
			if err := consumer.Consume(resp.Body(), &result); err != nil {
				return nil, err
			}
			return result, nil
		}),
	}
	res, err := a.httpTransport.Submit(op)
	if err != nil {
		return nil, fmt.Errorf("failed to get MOTD: %w", err)
	}
	return res.(map[string]any), nil
}
