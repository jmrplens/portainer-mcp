package client

import (
	"errors"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/stretchr/testify/assert"
)

// TestGetWebhooks verifies retrieval and conversion of webhooks.
func TestGetWebhooks(t *testing.T) {
	tests := []struct {
		name          string
		mockWebhooks  []*apimodels.PortainerWebhook
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockWebhooks: []*apimodels.PortainerWebhook{
				{ID: 1, Token: "abc123", ResourceID: "svc1", EndpointID: 1, Type: 1},
				{ID: 2, Token: "def456", ResourceID: "svc2", EndpointID: 2, Type: 1},
			},
			expectedCount: 2,
		},
		{
			name:          "empty list",
			mockWebhooks:  []*apimodels.PortainerWebhook{},
			expectedCount: 0,
		},
		{
			name:          "API error",
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("ListWebhooks").Return(tt.mockWebhooks, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetWebhooks()

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestCreateWebhook verifies creation of a webhook.
func TestCreateWebhook(t *testing.T) {
	tests := []struct {
		name          string
		resourceId    string
		endpointId    int
		webhookType   int
		mockId        int64
		mockError     error
		expectedId    int
		expectedError bool
	}{
		{
			name:        "successful creation",
			resourceId:  "my-service",
			endpointId:  1,
			webhookType: 1,
			mockId:      42,
			expectedId:  42,
		},
		{
			name:          "API error",
			resourceId:    "bad-service",
			endpointId:    99,
			webhookType:   1,
			mockError:     errors.New("endpoint not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("CreateWebhook", tt.resourceId, int64(tt.endpointId), int64(tt.webhookType)).Return(tt.mockId, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			id, err := c.CreateWebhook(tt.resourceId, tt.endpointId, tt.webhookType)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Zero(t, id)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedId, id)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestDeleteWebhook verifies deletion of a webhook.
func TestDeleteWebhook(t *testing.T) {
	tests := []struct {
		name          string
		id            int
		mockError     error
		expectedError bool
	}{
		{
			name: "successful deletion",
			id:   42,
		},
		{
			name:          "API error",
			id:            99,
			mockError:     errors.New("webhook not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("DeleteWebhook", int64(tt.id)).Return(tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			err := c.DeleteWebhook(tt.id)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}
