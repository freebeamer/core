package a2l

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func FuzzParse(f *testing.F) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "basic.a2l"))
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range [][]byte{
		fixture,
		[]byte(`/begin PROJECT P ""`),
		[]byte(`/begin PROJECT P "" /begin MODULE M "" /end MODULE /end PROJECT`),
		[]byte(`/begin PROJECT P "" /begin MODULE M "" /begin A2ML block "x" { }; /end A2ML /end MODULE /end PROJECT`),
		[]byte(`/* unterminated`),
		[]byte(`"unterminated`),
		[]byte(`/end`),
		[]byte(`/begin`),
		nil,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		definition, err := Parse(bytes.NewReader(input))
		if err == nil && definition == nil {
			t.Fatal("Parse returned a nil definition without an error")
		}
	})
}
