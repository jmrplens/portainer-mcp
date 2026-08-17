package portainer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
)

// MultipartForm builds a multipart/form-data request body for the handful of
// Portainer routes that accept nothing else.
//
// It lives here, beside the client, rather than in any one domain package,
// because several unrelated domains need the identical thing: custom
// templates upload a stack file, stacks upload two (create and update from a
// file), and endpoints upload TLS material. Nothing in this type knows what
// any of those fields mean — it takes names and values and renders them.
//
// Three properties are the whole reason it exists rather than each caller
// reaching for mime/multipart directly, and each is the kind of mistake that
// produces a server-side error naming the wrong cause:
//
//   - The content type must carry the boundary this writer generated.
//     mime/multipart derives a fresh boundary per writer and every delimiter
//     in the body is built from it, so a caller that sends a bare
//     "multipart/form-data" hands the server a body it cannot split at all.
//     Build returns the content type alongside the body precisely so the two
//     cannot be separated.
//   - A field the caller left unset must emit no part. An empty part is a
//     present field with an empty value, which is a different request: an
//     empty EdgeSettings is JSON that fails to parse, not an EdgeSettings
//     the caller declined to send. The Optional* methods take a pointer and
//     a nil one writes nothing, while their non-pointer counterparts always
//     write — including an empty string, which is a caller's explicit "".
//   - A file part must carry a filename. Go's own multipart reader — which
//     is what Portainer parses these bodies with — files a part under
//     Form.File only when its Content-Disposition names one, so a file
//     written as a plain text part arrives as no file at all and the server
//     reports the field missing while its bytes sit in the request.
//
// The body is assembled in memory. Every payload these routes take is a
// stack file, a manifest or a certificate — kilobytes — and buffering buys
// something net/http can observe: http.NewRequest type-switches on the
// reader it is handed, and for a *bytes.Reader it sets ContentLength and
// installs a GetBody closure that rewinds and replays. Wrap the identical
// bytes in anything opaque — an io.Pipe, an io.MultiReader — and the upload
// goes out chunked with GetBody nil, so a redirect or a retry after a
// half-closed connection fails instead of being resent. Uploads are the
// requests most worth replaying, which is why this is enforced by a test
// (TestUnit_MultipartFormBody_IsReplayableByNetHTTP) and not merely stated
// here: every other test in this package reads the body once and cannot
// tell the two apart.
//
// Errors are accumulated rather than returned per call, so a caller can
// declare a whole form as a run of statements and check once at Build. A
// form is single-use: Build closes the underlying writer to emit the
// trailing boundary, and a second call is refused rather than handed a body
// with a duplicated terminator.
type MultipartForm struct {
	buf    bytes.Buffer
	writer *multipart.Writer
	err    error
	built  bool
}

// NewMultipartForm returns an empty form with its own generated boundary.
func NewMultipartForm() *MultipartForm {
	form := &MultipartForm{}
	form.writer = multipart.NewWriter(&form.buf)
	return form
}

// Field writes a text part unconditionally, empty value included. Use it for
// a field the route lists as required, where an empty string is the caller's
// own value rather than an absence.
func (f *MultipartForm) Field(name, value string) {
	if f.err != nil {
		return
	}
	if f.built {
		f.err = errors.New("multipart: the form was already built")
		return
	}
	if name == "" {
		f.err = errors.New("multipart: a field name is required")
		return
	}
	if err := f.writer.WriteField(name, value); err != nil {
		f.err = fmt.Errorf("multipart: write field %q: %w", name, err)
	}
}

// OptionalField writes a text part only when value is non-nil. A nil value
// emits no part; a pointer to "" emits an empty one, which is a different
// request and a deliberate one.
func (f *MultipartForm) OptionalField(name string, value *string) {
	if value == nil {
		return
	}
	f.Field(name, *value)
}

// IntField writes an integer as its decimal rendering. Multipart carries no
// types: the server parses the text back, so 2 must arrive as "2" and not as
// JSON's 2 or as a padded "02".
func (f *MultipartForm) IntField(name string, value int) {
	f.Field(name, strconv.Itoa(value))
}

// OptionalIntField writes an integer part only when value is non-nil. A
// pointer to 0 emits "0": zero is a value here, not an absence.
func (f *MultipartForm) OptionalIntField(name string, value *int) {
	if value == nil {
		return
	}
	f.IntField(name, *value)
}

// BoolField writes a boolean as "true" or "false", which is what Go's own
// strconv.ParseBool on the server side reads back.
func (f *MultipartForm) BoolField(name string, value bool) {
	f.Field(name, strconv.FormatBool(value))
}

// OptionalBoolField writes a boolean part only when value is non-nil. A
// pointer to false emits "false", which is not the same as omitting the
// field: for a flag whose server-side default is true, the two differ.
func (f *MultipartForm) OptionalBoolField(name string, value *bool) {
	if value == nil {
		return
	}
	f.BoolField(name, *value)
}

// File writes a file part under the given field name, carrying filename in
// its Content-Disposition.
//
// filename must not be empty. Whether any given Portainer route reads the
// name is that route's business — this type does not know and does not
// guess — but Go's multipart reader decides whether the part is a file at
// all on the presence of this header, and Portainer parses these bodies
// with it. Refusing here keeps the diagnosis at the mistake rather than at a
// server answering that a field the caller plainly sent is missing.
func (f *MultipartForm) File(name, filename string, content []byte) {
	if f.err != nil {
		return
	}
	if f.built {
		f.err = errors.New("multipart: the form was already built")
		return
	}
	if name == "" {
		f.err = errors.New("multipart: a field name is required")
		return
	}
	if filename == "" {
		f.err = fmt.Errorf("multipart: file field %q needs a filename, or the server reads the part as text and reports the field missing", name)
		return
	}
	part, err := f.writer.CreateFormFile(name, filename)
	if err != nil {
		f.err = fmt.Errorf("multipart: create file part %q: %w", name, err)
		return
	}
	if _, err := part.Write(content); err != nil {
		f.err = fmt.Errorf("multipart: write file part %q: %w", name, err)
	}
}

// Build closes the form and returns its body together with the content type
// the request must carry, boundary included.
//
// The two are returned together and never separately: the boundary is
// generated per form, so a content type obtained anywhere else describes a
// different body.
//
// The reader is a *bytes.Reader, and that concrete choice is load-bearing
// rather than incidental — see this type's own doc comment for what
// net/http derives from it, and for the test that holds it in place.
func (f *MultipartForm) Build() (io.Reader, string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	if f.built {
		return nil, "", errors.New("multipart: the form was already built; build a new one per request")
	}
	f.built = true
	if err := f.writer.Close(); err != nil {
		return nil, "", fmt.Errorf("multipart: close body: %w", err)
	}
	return bytes.NewReader(f.buf.Bytes()), f.writer.FormDataContentType(), nil
}
