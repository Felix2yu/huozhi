package dto

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFlexDateUnmarshalJSONDate(t *testing.T) {
	var d FlexDate
	if err := json.Unmarshal([]byte(`"2026-03-15"`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Time.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("got %s", d.Time)
	}
}

func TestFlexDateUnmarshalJSONRFC(t *testing.T) {
	var d FlexDate
	if err := json.Unmarshal([]byte(`"2026-03-15T10:20:30Z"`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Time.Year() != 2026 || d.Time.Hour() != 10 {
		t.Fatalf("got %v", d.Time)
	}
}

func TestFlexDateUnmarshalJSONSlash(t *testing.T) {
	var d FlexDate
	if err := json.Unmarshal([]byte(`"2026/03/15"`), &d); err != nil {
		t.Fatal(err)
	}
	if d.Time.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("got %s", d.Time)
	}
}

func TestFlexDateUnmarshalJSONNullString(t *testing.T) {
	var d FlexDate
	if err := json.Unmarshal([]byte(`"null"`), &d); err != nil {
		t.Fatal(err)
	}
	if !d.Time.IsZero() {
		t.Fatal("expected zero time")
	}
}

func TestFlexDateUnmarshalInvalid(t *testing.T) {
	var d FlexDate
	// number is not a time -> parseString fails
	if err := json.Unmarshal([]byte(`12345`), &d); err == nil {
		t.Fatalf("expected error, got %v", d.Time)
	}
}

func TestFlexDateMarshalNull(t *testing.T) {
	d := FlexDate{}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "null" {
		t.Fatalf("got %s", b)
	}
}

func TestFlexDateMarshalDate(t *testing.T) {
	d := FlexDate{Time: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)}
	b, _ := json.Marshal(d)
	if string(b) != `"2026-03-15"` {
		t.Fatalf("got %s", b)
	}
}

func TestFlexDateUnmarshalParam(t *testing.T) {
	var d FlexDate
	if err := d.UnmarshalParam("2026-03-15"); err != nil {
		t.Fatal(err)
	}
	if d.Time.Format("2006-01-02") != "2026-03-15" {
		t.Fatal("param parse failed")
	}
}

func TestFlexDateString(t *testing.T) {
	d := FlexDate{Time: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)}
	if d.String() != "2026-03-15" {
		t.Fatal("String failed")
	}
	var z FlexDate
	if z.String() != "" {
		t.Fatal("zero String should be empty")
	}
	if !z.T().IsZero() {
		t.Fatal("T should be zero")
	}
}

func TestFlexDateParseStringEmpty(t *testing.T) {
	var d FlexDate
	if err := d.UnmarshalParam(""); err != nil {
		t.Fatal(err)
	}
	if !d.Time.IsZero() {
		t.Fatal("empty should be zero")
	}
	if err := d.UnmarshalParam("null"); err != nil {
		t.Fatal(err)
	}
}

func TestFlexDateParseStringFallback(t *testing.T) {
	// len > 10, first formats fail, s[:10] is a valid date
	var d FlexDate
	if err := d.UnmarshalParam("2026-03-15extra"); err != nil {
		t.Fatal(err)
	}
	if d.Time.Format("2006-01-02") != "2026-03-15" {
		t.Fatal("fallback parse failed")
	}
}

func TestFlexDateParseStringInvalid(t *testing.T) {
	var d FlexDate
	if err := d.UnmarshalParam("notadate"); err == nil {
		t.Fatal("expected error")
	}
}
