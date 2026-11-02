package phone

import (
	"slices"
	"testing"
)

func TestNormalizeUSFormats(t *testing.T) {
	// Area code 555 does not exist, so these are well-formed but invalid;
	// they must still be read as +1 numbers and explained, not mangled.
	for _, in := range []string{"(555) 123-4567", "555.123.4567", "+1 555 123 4567", "15551234567", "555-123-4567"} {
		r := Normalize(in, "US")
		if r.Problem != ProblemInvalid {
			t.Errorf("%q: problem = %q, want invalid (area code 555 does not exist)", in, r.Problem)
		}
		if r.Message == "" {
			t.Errorf("%q: no explanation", in)
		}
	}
	for _, in := range []string{"(212) 555-0123", "212.555.0123", "+1 212 555 0123", "12125550123", "212-555-0123",
		"2125550123", " 212 555 0123 ", "+1-212-555-0123", "1 (212) 555-0123", "'2125550123"} {
		r := Normalize(in, "US")
		if !r.OK() || r.E164 != "+12125550123" {
			t.Errorf("%q: got %+v, want +12125550123", in, r)
		}
		if len(r.Fixes) != 0 {
			t.Errorf("%q: unexpected fixes %v", in, r.Fixes)
		}
		if r.Display != "+1 212-555-0123" {
			t.Errorf("%q: display = %q", in, r.Display)
		}
	}
}

func TestNormalizeInternational(t *testing.T) {
	cases := map[string]string{
		"+44 7911 123456":    "+447911123456",
		"+92 300 1234567":    "+923001234567",
		"+971 50 123 4567":   "+971501234567",
		"0044 7911 123456":   "+447911123456",
		"011 44 7911 123456": "+447911123456", // US international dialling prefix
		"447911123456":       "+447911123456", // country code without +
		"+١ ٢١٢ ٥٥٥ ٠١٢٣":    "+12125550123",  // Arabic-Indic digits
	}
	for in, want := range cases {
		r := Normalize(in, "US")
		if !r.OK() || r.E164 != want {
			t.Errorf("%q: got %+v, want %s", in, r, want)
		}
	}
	// With the UK as the default country, a national number gains +44.
	if r := Normalize("07911 123456", "GB"); r.E164 != "+447911123456" {
		t.Errorf("UK national: got %+v", r)
	}
	if r := Normalize("0044 7911 123456", "US"); !slices.Contains(r.Fixes, FixIntlPrefix) {
		t.Errorf("00 prefix not reported as a fix: %+v", r)
	}
	if r := Normalize("447911123456", "US"); !slices.Contains(r.Fixes, FixInternational) {
		t.Errorf("missing + not reported as a fix: %+v", r)
	}
}

func TestNormalizeExcelNotation(t *testing.T) {
	r := Normalize("1.55512E+10", "US")
	if r.Problem != ProblemExcelDamaged {
		t.Fatalf("damaged scientific notation: got %+v", r)
	}
	if r.Message != "This number was damaged by Excel. Please fix it in the original file." {
		t.Errorf("message = %q", r.Message)
	}
	if r := Normalize("1.21255E+10", "US"); r.Problem != ProblemExcelDamaged {
		t.Errorf("got %+v", r)
	}
	// Every digit survived in the mantissa: recoverable.
	r = Normalize("1.2125550123E+10", "US")
	if !r.OK() || r.E164 != "+12125550123" || !slices.Contains(r.Fixes, FixExcelNumber) {
		t.Errorf("full-precision scientific notation: got %+v", r)
	}
	r = Normalize("12125550123.0", "US")
	if !r.OK() || r.E164 != "+12125550123" || !slices.Contains(r.Fixes, FixExcelNumber) {
		t.Errorf("trailing .0: got %+v", r)
	}
}

func TestNormalizeProblems(t *testing.T) {
	cases := map[string]Problem{
		"":                             ProblemMissing,
		"   ":                          ProblemMissing,
		"N/A":                          ProblemMissing,
		"none":                         ProblemMissing,
		"no phone":                     ProblemMissing,
		"555-0123":                     ProblemTooShort,
		"12345":                        ProblemTooShort,
		"21255501234567890":            ProblemTooLong,
		"212-555-0123 / 646-555-0188":  ProblemMultiple,
		"212-555-0123 or 646-555-0188": ProblemMultiple,
		"(000) 000-0000":               ProblemInvalid,
		"+999 1234 5678":               ProblemCountryCode,
	}
	for in, want := range cases {
		if r := Normalize(in, "US"); r.Problem != want || r.Message == "" {
			t.Errorf("%q: problem = %q (%q), want %q", in, r.Problem, r.Message, want)
		}
	}
}

func TestNormalizeCleansExtras(t *testing.T) {
	cases := map[string]Fix{
		"212-555-0123 ext. 45": FixExtension,
		"212-555-0123 x45":     FixExtension,
		"Cell: 212-555-0123":   FixExtraText,
		"212-555-0123 (cell)":  FixExtraText,
		"tel:+12125550123":     FixExtraText,
	}
	for in, fix := range cases {
		r := Normalize(in, "US")
		if !r.OK() || r.E164 != "+12125550123" || !slices.Contains(r.Fixes, fix) {
			t.Errorf("%q: got %+v, want fix %q", in, r, fix)
		}
		if r.Message == "" {
			t.Errorf("%q: fix not explained", in)
		}
	}
}

func TestCountries(t *testing.T) {
	cs := Countries()
	if len(cs) < 200 {
		t.Fatalf("only %d countries", len(cs))
	}
	var us Country
	for _, c := range cs {
		if c.Code == "US" {
			us = c
		}
	}
	if us.Name != "United States" || us.Dial != "+1" {
		t.Errorf("US = %+v", us)
	}
	if !ValidRegion("GB") || ValidRegion("XX") {
		t.Error("ValidRegion")
	}
}
