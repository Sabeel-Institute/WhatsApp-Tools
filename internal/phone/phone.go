// Package phone turns the phone numbers people type into spreadsheets into
// international format, repairing what can be repaired safely and explaining
// in plain words what cannot.
package phone

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/nyaruka/phonenumbers"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// Problem is why a number cannot be used.
type Problem string

const (
	ProblemNone         Problem = ""
	ProblemMissing      Problem = "missing"
	ProblemTooShort     Problem = "too_short"
	ProblemTooLong      Problem = "too_long"
	ProblemInvalid      Problem = "invalid"
	ProblemExcelDamaged Problem = "excel_damaged"
	ProblemMultiple     Problem = "multiple"
	ProblemCountryCode  Problem = "country_code"
)

// Fix is an automatic repair.
type Fix string

const (
	FixExcelNumber   Fix = "excel_number"  // "1.2125550123E+10" or "12125550123.0"
	FixIntlPrefix    Fix = "intl_prefix"   // "00" dialling prefix became "+"
	FixExtension     Fix = "extension"     // "x123" removed
	FixExtraText     Fix = "extra_text"    // words like "cell" removed
	FixInternational Fix = "international" // country code was present without "+"
)

// Result is a normalized number.
type Result struct {
	Input   string  `json:"input"`
	E164    string  `json:"e164,omitempty"`    // "+12125550123"; empty if unusable
	Display string  `json:"display,omitempty"` // "+1 212-555-0123"
	Region  string  `json:"region,omitempty"`  // "US"
	Fixes   []Fix   `json:"fixes,omitempty"`
	Problem Problem `json:"problem,omitempty"`
	// Message explains the problem or the fixes in plain words.
	Message string `json:"message,omitempty"`
}

// OK reports whether the number can be used.
func (r Result) OK() bool { return r.Problem == ProblemNone && r.E164 != "" }

// CountryName returns the English name of a region code, e.g. "United States".
func CountryName(region string) string {
	if r, err := language.ParseRegion(region); err == nil {
		if n := display.English.Regions().Name(r); n != "" {
			return n
		}
	}
	return region
}

var (
	placeholders = map[string]bool{"": true, "-": true, "--": true, "n/a": true, "na": true, "none": true,
		"null": true, "nil": true, "no": true, "unknown": true, "0": true, "#n/a": true, "tbd": true, "?": true}
	// Excel shows long numbers as 1.21255E+10 (digits lost) or keeps them
	// whole in the mantissa (1.2125550123E+10, recoverable).
	sciNotation = regexp.MustCompile(`^\+?(\d)[.,](\d+)[eE]\+?(\d{1,2})$`)
	trailingDot = regexp.MustCompile(`^(\+?\d{7,})[.,]0+$`)
	labelPrefix = regexp.MustCompile(`(?i)^(tel|phone|cell|mobile|whatsapp|wa|mob|m|c|h|w)\s*[:.]\s*`)
	extension   = regexp.MustCompile(`(?i)\s*(?:ext\.?|extension|x|#)\s*\d{1,6}\s*$`)
	multiSplit  = regexp.MustCompile(`(?i)\s*(?:/|;|,|&|\bor\b|\band\b|\n)\s*`)
	letterWords = regexp.MustCompile(`\(?\p{L}[\p{L}.]*\)?`)
)

func countDigits(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// clean removes invisible characters and normalizes spaces and dashes.
func clean(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\u00a0' || r == '\u2007' || r == '\u202f':
			b.WriteRune(' ')
		case r == '‐' || r == '‑' || r == '‒' || r == '–' || r == '—' || r == '−':
			b.WriteRune('-')
		case r == '＋':
			b.WriteRune('+')
		case unicode.Is(unicode.Cf, r): // zero-width and direction marks
		case unicode.IsDigit(r) && (r < '0' || r > '9'):
			// Arabic-Indic and full-width digits become ASCII.
			if d := digitValue(r); d >= 0 {
				b.WriteRune(rune('0' + d))
			} else {
				b.WriteRune(r)
			}
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// digitValue returns the value of a Unicode decimal digit, or -1.
func digitValue(r rune) int {
	for _, base := range []rune{0x0660, 0x06F0, 0x0966, 0x09E6, 0xFF10} {
		if r >= base && r <= base+9 {
			return int(r - base)
		}
	}
	return -1
}

// Normalize reads input as a phone number, using region (e.g. "US") for
// numbers written without a country code.
func Normalize(input, region string) Result {
	res := Result{Input: input}
	s := clean(input)
	s = strings.TrimPrefix(s, "'") // Excel's "treat as text" marker
	s = strings.TrimSpace(s)

	if placeholders[strings.ToLower(s)] {
		return res.fail(ProblemMissing, "There's no phone number for this person.")
	}

	// Excel number formats.
	if m := sciNotation.FindStringSubmatch(s); m != nil {
		exp := atoi(m[3])
		if len(m[2]) != exp {
			return res.fail(ProblemExcelDamaged,
				"This number was damaged by Excel. Please fix it in the original file.")
		}
		s = m[1] + m[2]
		res.Fixes = append(res.Fixes, FixExcelNumber)
	} else if m := trailingDot.FindStringSubmatch(s); m != nil {
		s = m[1]
		res.Fixes = append(res.Fixes, FixExcelNumber)
	}

	if loc := labelPrefix.FindStringIndex(s); loc != nil && countDigits(s[loc[1]:]) >= 7 {
		s = s[loc[1]:]
		res.Fixes = append(res.Fixes, FixExtraText)
	}

	// Several numbers in one cell: ask rather than guess.
	parts := 0
	for _, p := range multiSplit.Split(s, -1) {
		if countDigits(p) >= 7 {
			parts++
		}
	}
	if parts >= 2 {
		return res.fail(ProblemMultiple, "This cell has more than one number. Keep just one.")
	}

	if loc := extension.FindStringIndex(s); loc != nil && countDigits(s[:loc[0]]) >= 7 {
		s = s[:loc[0]]
		res.Fixes = append(res.Fixes, FixExtension)
	}

	if strings.IndexFunc(s, unicode.IsLetter) >= 0 {
		stripped := strings.TrimSpace(letterWords.ReplaceAllString(s, " "))
		if countDigits(stripped) >= 7 && strings.IndexFunc(stripped, unicode.IsLetter) < 0 {
			s = stripped
			res.Fixes = append(res.Fixes, FixExtraText)
		} else if countDigits(s) == 0 {
			return res.fail(ProblemMissing, "This isn't a phone number.")
		}
	}

	plus := strings.HasPrefix(s, "+")
	digits := digitsOnly(s)
	if !plus && strings.HasPrefix(digits, "00") && len(digits) > 9 && digits[2] != '0' {
		digits = digits[2:]
		plus = true
		res.Fixes = append(res.Fixes, FixIntlPrefix)
	}

	switch n := len(digits); {
	case n == 0:
		return res.fail(ProblemMissing, "There's no phone number for this person.")
	case n < 7:
		return res.fail(ProblemTooShort, "This number is too short. Check that no digits are missing.")
	case n > 15:
		return res.fail(ProblemTooLong, "This number is too long. Check for extra digits.")
	}

	candidate := digits
	if plus {
		candidate = "+" + digits
	}
	num, err := phonenumbers.Parse(candidate, region)
	if err != nil || !phonenumbers.IsValidNumber(num) {
		// A number that starts with its country code but lacks the "+".
		if !plus && len(digits) >= 10 {
			if alt, altErr := phonenumbers.Parse("+"+digits, region); altErr == nil && phonenumbers.IsValidNumber(alt) &&
				phonenumbers.GetRegionCodeForNumber(alt) != region {
				res.Fixes = append(res.Fixes, FixInternational)
				return res.ok(alt)
			}
		}
	}
	if err != nil {
		if err == phonenumbers.ErrInvalidCountryCode && plus {
			return res.fail(ProblemCountryCode, "The country code at the start of this number isn't right.")
		}
		return res.fail(ProblemInvalid, "This doesn't look like a phone number. Check it for typos.")
	}

	switch phonenumbers.IsPossibleNumberWithReason(num) {
	case phonenumbers.TOO_SHORT:
		return res.fail(ProblemTooShort, "This number is too short. Check that no digits are missing.")
	case phonenumbers.TOO_LONG:
		return res.fail(ProblemTooLong, "This number is too long. Check for extra digits.")
	case phonenumbers.IS_POSSIBLE_LOCAL_ONLY:
		return res.fail(ProblemTooShort, "This number is missing its area code.")
	case phonenumbers.INVALID_COUNTRY_CODE:
		if plus {
			return res.fail(ProblemCountryCode, "The country code at the start of this number isn't right.")
		}
		return res.fail(ProblemInvalid, "This doesn't look like a phone number. Check it for typos.")
	case phonenumbers.INVALID_LENGTH:
		return res.fail(ProblemInvalid, "This number has the wrong number of digits for "+countryOf(num, region)+".")
	}
	if !phonenumbers.IsValidNumber(num) {
		return res.fail(ProblemInvalid, "This isn't a valid "+countryOf(num, region)+
			" number. Check it for typos, or check the country selected above.")
	}
	return res.ok(num)
}

func countryOf(num *phonenumbers.PhoneNumber, fallback string) string {
	if r := phonenumbers.GetRegionCodeForNumber(num); r != "" {
		return CountryName(r)
	}
	if regions := phonenumbers.GetRegionCodesForCountryCode(int(num.GetCountryCode())); len(regions) > 0 {
		return CountryName(regions[0])
	}
	return CountryName(fallback)
}

func (r Result) fail(p Problem, msg string) Result {
	r.Problem, r.Message, r.E164, r.Display = p, msg, "", ""
	r.Fixes = nil
	return r
}

func (r Result) ok(num *phonenumbers.PhoneNumber) Result {
	r.E164 = phonenumbers.Format(num, phonenumbers.E164)
	r.Display = phonenumbers.Format(num, phonenumbers.INTERNATIONAL)
	r.Region = phonenumbers.GetRegionCodeForNumber(num)
	r.Message = describeFixes(r.Fixes)
	return r
}

func describeFixes(fixes []Fix) string {
	var parts []string
	for _, f := range fixes {
		switch f {
		case FixExcelNumber:
			parts = append(parts, "Restored from Excel's number format.")
		case FixIntlPrefix:
			parts = append(parts, "Changed the leading 00 to +.")
		case FixExtension:
			parts = append(parts, "Removed an extension, which WhatsApp doesn't use.")
		case FixExtraText:
			parts = append(parts, "Removed words around the number.")
		case FixInternational:
			parts = append(parts, "Read as an international number with its country code.")
		}
	}
	return strings.Join(parts, " ")
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

// Country is one entry in the country picker.
type Country struct {
	Code string `json:"code"` // "US"
	Name string `json:"name"` // "United States"
	Dial string `json:"dial"` // "+1"
}

// Countries lists every region phone numbers can belong to, by name.
func Countries() []Country {
	regions := phonenumbers.GetSupportedRegions()
	out := make([]Country, 0, len(regions))
	for code := range regions {
		out = append(out, Country{Code: code, Name: CountryName(code),
			Dial: "+" + strconv.Itoa(phonenumbers.GetCountryCodeForRegion(code))})
	}
	slices.SortFunc(out, func(a, b Country) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// ValidRegion reports whether code is a supported region such as "US".
func ValidRegion(code string) bool {
	return phonenumbers.GetSupportedRegions()[code]
}
