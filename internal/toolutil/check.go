// Package toolutil holds the contract every Portainer action declares and the
// helpers shared by all of them.
package toolutil

import (
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

// Response is the shape oapi-codegen gives every generated response type. All
// 442 of them satisfy it.
type Response interface {
	StatusCode() int
	GetBody() []byte
}

// Check turns a generated response into an error when Portainer refused it.
//
// Without this, every domain would reimplement status handling and the careful
// distinction ClassifyResponse draws — a JSON body means the resource is
// missing, plain text from Go's mux means the route does not exist in this
// version — would be lost the first time someone wrote `if resp.StatusCode() != 200`.
func Check(r Response) error {
	if err := portainer.ClassifyResponse(r.StatusCode(), r.GetBody()); err != nil {
		return fmt.Errorf("check response: %w", err)
	}
	return nil
}
