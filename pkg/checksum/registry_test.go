package checksum

import (
	"bytes"
	"errors"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

type fakeProvider struct {
	id         string
	confidence types.ChecksumDetectionConfidence
	verify     []types.ChecksumResult
	correct    func([]byte)
	verifyErr  error
	correctErr error
}

func (f *fakeProvider) ID() string { return f.id }
func (f *fakeProvider) Detect(image []byte) types.ChecksumDetection {
	if len(image) > 0 {
		image[0] ^= 0xff
	}
	return types.ChecksumDetection{Confidence: f.confidence, Reason: "test"}
}
func (f *fakeProvider) Verify(image []byte) ([]types.ChecksumResult, error) {
	return append([]types.ChecksumResult(nil), f.verify...), f.verifyErr
}
func (f *fakeProvider) Correct(image []byte) ([]types.ChecksumResult, error) {
	if f.correct != nil {
		f.correct(image)
	}
	return append([]types.ChecksumResult(nil), f.verify...), f.correctErr
}

func registryFor(t *testing.T, providers ...Provider) *Registry {
	t.Helper()
	registry, err := NewRegistry(providers...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestSelectHighestUniqueConfidence(t *testing.T) {
	low := &fakeProvider{id: "low", confidence: types.ChecksumDetectionLow}
	high := &fakeProvider{id: "high", confidence: types.ChecksumDetectionHigh}
	image := []byte{1}
	provider, detection, err := registryFor(t, low, high).Select(image)
	if err != nil {
		t.Fatal(err)
	}
	if provider.ID() != "high" || detection.Confidence != types.ChecksumDetectionHigh {
		t.Fatalf("selection=%s,%#v", provider.ID(), detection)
	}
	if image[0] != 1 {
		t.Fatal("Detect mutated caller image")
	}
}

func TestSelectRefusesZeroAndEqualMatches(t *testing.T) {
	if _, _, err := registryFor(t).Select([]byte{1}); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("empty error=%v", err)
	}
	a := &fakeProvider{id: "a", confidence: types.ChecksumDetectionMedium}
	b := &fakeProvider{id: "b", confidence: types.ChecksumDetectionMedium}
	if _, _, err := registryFor(t, a, b).Select([]byte{1}); !errors.Is(err, ErrAmbiguousProvider) {
		t.Fatalf("tie error=%v", err)
	}
}

func TestRegistryRejectsInvalidAndDuplicateProviders(t *testing.T) {
	registry, _ := NewRegistry()
	if err := registry.Register(nil); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("nil error=%v", err)
	}
	provider := &fakeProvider{id: "same"}
	if err := registry.Register(provider); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(provider); !errors.Is(err, ErrDuplicateProvider) {
		t.Fatalf("duplicate error=%v", err)
	}
}

func TestVerifyRequiresResults(t *testing.T) {
	provider := &fakeProvider{id: "test", confidence: types.ChecksumDetectionHigh}
	if _, _, err := registryFor(t, provider).Verify([]byte{1}); !errors.Is(err, ErrNoResults) {
		t.Fatalf("error=%v", err)
	}
}

func TestCorrectWorksOnCopyAndPostVerifies(t *testing.T) {
	valid := types.ChecksumResult{Name: "block", Valid: true}
	provider := &fakeProvider{id: "test", confidence: types.ChecksumDetectionHigh, verify: []types.ChecksumResult{valid}, correct: func(image []byte) { image[0] = 9 }}
	original := []byte{1, 2}
	id, corrected, results, err := registryFor(t, provider).CorrectAndVerify(original)
	if err != nil {
		t.Fatal(err)
	}
	if id != "test" || !bytes.Equal(corrected, []byte{9, 2}) || !AllValid(results) {
		t.Fatalf("result=%s,%v,%v", id, corrected, results)
	}
	if !bytes.Equal(original, []byte{1, 2}) {
		t.Fatal("original mutated")
	}
}

func TestCorrectWithholdsBytesAfterFailedVerify(t *testing.T) {
	invalid := types.ChecksumResult{Name: "block", Valid: false}
	provider := &fakeProvider{id: "test", confidence: types.ChecksumDetectionHigh, verify: []types.ChecksumResult{invalid}, correct: func(image []byte) { image[0] = 9 }}
	_, corrected, _, err := registryFor(t, provider).CorrectAndVerify([]byte{1})
	if !errors.Is(err, ErrPostCorrectionVerify) {
		t.Fatalf("error=%v", err)
	}
	if corrected != nil {
		t.Fatalf("corrected bytes leaked: %v", corrected)
	}
}

func TestAllValidRejectsEmptyAndInvalid(t *testing.T) {
	if AllValid(nil) {
		t.Fatal("empty results valid")
	}
	if AllValid([]types.ChecksumResult{{Valid: true}, {Valid: false}}) {
		t.Fatal("invalid result accepted")
	}
	if !AllValid([]types.ChecksumResult{{Valid: true}}) {
		t.Fatal("valid result rejected")
	}
}
