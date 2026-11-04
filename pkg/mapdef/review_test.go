package mapdef

import (
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func baseDoc() *types.MapDefinition {
	return &types.MapDefinition{
		Format: Format, FormatVersion: CurrentVersion, ID: "d", Name: "Boost tune",
		Provenance: types.MapProvenance{Source: "car.xdf", SourceFormat: "xdf", SourceSHA256: "abc"},
		Memory:     types.MapMemory{BaseOffset: 0},
		Parameters: []types.MapParameter{},
		Extensions: map[string]interface{}{"xdf.rawDocument": "<XDFFORMAT>...</XDFFORMAT>"},
	}
}

func TestReviewConvertedDefaultsToReviewRequired(t *testing.T) {
	review := ReviewForSharing(baseDoc())
	if review.Severity != "review-required" {
		t.Fatalf("severity = %q", review.Severity)
	}
	if review.Redistribution != "unknown" {
		t.Fatalf("redistribution = %q, want unknown (absent)", review.Redistribution)
	}
	if review.EmbeddedSource == nil || review.EmbeddedSource.Format != "xdf" || review.EmbeddedSource.Bytes == 0 {
		t.Fatalf("embedded source = %#v", review.EmbeddedSource)
	}
	if !reflect.DeepEqual(review.FreeTextFields, []string{"name"}) {
		t.Fatalf("free-text fields = %#v", review.FreeTextFields)
	}
	wantReasons := []string{"redistribution is not asserted as permitted (unknown)", "no licence is recorded"}
	if !reflect.DeepEqual(review.Reasons, wantReasons) {
		t.Fatalf("reasons = %#v", review.Reasons)
	}
}

func TestReviewOkOnlyWhenPermittedAndLicensed(t *testing.T) {
	doc := baseDoc()
	doc.Provenance.Redistribution = "permitted"
	doc.Provenance.License = "MIT"
	delete(doc.Extensions, "xdf.rawDocument") // irrelevant to severity, but tidy
	review := ReviewForSharing(doc)
	if review.Severity != "ok" || len(review.Reasons) != 0 {
		t.Fatalf("severity=%q reasons=%#v, want ok/none", review.Severity, review.Reasons)
	}
}

func TestReviewEmbeddedSourceDoesNotBlockButIsReported(t *testing.T) {
	doc := baseDoc()
	doc.Provenance.Redistribution = "permitted"
	doc.Provenance.License = "CC-BY-4.0"
	review := ReviewForSharing(doc)
	if review.Severity != "ok" {
		t.Fatalf("severity = %q; embedded source must not force review-required", review.Severity)
	}
	if review.EmbeddedSource == nil {
		t.Fatal("embedded source must still be reported")
	}
}

func TestReviewNormalizesUnknownRedistribution(t *testing.T) {
	doc := baseDoc()
	doc.Provenance.Redistribution = "maybe" // not a valid enum value
	if got := ReviewForSharing(doc).Redistribution; got != "unknown" {
		t.Fatalf("redistribution = %q, want unknown", got)
	}
	doc.Provenance.Redistribution = "not-permitted"
	if got := ReviewForSharing(doc).Redistribution; got != "not-permitted" {
		t.Fatalf("redistribution = %q", got)
	}
}

func TestStripSourceRemovesRawDocumentAndKeepsDigest(t *testing.T) {
	doc := baseDoc()
	doc.Extensions["a2l.rawDocument"] = "/begin PROJECT"
	doc.Extensions["xdf.attributes"] = map[string]interface{}{"keep": "me"}
	stripped, err := StripSource(doc)
	if err != nil {
		t.Fatal(err)
	}
	// Input is unchanged.
	if _, ok := doc.Extensions["xdf.rawDocument"]; !ok {
		t.Fatal("StripSource mutated its input")
	}
	if _, ok := stripped.Extensions["xdf.rawDocument"]; ok {
		t.Fatal("xdf.rawDocument not removed")
	}
	if _, ok := stripped.Extensions["a2l.rawDocument"]; ok {
		t.Fatal("a2l.rawDocument not removed")
	}
	if _, ok := stripped.Extensions["xdf.attributes"]; !ok {
		t.Fatal("non-rawDocument extension must be kept")
	}
	if stripped.Provenance.SourceSHA256 != "abc" {
		t.Fatalf("sourceSha256 lost: %q", stripped.Provenance.SourceSHA256)
	}
	if ReviewForSharing(stripped).EmbeddedSource != nil {
		t.Fatal("stripped doc must report no embedded source")
	}
	var found bool
	for _, d := range stripped.ImportDiagnostics {
		if d.Code == "mapdef.source-stripped" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing source-stripped diagnostic")
	}
}

func TestStripSourceNoEmbeddedSourceIsNoOpDiagnostic(t *testing.T) {
	doc := baseDoc()
	delete(doc.Extensions, "xdf.rawDocument")
	stripped, err := StripSource(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range stripped.ImportDiagnostics {
		if d.Code == "mapdef.source-stripped" {
			t.Fatal("no diagnostic expected when nothing was stripped")
		}
	}
}
