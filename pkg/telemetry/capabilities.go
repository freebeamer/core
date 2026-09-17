package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CatalogRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type Capabilities struct {
	Versions []int        `json:"telemetry_versions"`
	Catalogs []CatalogRef `json:"catalogs"`
}

func RelayCapabilities() Capabilities {
	return Capabilities{Versions: []int{1, ContractVersion}, Catalogs: []CatalogRef{{ID: MG1CatalogID, Version: MG1CatalogVersion}}}
}

// Negotiate selects legacy only on explicit legacy evidence. Offline/unknown
// relays return an error: producers retain v2 identity locally until delivery
// can verify support. No endpoint means an offline v2 recording.
func (u *Uploader) Negotiate() (int, error) {
	if u.Endpoint == "" {
		return ContractVersion, nil
	}
	endpoint, err := url.Parse(u.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || !strings.HasSuffix(endpoint.Path, "/telemetry") {
		return 0, fmt.Errorf("telemetry: capability check requires an HTTP(S) /telemetry endpoint")
	}
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/telemetry") + "/capabilities"
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	timeout := u.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, err
	}
	if u.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+u.APIKey)
	}
	client := u.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	// Never follow capability redirects, which could advertise a different relay.
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := copyClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return 1, nil
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("telemetry: capability check status %d", resp.StatusCode)
	}
	var capabilities Capabilities
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 64*1024))
	if err = decoder.Decode(&capabilities); err != nil {
		return 0, err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return 0, fmt.Errorf("telemetry: invalid capabilities response")
	}
	hasLegacy, hasV2, hasCatalog := false, false, false
	for _, version := range capabilities.Versions {
		hasLegacy = hasLegacy || version == 1
		hasV2 = hasV2 || version == ContractVersion
	}
	for _, catalog := range capabilities.Catalogs {
		hasCatalog = hasCatalog || (catalog.ID == MG1CatalogID && catalog.Version == MG1CatalogVersion)
	}
	if hasV2 && hasCatalog {
		return ContractVersion, nil
	}
	if hasLegacy {
		return 1, nil
	}
	return 0, fmt.Errorf("telemetry: relay offers no compatible contract/catalog")
}
