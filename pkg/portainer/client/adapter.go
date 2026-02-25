package client

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	sdkclient "github.com/portainer/client-api-go/v2/client"
	swaggerclient "github.com/portainer/client-api-go/v2/pkg/client"
	"github.com/portainer/client-api-go/v2/pkg/client/tags"
	"github.com/portainer/client-api-go/v2/pkg/client/teams"
	"github.com/portainer/client-api-go/v2/pkg/client/users"
)

// portainerAPIAdapter wraps the SDK PortainerClient and adds methods
// that are available in the Swagger-generated client but not exposed
// by the SDK's high-level client (e.g., delete operations).
type portainerAPIAdapter struct {
	*sdkclient.PortainerClient
	swagger *swaggerclient.PortainerClientAPI
}

// newPortainerAPIAdapter creates a new adapter that embeds the SDK high-level
// client and also holds a reference to the low-level Swagger client for
// operations not exposed by the SDK.
func newPortainerAPIAdapter(host, apiKey string, skipTLSVerify bool) *portainerAPIAdapter {
	sdkCli := sdkclient.NewPortainerClient(host, apiKey, sdkclient.WithSkipTLSVerify(skipTLSVerify))

	transport := httptransport.New(host, "/api", []string{"https"})
	if skipTLSVerify {
		transport.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	apiKeyAuth := runtime.ClientAuthInfoWriterFunc(func(r runtime.ClientRequest, _ strfmt.Registry) error {
		return r.SetHeaderParam("x-api-key", apiKey)
	})
	transport.DefaultAuthentication = apiKeyAuth

	return &portainerAPIAdapter{
		PortainerClient: sdkCli,
		swagger:         swaggerclient.New(transport, nil),
	}
}

// DeleteTag deletes a tag by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteTag(id int64) error {
	params := tags.NewTagDeleteParams().WithID(id)
	_, err := a.swagger.Tags.TagDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	return nil
}

// DeleteTeam deletes a team by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteTeam(id int64) error {
	params := teams.NewTeamDeleteParams().WithID(id)
	_, err := a.swagger.Teams.TeamDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	return nil
}

// DeleteUser deletes a user by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteUser(id int64) error {
	params := users.NewUserDeleteParams().WithID(id)
	_, err := a.swagger.Users.UserDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}
