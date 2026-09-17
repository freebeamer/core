package vehicleprofile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func fixture() (types.VehicleProfile, Context) {
	workspace := types.BindingWorkspace{DefinitionID: "synthetic", DefinitionSHA256: strings.Repeat("a", 64), OriginalSHA256: strings.Repeat("b", 64), OriginalSize: 1024}
	profile := types.VehicleProfile{ID: "test", Label: "Synthetic only", ECUFamily: "mock-ecu", Software: "mock-sw", Workspace: workspace, Catalogs: []types.BindingCatalog{{ID: "catalog", Version: 1}}, Bindings: []types.AxisBinding{{ParameterID: "parameter-1", Axis: "x", ChannelID: "RPM", ChannelMeaning: "engine-speed", AxisMeaning: "engine-speed", SourceUnit: "rpm", TargetUnit: "rpm", Scale: 1}}}
	context := Context{Workspace: workspace, Axes: []types.BindingAxisOption{{ParameterID: "parameter-1", Title: "Duplicate title", Axis: "x", Unit: "rpm", Count: 2}, {ParameterID: "parameter-2", Title: "Duplicate title", Axis: "x", Unit: "%", Count: 2}}, Session: types.BindingSession{ScopeSHA256: strings.Repeat("c", 64), DeviceID: "device", SessionID: "session", CatalogID: "catalog", CatalogVersion: 1, ECUFamily: "mock-ecu", Software: "mock-sw", Channels: []string{"RPM"}}, Channels: []Channel{{ID: "RPM", Unit: "rpm", Meaning: "engine-speed"}}, ECUFamily: "mock-ecu", Software: []string{"mock-sw"}}
	return profile, context
}
func TestCompatibilityStatesAndReviewedEvidence(t *testing.T) {
	p, c := fixture()
	if got := Evaluate(p, c, false, nil); got.State != "unknown" || got.Verified {
		t.Fatal(got)
	}
	p.TelemetryEvidence = "user-entered claim"
	p.CalibrationEvidence = "another user-entered claim"
	if got := Evaluate(p, c, true, nil); got.State != "manual" || got.Verified {
		t.Fatal(got)
	}
	digest, _ := ProfileFingerprint(p)
	evidence := &Evidence{ProfileSHA256: digest, SessionSHA256: SessionFingerprint(c.Session), TelemetryReference: "synthetic-telemetry-trial", CalibrationReference: "synthetic-calibration-trial"}
	if got := Evaluate(p, c, true, evidence); got.State != "validated" || !got.Verified {
		t.Fatal(got)
	}
	c.Session.SessionID = "new-session"
	if got := Evaluate(p, c, true, evidence); got.State != "manual" || got.Verified {
		t.Fatal(got)
	}
	c.Session.Software = ""
	if got := Evaluate(p, c, true, evidence); got.State != "manual" || got.Verified {
		t.Fatal(got)
	}
	c.Session.SessionID = ""
	if got := Evaluate(p, c, true, evidence); got.State != "unknown" || got.Verified {
		t.Fatal(got)
	}
}
func TestRefusalCases(t *testing.T) {
	tests := []struct {
		name   string
		change func(*types.VehicleProfile, *Context)
	}{
		{"original firmware", func(p *types.VehicleProfile, c *Context) { c.Workspace.OriginalSHA256 = strings.Repeat("d", 64) }},
		{"definition", func(p *types.VehicleProfile, c *Context) { c.Workspace.DefinitionSHA256 = strings.Repeat("d", 64) }},
		{"software", func(p *types.VehicleProfile, c *Context) { c.Session.Software = "different" }},
		{"ECU", func(p *types.VehicleProfile, c *Context) { c.Session.ECUFamily = "different" }},
		{"workspace software", func(p *types.VehicleProfile, c *Context) { c.Software = []string{"different"} }},
		{"catalog version", func(p *types.VehicleProfile, c *Context) { c.Session.CatalogVersion = 2 }},
		{"missing catalog channel", func(p *types.VehicleProfile, c *Context) { c.Channels = nil }},
		{"missing source channel", func(p *types.VehicleProfile, c *Context) { c.Session.Channels = nil }},
		{"title is not identity", func(p *types.VehicleProfile, c *Context) { p.Bindings[0].ParameterID = "Duplicate title" }},
		{"duplicate titles distinct axes", func(p *types.VehicleProfile, c *Context) { p.Bindings[0].ParameterID = "parameter-2" }},
		{"duplicate parameter axis", func(p *types.VehicleProfile, c *Context) { c.Axes = append(c.Axes, c.Axes[0]) }},
		{"scalar axis", func(p *types.VehicleProfile, c *Context) { c.Axes[0].Count = 1 }},
		{"absent y axis", func(p *types.VehicleProfile, c *Context) { p.Bindings[0].Axis = "y" }},
		{"wrong unit", func(p *types.VehicleProfile, c *Context) { p.Bindings[0].TargetUnit = "%" }},
		{"wrong conversion", func(p *types.VehicleProfile, c *Context) { p.Bindings[0].Scale = 2 }},
		{"wrong meaning", func(p *types.VehicleProfile, c *Context) { p.Bindings[0].AxisMeaning = "vehicle-speed" }},
		{"duplicate binding", func(p *types.VehicleProfile, c *Context) { p.Bindings = append(p.Bindings, p.Bindings[0]) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p, c := fixture()
			test.change(&p, &c)
			if got := Evaluate(p, c, true, nil); got.State != "incompatible" || got.Verified {
				t.Fatal(got)
			}
		})
	}
}
func TestUnitAndMeaningConversions(t *testing.T) {
	tests := []struct {
		from, to, meaning string
		scale, offset     float64
	}{{"psi", "kPa", "pressure-absolute", 6.894757293168, 0}, {"°C", "°F", "temperature", 1.8, 32}, {"%", "ratio", "load-measured", .01, 0}, {"km/h", "m/s", "vehicle-speed", 1.0 / 3.6, 0}}
	for _, test := range tests {
		scale, offset, err := Conversion(test.from, test.to, test.meaning)
		if err != nil || !closeEnough(scale, test.scale) || !closeEnough(offset, test.offset) {
			t.Fatalf("%+v: %v %v %v", test, scale, offset, err)
		}
	}
	if _, _, err := Conversion("rpm", "%", "engine-speed"); err == nil {
		t.Fatal("dimension mismatch accepted")
	}
	for _, meanings := range [][2]string{{"load-measured", "load-requested"}, {"pressure-absolute", "pressure-gauge"}} {
		p, c := fixture()
		p.Bindings[0].ChannelMeaning = meanings[0]
		p.Bindings[0].AxisMeaning = meanings[1]
		unit := "%"
		if meanings[0] == "pressure-absolute" {
			unit = "kPa"
		}
		p.Bindings[0].SourceUnit = unit
		p.Bindings[0].TargetUnit = unit
		c.Axes[0].Unit = unit
		c.Channels[0].Unit = unit
		c.Channels[0].Meaning = meanings[0]
		if got := Evaluate(p, c, true, nil); got.State != "incompatible" {
			t.Fatal(got)
		}
	}
}
func TestFingerprintsArePathIndependentAndSensitiveToContent(t *testing.T) {
	definition := &types.MapDefinition{ID: "test", Provenance: types.MapProvenance{Source: "/first/file.xdf", SourceSHA256: strings.Repeat("a", 64)}, Parameters: []types.MapParameter{{ID: "stable", Name: "same", Axes: map[string]types.MapAxis{"x": {Count: 2, Unit: "rpm"}}}}}
	first, err := DefinitionFingerprint(definition)
	if err != nil {
		t.Fatal(err)
	}
	definition.Provenance.Source = "/other/file.xdf"
	second, _ := DefinitionFingerprint(definition)
	if first != second {
		t.Fatal("moving file changed fingerprint")
	}
	definition.Parameters[0].Axes["x"] = types.MapAxis{Count: 2, Unit: "%"}
	third, _ := DefinitionFingerprint(definition)
	if first == third {
		t.Fatal("axis change retained fingerprint")
	}
	_, context := fixture()
	digest := SessionFingerprint(context.Session)
	context.Session.Channels = append(context.Session.Channels, "LOAD")
	if digest != SessionFingerprint(context.Session) {
		t.Fatal("sample channels changed session identity")
	}
	context.Session.VehicleLabel = "changed"
	if digest == SessionFingerprint(context.Session) {
		t.Fatal("vehicle claim change retained session identity")
	}
}
func TestPersistenceStrictRoundTripAndCorruption(t *testing.T) {
	p, _ := fixture()
	path := filepath.Join(t.TempDir(), "profiles.json")
	if err := Write(path, []types.VehicleProfile{p}); err != nil {
		t.Fatal(err)
	}
	profiles, err := Read(path)
	if err != nil || len(profiles) != 1 || profiles[0].Bindings[0].ParameterID != "parameter-1" {
		t.Fatal(profiles, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("profile permissions", info.Mode())
	}
	original, _ := os.ReadFile(path)
	for _, data := range [][]byte{[]byte(`{"format":"freehorse-vehicle-profiles","version":99,"profiles":[]}`), []byte(`{"format":"freehorse-vehicle-profiles","version":1,"profiles":[],"verified":true}`), append(append([]byte{}, original...), []byte(` {}`)...), []byte(`{`)} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(path); err == nil {
			t.Fatal("accepted invalid document", string(data))
		}
	}
	bad := p
	bad.Bindings = nil
	if err := Write(path, []types.VehicleProfile{bad}); err == nil {
		t.Fatal("accepted empty bindings")
	}
	after, _ := os.ReadFile(path)
	if string(after) != "{" {
		t.Fatal("failed save altered existing file")
	}
	blob, _ := json.Marshal(Document{Format: Format, Version: 1, Profiles: []types.VehicleProfile{p, p}})
	os.WriteFile(path, blob, 0600)
	if _, err := Read(path); err == nil {
		t.Fatal("accepted duplicate IDs")
	}
}
