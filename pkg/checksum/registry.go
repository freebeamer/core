package checksum

import (
	"errors"
	"fmt"

	"github.com/freebeamer/core/pkg/types"
)

var (
	ErrProviderNotFound     = errors.New("checksum provider not found for this firmware")
	ErrAmbiguousProvider    = errors.New("multiple checksum providers matched equally")
	ErrDuplicateProvider    = errors.New("duplicate checksum provider ID")
	ErrInvalidProvider      = errors.New("invalid checksum provider")
	ErrNoResults            = errors.New("checksum provider returned no integrity results")
	ErrPostCorrectionVerify = errors.New("checksum correction failed post-correction verification")
)

type Registry struct{ providers []Provider }

func NewRegistry(providers ...Provider) (*Registry, error) {
	registry := &Registry{}
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *Registry) Register(provider Provider) error {
	if provider == nil || provider.ID() == "" {
		return ErrInvalidProvider
	}
	for _, existing := range r.providers {
		if existing.ID() == provider.ID() {
			return fmt.Errorf("%w: %s", ErrDuplicateProvider, provider.ID())
		}
	}
	r.providers = append(r.providers, provider)
	return nil
}

func (r *Registry) Select(image []byte) (Provider, types.ChecksumDetection, error) {
	var selected Provider
	var best types.ChecksumDetection
	ties := 0
	for _, provider := range r.providers {
		detection := provider.Detect(clone(image))
		if detection.Confidence > types.ChecksumDetectionHigh {
			return nil, types.ChecksumDetection{}, fmt.Errorf("provider %q: invalid detection confidence %d", provider.ID(), detection.Confidence)
		}
		if detection.Confidence == types.ChecksumDetectionNone {
			continue
		}
		if selected == nil || detection.Confidence > best.Confidence {
			selected, best, ties = provider, detection, 1
		} else if detection.Confidence == best.Confidence {
			ties++
		}
	}
	if selected == nil {
		return nil, types.ChecksumDetection{}, ErrProviderNotFound
	}
	if ties > 1 {
		return nil, types.ChecksumDetection{}, fmt.Errorf("%w at confidence %d", ErrAmbiguousProvider, best.Confidence)
	}
	return selected, best, nil
}

func (r *Registry) Verify(image []byte) (string, []types.ChecksumResult, error) {
	provider, _, err := r.Select(image)
	if err != nil {
		return "", nil, err
	}
	results, err := provider.Verify(clone(image))
	if err != nil {
		return provider.ID(), nil, fmt.Errorf("verify with %q: %w", provider.ID(), err)
	}
	if len(results) == 0 {
		return provider.ID(), nil, fmt.Errorf("provider %q: %w", provider.ID(), ErrNoResults)
	}
	return provider.ID(), results, nil
}

// CorrectAndVerify corrects a private copy and returns it only after the same
// selected provider verifies at least one region and every region is valid.
func (r *Registry) CorrectAndVerify(image []byte) (string, []byte, []types.ChecksumResult, error) {
	provider, _, err := r.Select(image)
	if err != nil {
		return "", nil, nil, err
	}
	working := clone(image)
	if _, err := provider.Correct(working); err != nil {
		return provider.ID(), nil, nil, fmt.Errorf("correct with %q: %w", provider.ID(), err)
	}
	results, err := provider.Verify(clone(working))
	if err != nil {
		return provider.ID(), nil, nil, fmt.Errorf("post-correction verify with %q: %w", provider.ID(), err)
	}
	if !AllValid(results) {
		return provider.ID(), nil, results, fmt.Errorf("provider %q: %w", provider.ID(), ErrPostCorrectionVerify)
	}
	return provider.ID(), working, results, nil
}

func clone(image []byte) []byte { return append([]byte(nil), image...) }
