package telemetry

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSharedContractFixtures(t *testing.T) {
	for _, name := range []string{"v2", "legacy"} {
		data, err := os.ReadFile("../../testdata/public/telemetry/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var sample Sample
		if err = json.Unmarshal(data, &sample); err != nil {
			t.Fatal(err)
		}
		if err = sample.Validate(); err != nil {
			t.Fatal(err)
		}
		roundtrip, err := json.Marshal(sample)
		if err != nil {
			t.Fatal(err)
		}
		var before, after any
		json.Unmarshal(data, &before)
		json.Unmarshal(roundtrip, &after)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("%s changed: %s", name, roundtrip)
		}
	}
}

func TestContractRejectsInvalidIdentity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/public/telemetry/v2.json")
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Sample){
		"version":         func(s *Sample) { s.Version = 3 },
		"catalog":         func(s *Sample) { s.Session.CatalogVersion = 2 },
		"sequence":        func(s *Sample) { s.Sequence = nil },
		"overflow":        func(s *Sample) { n := MaxSequence + 1; s.Sequence = &n },
		"legacy_identity": func(s *Sample) { s.Version = 0 },
		"unknown_channel": func(s *Sample) { s.Values["NOPE"] = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			var s Sample
			json.Unmarshal(data, &s)
			mutate(&s)
			if s.Validate() == nil {
				t.Fatal("accepted invalid sample")
			}
		})
	}
}

func TestCatalogPreservesUnitsAndValidation(t *testing.T) {
	c := MonitorCatalog()
	if len(c.Channels) != 19 || c.Channels[0].Unit != "rpm" || c.Channels[0].SourceUnit != "l/min" {
		t.Fatalf("catalog: %+v", c)
	}
	for _, channel := range c.Channels {
		if channel.Validation != "definition-derived" {
			t.Fatal("unsupported hardware claim")
		}
	}
}
