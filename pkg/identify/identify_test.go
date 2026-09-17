package identify

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/freebeamer/core/pkg/types"
)

func targetImage(markers string) []byte {
	image := make([]byte, targetSize)
	copy(image[0x1000:], markers)
	return image
}

func TestIdentifyMEVD1726N55WithIndependentSignals(t *testing.T) {
	identity := Identify(targetImage("Bosch MEVD17.2.6 BMW N55 75P9EJ0B FEF81A001"))
	if identity.Manufacturer != "BMW" || identity.ECUVendor != "Bosch" || identity.ECUFamily != "MEVD17.2.6" || identity.Engine != "BMW N55" || identity.Confidence != types.IdentificationHigh {
		t.Fatalf("identity=%#v", identity)
	}
	if !reflect.DeepEqual(identity.Software, []string{"75P9EJ0B", "FEF81A001"}) {
		t.Fatalf("software=%v", identity.Software)
	}
}

func TestIdentificationDoesNotUseFilename(t *testing.T) {
	identity := Identify(targetImage("unrelated binary content"))
	if identity.Confidence != types.IdentificationNone || identity.ECUFamily != "" {
		t.Fatalf("identity=%#v", identity)
	}
}

func TestConfidenceRequiresMultipleSignals(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want types.IdentificationConfidence
	}{{"family only", []byte("MEVD17.2.6"), types.IdentificationLow}, {"family and size", targetImage("MEVD17.2.6"), types.IdentificationMedium}, {"markers wrong size", []byte("BMW N55 MEVD17.2.6 75P9EJ0B"), types.IdentificationMedium}, {"BMW and N55 only", []byte("BMW N55"), types.IdentificationLow}, {"size only", make([]byte, targetSize), types.IdentificationNone}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Identify(test.data).Confidence; got != test.want {
				t.Fatalf("confidence=%s want %s", got, test.want)
			}
		})
	}
}

func TestUTF16MarkersAreDetected(t *testing.T) {
	text := []byte("BMW N55 MEVD17.2.6 75P9EJ0B")
	image := make([]byte, targetSize)
	for index, value := range text {
		image[0x2000+index*2] = value
	}
	identity := Identify(image)
	if identity.Confidence != types.IdentificationHigh || identity.ECUFamily != "MEVD17.2.6" {
		t.Fatalf("identity=%#v", identity)
	}
}

func TestSoftwareIdentifiersAreUniqueAndSorted(t *testing.T) {
	identity := Identify(targetImage("FEF81A001 75P9EJ0B FEF81A001 1037555555"))
	want := []string{"1037555555", "75P9EJ0B", "FEF81A001"}
	if !reflect.DeepEqual(identity.Software, want) {
		t.Fatalf("software=%v", identity.Software)
	}
}

func TestContainsMarkerCaseInsensitive(t *testing.T) {
	if !ContainsMarker([]byte("xxMeVd17.2.6xx"), "MEVD17.2.6") {
		t.Fatal("marker not found")
	}
	if ContainsMarker(bytes.Repeat([]byte{0}, 10), "BMW") {
		t.Fatal("false marker")
	}
}
