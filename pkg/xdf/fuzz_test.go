package xdf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func FuzzParse(f *testing.F) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "basic.xdf"))
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range [][]byte{
		fixture,
		[]byte(`<XDFFORMAT version="1.70"></XDFFORMAT>`),
		[]byte(`<XDFFORMAT><XDFTABLE><title>T</title></XDFTABLE></XDFFORMAT>`),
		[]byte(`<XDFFORMAT>`),
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
