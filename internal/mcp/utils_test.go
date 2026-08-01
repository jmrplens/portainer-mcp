package mcp

import (
	"reflect"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

// TestParseAccessMap verifies parse access map behavior.
func TestParseAccessMap(t *testing.T) {
	tests := []struct {
		name    string
		entries []any
		want    map[int]string
		wantErr bool
	}{
		{
			name: "Valid single entry",
			entries: []any{
				map[string]any{
					"id":     float64(1),
					"access": AccessLevelEnvironmentAdmin,
				},
			},
			want: map[int]string{
				1: AccessLevelEnvironmentAdmin,
			},
			wantErr: false,
		},
		{
			name: "Valid multiple entries",
			entries: []any{
				map[string]any{
					"id":     float64(1),
					"access": AccessLevelEnvironmentAdmin,
				},
				map[string]any{
					"id":     float64(2),
					"access": AccessLevelReadonlyUser,
				},
			},
			want: map[int]string{
				1: AccessLevelEnvironmentAdmin,
				2: AccessLevelReadonlyUser,
			},
			wantErr: false,
		},
		{
			name: "Invalid entry type",
			entries: []any{
				"not a map",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid ID type",
			entries: []any{
				map[string]any{
					"id":     "string-id",
					"access": AccessLevelEnvironmentAdmin,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid access type",
			entries: []any{
				map[string]any{
					"id":     float64(1),
					"access": 123,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid access level",
			entries: []any{
				map[string]any{
					"id":     float64(1),
					"access": "invalid_access_level",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Empty entries",
			entries: []any{},
			want:    map[int]string{},
			wantErr: false,
		},
		{
			name: "Missing ID field",
			entries: []any{
				map[string]any{
					"access": AccessLevelEnvironmentAdmin,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Missing access field",
			entries: []any{
				map[string]any{
					"id": float64(1),
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAccessMap(tt.entries)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAccessMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAccessMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsValidHTTPMethod verifies is valid h t t p method behavior.
func TestIsValidHTTPMethod(t *testing.T) {
	tests := []struct {
		name   string
		method string
		expect bool
	}{
		{"Valid GET", "GET", true},
		{"Valid POST", "POST", true},
		{"Valid PUT", "PUT", true},
		{"Valid DELETE", "DELETE", true},
		{"Valid HEAD", "HEAD", true},
		{"Invalid lowercase get", "get", false},
		{"Valid PATCH", "PATCH", true},
		{"Invalid OPTIONS", "OPTIONS", false},
		{"Invalid Empty", "", false},
		{"Invalid Random", "RANDOM", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidHTTPMethod(tt.method)
			if got != tt.expect {
				t.Errorf("isValidHTTPMethod(%q) = %v, want %v", tt.method, got, tt.expect)
			}
		})
	}
}

// TestParseKeyValueMap verifies parse key value map behavior.
func TestParseKeyValueMap(t *testing.T) {
	tests := []struct {
		name    string
		items   []any
		want    map[string]string
		wantErr bool
	}{
		{
			name: "Valid single entry",
			items: []any{
				map[string]any{"key": "k1", "value": "v1"},
			},
			want: map[string]string{
				"k1": "v1",
			},
			wantErr: false,
		},
		{
			name: "Valid multiple entries",
			items: []any{
				map[string]any{"key": "k1", "value": "v1"},
				map[string]any{"key": "k2", "value": "v2"},
			},
			want: map[string]string{
				"k1": "v1",
				"k2": "v2",
			},
			wantErr: false,
		},
		{
			name:    "Empty items",
			items:   []any{},
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name: "Invalid item type",
			items: []any{
				"not a map",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid key type",
			items: []any{
				map[string]any{"key": 123, "value": "v1"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid value type",
			items: []any{
				map[string]any{"key": "k1", "value": 123},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Missing key field",
			items: []any{
				map[string]any{"value": "v1"},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Missing value field",
			items: []any{
				map[string]any{"key": "k1"},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseKeyValueMap(tt.items)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseKeyValueMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseKeyValueMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidateName verifies validateName behavior for the uncovered branches.
func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid name",
			input:   "my-stack",
			wantErr: false,
		},
		{
			name:    "empty string returns error",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace-only string returns error",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "tab-only string returns error",
			input:   "\t",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateURL verifies validateURL behavior for the uncovered branches.
func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid http URL",
			input:   "http://example.com",
			wantErr: false,
		},
		{
			name:    "valid https URL",
			input:   "https://example.com",
			wantErr: false,
		},
		{
			name:    "valid oci URL",
			input:   "oci://registry.example.com/image",
			wantErr: false,
		},
		{
			name:    "wrong scheme returns error",
			input:   "ftp://foo.example.com",
			wantErr: true,
		},
		{
			name:    "missing host returns error",
			input:   "http://",
			wantErr: true,
		},
		{
			name:    "empty scheme returns error",
			input:   "example.com",
			wantErr: true,
		},
		{
			name:    "invalid escape sequence returns parse error",
			input:   "http://example.com/%zz",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateComposeYAML verifies validateComposeYAML behavior for the uncovered branches.
func TestValidateComposeYAML(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "valid YAML returns no error",
			content: "version: \"3\"\nservices:\n  web:\n    image: nginx\n",
			wantErr: false,
		},
		{
			name:    "empty content returns error",
			content: "",
			wantErr: true,
		},
		{
			name:    "whitespace-only content returns error",
			content: "   \n\t  ",
			wantErr: true,
		},
		{
			name:    "invalid YAML syntax returns error",
			content: "key: [unclosed bracket",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateComposeYAML(tt.content)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestJsonResult verifies jsonResult behavior for the uncovered marshal-failure branch.
func TestJsonResult(t *testing.T) {
	t.Run("marshal success returns text result", func(t *testing.T) {
		result, err := jsonResult(map[string]string{"key": "value"}, "marshal error")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsError)
	})

	t.Run("marshal failure returns tool error result", func(t *testing.T) {
		// channels cannot be marshalled to JSON — triggers the error branch
		result, err := jsonResult(make(chan int), "marshal failed")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.IsError)
	})
}

// TestParseProxyParams verifies parseProxyParams behavior for the path traversal branch.
func TestParseProxyParams(t *testing.T) {
	t.Run("zero environmentId returns tool error", func(t *testing.T) {
		request := CreateMCPRequest(map[string]any{
			"environmentId": float64(0),
			"method":        "GET",
			"dockerAPIPath": "/containers/json",
		})

		params, toolErr := parseProxyParams(request, "dockerAPIPath")
		assert.Nil(t, params)
		assert.NotNil(t, toolErr)
		assert.True(t, toolErr.IsError)
	})

	t.Run("negative environmentId returns tool error", func(t *testing.T) {
		request := CreateMCPRequest(map[string]any{
			"environmentId": float64(-1),
			"method":        "GET",
			"dockerAPIPath": "/containers/json",
		})

		params, toolErr := parseProxyParams(request, "dockerAPIPath")
		assert.Nil(t, params)
		assert.NotNil(t, toolErr)
		assert.True(t, toolErr.IsError)
	})

	t.Run("path traversal in API path returns tool error", func(t *testing.T) {
		request := CreateMCPRequest(map[string]any{
			"environmentId": float64(1),
			"method":        "GET",
			"dockerAPIPath": "/containers/../secrets",
		})

		params, toolErr := parseProxyParams(request, "dockerAPIPath")
		assert.Nil(t, params)
		assert.NotNil(t, toolErr)
		assert.True(t, toolErr.IsError)
	})

	t.Run("double-encoded path traversal returns tool error", func(t *testing.T) {
		request := CreateMCPRequest(map[string]any{
			"environmentId": float64(1),
			"method":        "GET",
			"dockerAPIPath": "/containers/%252e%252e/secrets",
		})

		params, toolErr := parseProxyParams(request, "dockerAPIPath")
		assert.Nil(t, params)
		assert.NotNil(t, toolErr)
		assert.True(t, toolErr.IsError)
	})

	t.Run("API path without leading slash returns tool error", func(t *testing.T) {
		request := CreateMCPRequest(map[string]any{
			"environmentId": float64(1),
			"method":        "GET",
			"dockerAPIPath": "containers/json",
		})

		params, toolErr := parseProxyParams(request, "dockerAPIPath")
		assert.Nil(t, params)
		assert.NotNil(t, toolErr)
		assert.True(t, toolErr.IsError)
		textContent, ok := toolErr.Content[0].(mcpgo.TextContent)
		assert.True(t, ok)
		assert.Contains(t, textContent.Text, "must start with a leading slash")
	})

	t.Run("valid params return populated struct", func(t *testing.T) {
		request := CreateMCPRequest(map[string]any{
			"environmentId": float64(42),
			"method":        "GET",
			"dockerAPIPath": "/containers/json",
		})

		params, toolErr := parseProxyParams(request, "dockerAPIPath")
		assert.Nil(t, toolErr)
		assert.NotNil(t, params)
		assert.Equal(t, 42, params.environmentID)
		assert.Equal(t, "GET", params.method)
		assert.Equal(t, "/containers/json", params.apiPath)
	})
}
