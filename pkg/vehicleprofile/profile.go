// Package vehicleprofile evaluates explicit telemetry/calibration associations.
// It never infers physical equivalence from names or edits firmware.
package vehicleprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/freebeamer/core/pkg/types"
)

func fingerprint(prefix string, value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(append([]byte(prefix+"\n"), data...))
	return hex.EncodeToString(hash[:]), nil
}
func DefinitionFingerprint(definition *types.MapDefinition) (string, error) {
	copy := *definition
	copy.Provenance.Source = ""
	return fingerprint("freehorse-definition-v1", copy)
}
func ProfileFingerprint(profile types.VehicleProfile) (string, error) {
	return fingerprint("freehorse-profile-v1", profile)
}
func SessionFingerprint(session types.BindingSession) string {
	session.Channels = nil
	result, _ := fingerprint("freehorse-session-v1", session)
	return result
}

// Evidence is supplied by a reviewed registry, NEVER deserialized from user profiles.
// The current desktop has no reviewed real-vehicle entries.
type Evidence struct {
	ProfileSHA256        string
	SessionSHA256        string
	TelemetryReference   string
	CalibrationReference string
}
type Channel struct{ ID, Unit, Meaning string }
type Context struct {
	Workspace types.BindingWorkspace
	Axes      []types.BindingAxisOption
	Session   types.BindingSession
	Channels  []Channel
	ECUFamily string
	Software  []string
}

func Validate(profile types.VehicleProfile) error {
	if strings.TrimSpace(profile.ID) == "" || len(profile.ID) > 128 || strings.TrimSpace(profile.Label) == "" || len(profile.Label) > 256 {
		return fmt.Errorf("profile ID and label are required (maximum 128/256 characters)")
	}
	for _, hash := range []string{profile.Workspace.DefinitionSHA256, profile.Workspace.OriginalSHA256} {
		b, err := hex.DecodeString(hash)
		if err != nil || len(b) != 32 || hash != strings.ToLower(hash) {
			return fmt.Errorf("workspace requires lowercase SHA-256 fingerprints")
		}
	}
	if profile.Workspace.DefinitionID == "" || profile.Workspace.OriginalSize == 0 {
		return fmt.Errorf("workspace identity is incomplete")
	}
	if len(profile.Catalogs) == 0 || len(profile.Catalogs) > 32 {
		return fmt.Errorf("one to 32 eligible catalogs are required")
	}
	catalogs := map[types.BindingCatalog]bool{}
	for _, catalog := range profile.Catalogs {
		if catalog.ID == "" || catalog.Version < 1 || catalogs[catalog] {
			return fmt.Errorf("invalid or duplicate catalog")
		}
		catalogs[catalog] = true
	}
	if len(profile.Bindings) == 0 || len(profile.Bindings) > 256 {
		return fmt.Errorf("one to 256 axis bindings are required")
	}
	seen := map[string]bool{}
	for _, b := range profile.Bindings {
		key := b.ParameterID + "\x00" + b.Axis
		if b.ParameterID == "" || (b.Axis != "x" && b.Axis != "y") || b.ChannelID == "" || seen[key] {
			return fmt.Errorf("invalid or duplicate parameter/axis binding")
		}
		seen[key] = true
		if !finite(b.Scale) || !finite(b.Offset) || b.Scale == 0 {
			return fmt.Errorf("conversion must have a finite nonzero scale and finite offset")
		}
		if _, ok := meaningDimension[b.ChannelMeaning]; !ok {
			return fmt.Errorf("unknown channel physical meaning")
		}
		if _, ok := meaningDimension[b.AxisMeaning]; !ok {
			return fmt.Errorf("unknown axis physical meaning")
		}
	}
	return nil
}

func Evaluate(profile types.VehicleProfile, context Context, manual bool, evidence *Evidence) types.BindingCompatibility {
	result := types.BindingCompatibility{State: "unknown", Reasons: []string{}}
	incompatible := func(reason string) { result.State = "incompatible"; result.Reasons = append(result.Reasons, reason) }
	if err := Validate(profile); err != nil {
		incompatible(err.Error())
		return result
	}
	if profile.Workspace != context.Workspace {
		incompatible("definition or original firmware identity does not match")
		return result
	}
	if context.ECUFamily != "" && profile.ECUFamily != "" && context.ECUFamily != profile.ECUFamily {
		incompatible("workspace ECU family contradicts profile")
	}
	if len(context.Software) > 0 && profile.Software != "" && !contains(context.Software, profile.Software) {
		incompatible("workspace software identifier contradicts profile")
	}
	session := context.Session
	if session.ECUFamily != "" && profile.ECUFamily != "" && session.ECUFamily != profile.ECUFamily {
		incompatible("telemetry ECU family contradicts profile")
	}
	if session.Software != "" && profile.Software != "" && session.Software != profile.Software {
		incompatible("telemetry software identifier contradicts profile")
	}
	if session.CatalogID != "" {
		eligible := false
		for _, c := range profile.Catalogs {
			if c.ID == session.CatalogID && c.Version == session.CatalogVersion {
				eligible = true
			}
		}
		if !eligible {
			incompatible("session catalog is not eligible for this profile")
		}
	}
	if session.SessionID == "" || session.CatalogID == "" || session.ScopeSHA256 == "" {
		if result.State != "incompatible" {
			result.Reasons = append(result.Reasons, "session/catalog identity is unknown")
		}
		return result
	}
	for _, binding := range profile.Bindings {
		var axis *types.BindingAxisOption
		for i := range context.Axes {
			a := &context.Axes[i]
			if a.ParameterID == binding.ParameterID && a.Axis == binding.Axis {
				if axis != nil {
					incompatible("ambiguous parameter ID/axis")
				}
				axis = a
			}
		}
		if axis == nil || axis.Count < 2 {
			incompatible("binding axis is absent, implicit, nonnumeric or dimensionally invalid: " + binding.ParameterID + "/" + binding.Axis)
			continue
		}
		var channel *Channel
		for i := range context.Channels {
			if context.Channels[i].ID == binding.ChannelID {
				channel = &context.Channels[i]
			}
		}
		if channel == nil {
			incompatible("channel is absent from the session catalog: " + binding.ChannelID)
			continue
		}
		if !contains(session.Channels, binding.ChannelID) {
			incompatible("channel is absent from the selected source: " + binding.ChannelID)
		}
		if normalizeUnit(binding.TargetUnit) != normalizeUnit(axis.Unit) || normalizeUnit(binding.SourceUnit) != normalizeUnit(channel.Unit) {
			incompatible("declared units do not match channel/axis units")
		}
		if binding.ChannelMeaning != binding.AxisMeaning || (channel.Meaning != "" && channel.Meaning != binding.ChannelMeaning) {
			incompatible("physical meanings differ; contextual conversions are unsupported")
		}
		scale, offset, err := Conversion(binding.SourceUnit, binding.TargetUnit, binding.ChannelMeaning)
		if err != nil || !closeEnough(scale, binding.Scale) || !closeEnough(offset, binding.Offset) {
			incompatible("unit conversion is incompatible with the declared physical meaning")
		}
	}
	if result.State == "incompatible" {
		return result
	}
	if session.SessionID == "" || session.CatalogID == "" || session.ScopeSHA256 == "" {
		result.Reasons = append(result.Reasons, "session/catalog identity is unknown")
		return result
	}
	identityKnown := profile.ECUFamily != "" && profile.Software != "" && session.ECUFamily != "" && session.Software != ""
	if !identityKnown {
		result.Reasons = append(result.Reasons, "vehicle ECU/software identity is incomplete")
	}
	if manual {
		result.State = "manual"
		result.Reasons = append(result.Reasons, "manually associated; vehicle/calibration equivalence is unverified")
		digest, _ := ProfileFingerprint(profile)
		if identityKnown && evidence != nil && evidence.ProfileSHA256 == digest && evidence.SessionSHA256 == SessionFingerprint(session) && evidence.TelemetryReference != "" && evidence.CalibrationReference != "" {
			result.State = "validated"
			result.Verified = true
			result.Reasons = []string{"reviewed telemetry and calibration evidence match this exact profile and session"}
		}
	} else {
		result.Reasons = append(result.Reasons, "select this profile explicitly to associate the current session")
	}
	return result
}
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func finite(value float64) bool     { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func closeEnough(a, b float64) bool { return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(a)) }

var meaningDimension = map[string]string{"engine-speed": "rotation", "load-measured": "ratio", "load-requested": "ratio", "pressure-absolute": "pressure", "pressure-gauge": "pressure", "temperature": "temperature", "torque-measured": "torque", "torque-requested": "torque", "vehicle-speed": "speed", "lambda": "lambda"}

type unit struct {
	dimension     string
	scale, offset float64
}

var units = map[string]unit{"rpm": {"rotation", 1, 0}, "%": {"ratio", .01, 0}, "ratio": {"ratio", 1, 0}, "kpa": {"pressure", 1, 0}, "bar": {"pressure", 100, 0}, "psi": {"pressure", 6.894757293168, 0}, "pa": {"pressure", .001, 0}, "°c": {"temperature", 1, 273.15}, "k": {"temperature", 1, 0}, "°f": {"temperature", 5.0 / 9, 255.3722222222222}, "nm": {"torque", 1, 0}, "km/h": {"speed", 1.0 / 3.6, 0}, "m/s": {"speed", 1, 0}, "lambda": {"lambda", 1, 0}}

func normalizeUnit(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "1/min", "r/min":
		return "rpm"
	case "c", "degc":
		return "°c"
	case "f", "degf":
		return "°f"
	case "n·m", "n*m":
		return "nm"
	}
	return value
}
func Conversion(source, target, meaning string) (float64, float64, error) {
	a, okA := units[normalizeUnit(source)]
	b, okB := units[normalizeUnit(target)]
	dimension, known := meaningDimension[meaning]
	if !okA || !okB || !known || a.dimension != b.dimension || a.dimension != dimension {
		return 0, 0, fmt.Errorf("unsupported units or physical meaning")
	}
	return a.scale / b.scale, (a.offset - b.offset) / b.scale, nil
}
