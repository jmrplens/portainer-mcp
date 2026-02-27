// Package tooldef handles loading and parsing MCP tool definitions from YAML.
// It supports both embedded and external tool definition files with version
// validation to ensure compatibility between the server and its tool definitions.
package tooldef

import (
	_ "embed"
	"os"
)

// ToolsFile contains the embedded contents of the tools.yaml definition file.
//
//go:embed tools.yaml
var ToolsFile []byte

// CreateToolsFileIfNotExists creates the tools.yaml file if it doesn't exist
// It returns true if the file already exists, false if it was created or an error occurred
func CreateToolsFileIfNotExists(path string) (bool, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.WriteFile(path, ToolsFile, 0644)
		if err != nil {
			return false, err
		}
		return false, nil
	}
	return true, nil
}
