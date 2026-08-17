package portainer

import (
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"
)

// readBack renders a form and parses the result the way net/http's own
// server does, so every assertion below is made against what a server would
// actually see rather than against the bytes this package chose to emit.
//
// multipart.Reader.ReadForm is the same call http.Request.ParseMultipartForm
// makes, and it is what enforces the distinction this file exists to pin: a
// part is filed under Form.File only when its Content-Disposition carries a
// filename, and under Form.Value otherwise.
func readBack(t *testing.T, form *MultipartForm) *multipart.Form {
	t.Helper()
	body, contentType, err := form.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type %q: %v", contentType, err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("media type = %q, want multipart/form-data", mediaType)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatalf("content type %q carries no boundary; a server cannot split the body without one", contentType)
	}
	parsed, err := multipart.NewReader(body, boundary).ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("ReadForm with the boundary from %q: %v", contentType, err)
	}
	t.Cleanup(func() { _ = parsed.RemoveAll() })
	return parsed
}

// TestUnit_MultipartFormContentType_CarriesTheGeneratedBoundary pins the
// first thing a naive implementation gets wrong: returning a bare
// "multipart/form-data" as the content type. Every part delimiter in the
// body is derived from a boundary mime/multipart generates per writer, and a
// server given the media type without it has no way to split the body at
// all — it answers 400 with a message about a missing boundary, never about
// the field the caller actually cares about.
//
// Asserted as "the boundary in the header is the one the body uses", not
// merely as "the header contains the word boundary": a hard-coded boundary
// string would satisfy the weaker check and still corrupt every request
// whose payload happened to contain it.
func TestUnit_MultipartFormContentType_CarriesTheGeneratedBoundary(t *testing.T) {
	t.Parallel()

	form := NewMultipartForm()
	form.Field("Title", "nginx")
	body, contentType, err := form.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type %q: %v", contentType, err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatalf("content type = %q, want a boundary parameter", contentType)
	}

	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(raw), "--"+boundary) {
		t.Errorf("the body does not use the boundary the content type advertises (%q); body:\n%s", boundary, raw)
	}

	// Two forms must not share a boundary: mime/multipart derives one per
	// writer, and a package-level constant would tie every concurrent
	// request to the same delimiter.
	_, otherContentType, err := NewMultipartForm().Build()
	if err != nil {
		t.Fatalf("Build() on a second form: error = %v", err)
	}
	if otherContentType == contentType {
		t.Errorf("two forms share the content type %q, so the boundary is not generated per form", contentType)
	}
}

// TestUnit_MultipartFormUnsetField_EmitsNoPart pins the second thing a naive
// implementation gets wrong: writing an empty part for a field the caller
// left unset. Portainer reads these bodies with Go's own multipart parser
// and then checks presence, so an empty part is not the same as an absent
// one — an empty EdgeSettings part, for instance, is a present field whose
// JSON fails to parse, not a field the caller declined to send.
func TestUnit_MultipartFormUnsetField_EmitsNoPart(t *testing.T) {
	t.Parallel()

	set := "sent"
	tests := []struct {
		name    string
		add     func(*MultipartForm)
		want    string
		present bool
	}{
		{
			name:    "unset string emits nothing",
			add:     func(f *MultipartForm) { f.OptionalField("Logo", nil) },
			present: false,
		},
		{
			name:    "set string emits its value",
			add:     func(f *MultipartForm) { f.OptionalField("Logo", &set) },
			want:    "sent",
			present: true,
		},
		{
			name:    "set empty string still emits a part",
			add:     func(f *MultipartForm) { f.OptionalField("Logo", new(string)) },
			want:    "",
			present: true,
		},
		{
			name:    "unset int emits nothing",
			add:     func(f *MultipartForm) { f.OptionalIntField("Logo", nil) },
			present: false,
		},
		{
			name:    "set int emits its decimal rendering",
			add:     func(f *MultipartForm) { n := 2; f.OptionalIntField("Logo", &n) },
			want:    "2",
			present: true,
		},
		{
			name:    "set zero int still emits a part",
			add:     func(f *MultipartForm) { f.OptionalIntField("Logo", new(int)) },
			want:    "0",
			present: true,
		},
		{
			name:    "unset bool emits nothing",
			add:     func(f *MultipartForm) { f.OptionalBoolField("Logo", nil) },
			present: false,
		},
		{
			name:    "set false bool still emits a part",
			add:     func(f *MultipartForm) { f.OptionalBoolField("Logo", new(bool)) },
			want:    "false",
			present: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			form := NewMultipartForm()
			// A second, always-present field so the "absent" cases cannot
			// pass by the whole body being empty.
			form.Field("Title", "nginx")
			tt.add(form)

			parsed := readBack(t, form)
			values, ok := parsed.Value["Logo"]
			if !tt.present {
				if ok {
					t.Errorf("Logo was left unset but the body carries it as %q; an unset field must emit no part", values)
				}
				if parsed.Value["Title"][0] != "nginx" {
					t.Errorf("Title = %q, want nginx", parsed.Value["Title"])
				}
				return
			}
			if !ok {
				t.Fatalf("Logo was set but the body carries no part for it; parts present: %v", parsed.Value)
			}
			if len(values) != 1 || values[0] != tt.want {
				t.Errorf("Logo = %q, want [%q]", values, tt.want)
			}
		})
	}
}

// TestUnit_MultipartFormFilePart_CarriesAFilename pins the third thing a
// naive implementation gets wrong: writing the file as an ordinary text
// part. Go's multipart reader files a part under Form.File only when its
// Content-Disposition carries a filename, so a file part written without one
// reaches Portainer's own r.FormFile("File") as nothing at all — the upload
// fails with the field reported missing even though its bytes were sent.
func TestUnit_MultipartFormFilePart_CarriesAFilename(t *testing.T) {
	t.Parallel()

	form := NewMultipartForm()
	form.Field("Title", "nginx")
	form.File("File", "template.yml", []byte("services:\n  web:\n    image: nginx\n"))

	parsed := readBack(t, form)

	if _, isText := parsed.Value["File"]; isText {
		t.Error("File arrived as a text part; without a filename a server's FormFile lookup finds nothing")
	}
	headers, ok := parsed.File["File"]
	if !ok || len(headers) != 1 {
		t.Fatalf("File is not a file part; file parts present: %v", parsed.File)
	}
	if headers[0].Filename != "template.yml" {
		t.Errorf("File filename = %q, want template.yml", headers[0].Filename)
	}

	opened, err := headers[0].Open()
	if err != nil {
		t.Fatalf("open the file part: %v", err)
	}
	defer func() { _ = opened.Close() }()
	content, err := io.ReadAll(opened)
	if err != nil {
		t.Fatalf("read the file part: %v", err)
	}
	if got, want := string(content), "services:\n  web:\n    image: nginx\n"; got != want {
		t.Errorf("File content = %q, want %q", got, want)
	}
}

// TestUnit_MultipartFormWithoutAFilename_FailsTheBuild pins that the writer
// refuses rather than silently emitting a text part, which is the failure
// TestUnit_MultipartFormFilePart_CarriesAFilename describes: a server
// reports the field missing, so the caller learns nothing about the real
// cause. Failing at Build keeps the diagnosis where the mistake is.
func TestUnit_MultipartFormWithoutAFilename_FailsTheBuild(t *testing.T) {
	t.Parallel()

	form := NewMultipartForm()
	form.File("File", "", []byte("services: {}"))
	if _, _, err := form.Build(); err == nil {
		t.Fatal("Build() error = nil, want a refusal for a file part with no filename")
	}
}

// TestUnit_MultipartFormBuiltTwice_RefusesTheSecondCall pins that a form is
// single-use. Build closes the underlying writer to emit the trailing
// boundary; a second call would otherwise hand back a body whose terminator
// is duplicated or, worse, silently append to a closed writer.
func TestUnit_MultipartFormBuiltTwice_RefusesTheSecondCall(t *testing.T) {
	t.Parallel()

	form := NewMultipartForm()
	form.Field("Title", "nginx")
	if _, _, err := form.Build(); err != nil {
		t.Fatalf("first Build() error = %v", err)
	}
	if _, _, err := form.Build(); err == nil {
		t.Fatal("second Build() error = nil, want a refusal")
	}
}

// TestUnit_MultipartFormWithEveryFieldKind_RoundTripsThroughAParser is the
// end-to-end shape three more domains will rely on: required text, rendered
// numbers and booleans, an omitted optional and a file, all in one body.
func TestUnit_MultipartFormWithEveryFieldKind_RoundTripsThroughAParser(t *testing.T) {
	t.Parallel()

	edge := true
	form := NewMultipartForm()
	form.Field("Title", "nginx")
	form.IntField("Type", 2)
	form.BoolField("Swarm", false)
	form.OptionalBoolField("EdgeTemplate", &edge)
	form.OptionalField("Logo", nil)
	form.File("File", "template.yml", []byte("services: {}"))

	parsed := readBack(t, form)

	want := map[string]string{
		"Title":        "nginx",
		"Type":         "2",
		"Swarm":        "false",
		"EdgeTemplate": "true",
	}
	for name, value := range want {
		got, ok := parsed.Value[name]
		if !ok {
			t.Errorf("%s is absent; parts present: %v", name, parsed.Value)
			continue
		}
		if got[0] != value {
			t.Errorf("%s = %q, want %q", name, got[0], value)
		}
	}
	if _, ok := parsed.Value["Logo"]; ok {
		t.Error("Logo was omitted but a part was emitted for it")
	}
	if _, ok := parsed.File["File"]; !ok {
		t.Errorf("File is not a file part; file parts present: %v", parsed.File)
	}
}
