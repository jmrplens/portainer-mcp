package toolutil

import (
	"errors"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

// fakeResponse mirrors the shape oapi-codegen gives all 442 generated response
// types: a StatusCode method and a GetBody method.
type fakeResponse struct {
	code int
	body []byte
}

func (f fakeResponse) StatusCode() int { return f.code }
func (f fakeResponse) GetBody() []byte { return f.body }

func TestCheck_Success_ReturnsNil(t *testing.T) {
	t.Parallel()
	if err := Check(fakeResponse{code: 200, body: []byte(`{"Id":1}`)}); err != nil {
		t.Errorf("Check(200) = %v, want nil", err)
	}
}

func TestCheck_NotFound_IsClassified(t *testing.T) {
	t.Parallel()
	body := []byte(`{"message":"Unable to find a tag","details":"Object not found inside the database"}`)
	err := Check(fakeResponse{code: 404, body: body})
	if !errors.Is(err, portainer.ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = false, want true; err = %v", err)
	}
}

func TestCheck_Unauthorized_IsClassified(t *testing.T) {
	t.Parallel()
	err := Check(fakeResponse{code: 401, body: []byte(`{"message":"Unauthorized"}`)})
	if !errors.Is(err, portainer.ErrUnauthorized) {
		t.Errorf("errors.Is(err, ErrUnauthorized) = false, want true; err = %v", err)
	}
}
