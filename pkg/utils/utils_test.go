package utils_test

import (
	"encoding/xml"
	"testing"

	"github.com/freebeamer/core/pkg/utils"
)

func TestAttrsNormalizesNames(t *testing.T) {
	attributes := []xml.Attr{{Name: xml.Name{Local: "MixedCase"}, Value: "value"}}
	if got := utils.Attrs(attributes)["mixedcase"]; got != "value" {
		t.Fatalf("attribute = %q", got)
	}
}

func TestInteger(t *testing.T) {
	values := map[string]string{"hex": " 0x10 ", "bad": "no"}
	if value, err := utils.Integer(values, "hex", 64); err != nil || value != 16 {
		t.Fatalf("Integer = %d, %v", value, err)
	}
	if _, err := utils.Integer(values, "bad", 64); err == nil {
		t.Fatal("invalid unsigned integer accepted")
	}
}

func TestSignedInteger(t *testing.T) {
	values := map[string]string{"negative": "-32", "bad": "no"}
	if value, err := utils.SignedInteger(values, "negative"); err != nil || value != -32 {
		t.Fatalf("SignedInteger = %d, %v", value, err)
	}
	if _, err := utils.SignedInteger(values, "bad"); err == nil {
		t.Fatal("invalid signed integer accepted")
	}
}

func TestIntField(t *testing.T) {
	if value, err := utils.IntField(map[string]string{"count": "12"}, "count"); err != nil || value != 12 {
		t.Fatalf("IntField = %d, %v", value, err)
	}
}

func TestBoolField(t *testing.T) {
	for input, want := range map[string]bool{"": false, "0": false, "1": true, "true": true, "false": false} {
		value, err := utils.BoolField(map[string]string{"flag": input}, "flag")
		if err != nil || value != want {
			t.Errorf("BoolField(%q) = %v, %v", input, value, err)
		}
	}
	if _, err := utils.BoolField(map[string]string{"flag": "maybe"}, "flag"); err == nil {
		t.Fatal("invalid boolean accepted")
	}
}
