package mapdef

import (
	"fmt"
	"sort"
	"strings"

	"github.com/freebeamer/core/pkg/types"
)

// SharingReview is a structured, user-facing summary of what a converted
// .mapdef discloses and what is known about the right to redistribute it. It
// reports facts and the user's own assertions; it never verifies a licence
// claim or makes a legal judgement. See
// docs/definition-export-and-provenance-plan.md.
type SharingReview struct {
	// Severity is "ok" only when redistribution is asserted "permitted" and a
	// licence is recorded; otherwise "review-required".
	Severity string `json:"severity"`
	// Reasons lists why the severity is "review-required" (empty when "ok").
	Reasons []string `json:"reasons,omitempty"`

	Author         string `json:"author,omitempty"`
	License        string `json:"license,omitempty"`
	Source         string `json:"source,omitempty"`
	SourceSHA256   string `json:"sourceSha256,omitempty"`
	Redistribution string `json:"redistribution"` // normalized: permitted | not-permitted | unknown

	// EmbeddedSource describes a verbatim original source retained in
	// extensions.*.rawDocument, nil when none is present.
	EmbeddedSource *EmbeddedSource `json:"embeddedSource,omitempty"`
	// FreeTextFields names fields that may hold personal or vehicle data for
	// the user to review before sharing.
	FreeTextFields []string `json:"freeTextFields,omitempty"`
}

// EmbeddedSource records a retained verbatim source document.
type EmbeddedSource struct {
	// Format is the extension prefix, e.g. "xdf" or "a2l".
	Format string `json:"format"`
	// Bytes is the size of the retained source text.
	Bytes int `json:"bytes"`
}

const (
	redistributionPermitted    = "permitted"
	redistributionNotPermitted = "not-permitted"
	redistributionUnknown      = "unknown"
)

// ReviewForSharing inspects a document and reports what sharing it would
// disclose. It reads the document as-is and asserts nothing the document does
// not state. This is engineering diligence, not legal advice.
func ReviewForSharing(document *types.MapDefinition) SharingReview {
	review := SharingReview{
		Author:       document.Provenance.Author,
		License:      document.Provenance.License,
		Source:       document.Provenance.Source,
		SourceSHA256: document.Provenance.SourceSHA256,
	}

	switch document.Provenance.Redistribution {
	case redistributionPermitted, redistributionNotPermitted:
		review.Redistribution = document.Provenance.Redistribution
	default:
		review.Redistribution = redistributionUnknown
	}

	review.EmbeddedSource = embeddedSource(document.Extensions)
	review.FreeTextFields = freeTextFields(document)

	// Reasons are the blocking gaps only, so an empty Reasons means "ok". An
	// embedded verbatim source is reported through EmbeddedSource (and is a
	// strong caution the caller should surface) but does not by itself force
	// review-required, per the sharing plan.
	var reasons []string
	if review.Redistribution != redistributionPermitted {
		reasons = append(reasons, fmt.Sprintf("redistribution is not asserted as permitted (%s)", review.Redistribution))
	}
	if strings.TrimSpace(review.License) == "" {
		reasons = append(reasons, "no licence is recorded")
	}
	if len(reasons) == 0 {
		review.Severity = "ok"
	} else {
		review.Severity = "review-required"
	}
	review.Reasons = reasons
	return review
}

// embeddedSource finds a retained verbatim source in extensions under a
// "<format>.rawDocument" key (as xdf.Converter and a2l.Converter write it).
func embeddedSource(extensions map[string]interface{}) *EmbeddedSource {
	var best *EmbeddedSource
	for key, value := range extensions {
		if !strings.HasSuffix(key, ".rawDocument") {
			continue
		}
		text, ok := value.(string)
		if !ok || text == "" {
			continue
		}
		candidate := EmbeddedSource{Format: strings.TrimSuffix(key, ".rawDocument"), Bytes: len(text)}
		// Report the largest retained source when more than one is present.
		if best == nil || candidate.Bytes > best.Bytes {
			b := candidate
			best = &b
		}
	}
	return best
}

// freeTextFields lists the free-text locations that may carry personal or
// vehicle-identifying data, for the user to check before sharing. It reports
// where such data lives, not its contents.
func freeTextFields(document *types.MapDefinition) []string {
	fields := map[string]bool{}
	if strings.TrimSpace(document.Name) != "" {
		fields["name"] = true
	}
	if strings.TrimSpace(document.Description) != "" {
		fields["description"] = true
	}
	for i := range document.Parameters {
		if len(document.Parameters[i].SourceMetadata) > 0 {
			fields["parameters[].sourceMetadata"] = true
		}
		for _, axis := range document.Parameters[i].Axes {
			if len(axis.SourceMetadata) > 0 {
				fields["parameters[].axes[].sourceMetadata"] = true
			}
		}
	}
	out := make([]string, 0, len(fields))
	for field := range fields {
		out = append(out, field)
	}
	sort.Strings(out)
	return out
}

// StripSource returns a copy of document with every retained verbatim source
// (extensions.*.rawDocument) removed, its sourceSha256 kept, and a diagnostic
// recording the removal. The input is unchanged. Stripping does not make the
// result redistributable: the parameters can still derive from protected
// source material.
func StripSource(document *types.MapDefinition) (*types.MapDefinition, error) {
	clone, err := cloneDocument(document)
	if err != nil {
		return nil, fmt.Errorf("mapdef: strip source: %w", err)
	}
	removed := make([]string, 0)
	for key := range clone.Extensions {
		if strings.HasSuffix(key, ".rawDocument") {
			removed = append(removed, key)
		}
	}
	sort.Strings(removed)
	for _, key := range removed {
		delete(clone.Extensions, key)
	}
	if len(clone.Extensions) == 0 {
		clone.Extensions = nil
	}
	if len(removed) > 0 {
		clone.ImportDiagnostics = append(clone.ImportDiagnostics, types.ImportDiagnostic{
			Severity: "info",
			Code:     "mapdef.source-stripped",
			Message: fmt.Sprintf("Removed embedded verbatim source (%s). provenance.sourceSha256 is retained; the original remains recoverable only from the unstripped copy.",
				strings.Join(removed, ", ")),
		})
	}
	return clone, nil
}
