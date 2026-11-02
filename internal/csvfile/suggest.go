package csvfile

import (
	"regexp"
	"strings"
	"unicode"
)

// Suggestion is the column mapping we propose; the user confirms or
// changes it. Column indexes are -1 when nothing fits.
type Suggestion struct {
	Phone    int    `json:"phone"`
	NameMode string `json:"nameMode"` // "none", "full" or "split"
	Name     int    `json:"name"`
	First    int    `json:"first"`
	Last     int    `json:"last"`
}

var (
	emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	dateRes = []*regexp.Regexp{
		regexp.MustCompile(`^\d{4}[-/.]\d{1,2}[-/.]\d{1,2}([ T].*)?$`),
		regexp.MustCompile(`^\d{1,2}[-/.]\d{1,2}[-/.]\d{2,4}([ T].*)?$`),
		regexp.MustCompile(`(?i)^\d{1,2}[ -](jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*[ -,]+\d{2,4}`),
		regexp.MustCompile(`(?i)^(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*\.? \d{1,2},? \d{2,4}`),
		regexp.MustCompile(`^\d{1,2}:\d{2}(:\d{2})?( ?[ap]m)?$`),
	}
	amountRe   = regexp.MustCompile(`^[-+]?[$€£¥₹]?\s?-?\d{1,3}([,.]?\d{3})*([.,]\d{1,2})?\s?(usd|eur|gbp|cad)?$`)
	moneyRe    = regexp.MustCompile(`[$€£¥₹]|(?i)\b(usd|eur|gbp|cad)\b`)
	sciRe      = regexp.MustCompile(`^\+?\d[.,]\d+[eE]\+?\d{1,2}$`)
	phoneChars = regexp.MustCompile(`^[+(]?[\d\s().\-/]+$`)
	nameRe     = regexp.MustCompile(`^[\p{L}][\p{L}\p{M}'’.\- ]*$`)
)

func isEmail(v string) bool { return emailRe.MatchString(v) }

func isDate(v string) bool {
	for _, re := range dateRes {
		if re.MatchString(v) {
			return true
		}
	}
	return false
}

func isAmount(v string) bool {
	if moneyRe.MatchString(v) {
		return true
	}
	// Short numbers with decimals (12.50, 1,200.00) are amounts; phone
	// numbers have at least seven digits and no two-digit decimal tail.
	return amountRe.MatchString(v) && digitCount(v) < 7
}

func digitCount(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}

// looksLikePhone is deliberately loose: it only ranks columns.
func looksLikePhone(v string) bool {
	v = strings.TrimSpace(strings.TrimPrefix(v, "'"))
	if v == "" || isEmail(v) || isDate(v) {
		return false
	}
	if sciRe.MatchString(v) {
		return true // a number Excel turned into scientific notation
	}
	low := strings.ToLower(v)
	for _, p := range []string{"tel:", "cell:", "mobile:", "phone:"} {
		low = strings.TrimPrefix(low, p)
	}
	if i := strings.IndexAny(low, "x#"); i > 0 {
		low = low[:i] // extension
	}
	low = strings.TrimSpace(low)
	if !phoneChars.MatchString(low) {
		return false
	}
	n := digitCount(low)
	return n >= 7 && n <= 15
}

func looksLikeName(v string) bool {
	return nameRe.MatchString(v) && len(strings.Fields(v)) >= 1 && len(v) <= 60
}

// headerWords lowercases a header and splits it into words, also splitting
// joined forms such as "FirstName" and "phone_number".
func headerWords(h string) []string {
	var b strings.Builder
	prevLower := false
	for _, r := range h {
		switch {
		case unicode.IsUpper(r) && prevLower:
			b.WriteRune(' ')
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(' ')
		}
		prevLower = unicode.IsLower(r)
	}
	words := strings.Fields(b.String())
	// Split common run-together headers.
	var out []string
	for _, w := range words {
		switch w {
		case "firstname", "fname":
			out = append(out, "first", "name")
		case "lastname", "lname", "surname":
			out = append(out, "last", "name")
		case "fullname":
			out = append(out, "full", "name")
		case "phonenumber", "mobilenumber", "cellphone", "cellnumber", "telephone", "whatsappnumber":
			out = append(out, "phone")
		default:
			out = append(out, w)
		}
	}
	return out
}

func hasAny(words []string, set ...string) bool {
	for _, w := range words {
		for _, s := range set {
			if w == s {
				return true
			}
		}
	}
	return false
}

var (
	phoneWords    = []string{"phone", "mobile", "cell", "whatsapp", "tel", "telephone", "number", "contact", "wa"}
	notPhoneWords = []string{"email", "mail", "date", "amount", "price", "total", "id", "order", "zip", "postal",
		"postcode", "ticket", "qty", "quantity", "age", "year", "code", "transaction", "invoice", "reference", "ref",
		"name", "address", "time", "fee", "paid", "payment", "count"}
	headerKeywords = []string{"name", "phone", "mobile", "cell", "email", "mail", "first", "last", "date", "amount",
		"ticket", "order", "id", "address", "city", "state", "zip", "country", "total", "price", "qty", "quantity",
		"status", "gender", "age", "notes", "note", "comments", "registration", "registered", "payment", "paid",
		"time", "number", "contact", "whatsapp", "attendee", "participant", "buyer", "surname", "given", "family",
		"course", "class", "company", "type", "tel", "telephone", "student", "member", "fee", "created"}
)

// looksLikeHeader decides whether the first row names the columns.
func looksLikeHeader(first []string, rest [][]string) bool {
	nonEmpty, keyword := 0, 0
	for _, c := range first {
		if c == "" {
			continue
		}
		nonEmpty++
		if looksLikePhone(c) || isEmail(c) || isDate(c) || isAmount(c) {
			return false // headers never contain data like this
		}
		if hasAny(headerWords(c), headerKeywords...) {
			keyword++
		}
	}
	if nonEmpty == 0 {
		return false
	}
	if keyword > 0 {
		return true
	}
	// No familiar header words: it's a header if some column's first cell is
	// plain text while the values below it are mostly data.
	for i, c := range first {
		if c == "" {
			continue
		}
		data, total := 0, 0
		for _, r := range rest {
			if i >= len(r) || r[i] == "" {
				continue
			}
			total++
			if looksLikePhone(r[i]) || isEmail(r[i]) || isDate(r[i]) || isAmount(r[i]) {
				data++
			}
		}
		if total > 0 && float64(data)/float64(total) >= 0.6 {
			return true
		}
	}
	// A first row whose values repeat further down is data, not a header.
	for i, c := range first {
		for _, r := range rest {
			if i < len(r) && r[i] == c && c != "" {
				return false
			}
		}
	}
	return len(rest) == 0
}

type colStats struct {
	phone, email, date, amount, name float64
}

func stats(t *Table, col int) colStats {
	var s colStats
	n := 0
	for _, r := range t.Rows {
		if n >= 300 {
			break
		}
		v := r.Cells[col]
		if v == "" {
			continue
		}
		n++
		switch {
		case isEmail(v):
			s.email++
		case isDate(v):
			s.date++
		case looksLikePhone(v):
			s.phone++
		case isAmount(v):
			s.amount++
		case looksLikeName(v):
			s.name++
		}
	}
	if n == 0 {
		return s
	}
	f := float64(n)
	return colStats{s.phone / f, s.email / f, s.date / f, s.amount / f, s.name / f}
}

// Suggest proposes which columns hold the phone number and the name.
func Suggest(t *Table) Suggestion {
	sg := Suggestion{Phone: -1, NameMode: "none", Name: -1, First: -1, Last: -1}
	st := make([]colStats, len(t.Columns))
	words := make([][]string, len(t.Columns))
	for i, c := range t.Columns {
		st[i] = stats(t, i)
		words[i] = headerWords(c.Header)
	}

	// Phone: the values decide; the header breaks ties and rescues columns
	// with many blanks or typos. Never a column of emails, dates or amounts.
	bestScore := 0.0
	for i := range t.Columns {
		s := st[i]
		if s.email >= 0.5 || s.date >= 0.5 || s.amount >= 0.5 {
			continue
		}
		pos := hasAny(words[i], phoneWords...)
		neg := hasAny(words[i], notPhoneWords...) && !hasAny(words[i], "phone", "mobile", "cell", "whatsapp", "tel")
		if neg {
			continue
		}
		score := 3 * s.phone
		if pos {
			score += 1.5
		}
		if (s.phone >= 0.5 || (pos && s.phone >= 0.2)) && score > bestScore {
			sg.Phone, bestScore = i, score
		}
	}

	// Names: prefer the attendee's own name over the buyer's.
	rank := func(w []string) int {
		switch {
		case hasAny(w, "attendee", "participant", "student", "registrant", "member"):
			return 3
		case hasAny(w, "buyer", "customer", "purchaser", "billing", "contact", "parent", "guardian"):
			return 1
		}
		return 2
	}
	excluded := func(i int) bool {
		return i == sg.Phone || st[i].email >= 0.5 || st[i].phone >= 0.5 ||
			hasAny(words[i], "email", "mail", "user", "username", "company", "course", "event", "ticket", "middle",
				"nickname", "file", "emergency", "organization", "school")
	}
	pick := func(match func(w []string) bool) int {
		best, bestRank := -1, 0
		for i := range t.Columns {
			if excluded(i) || !match(words[i]) {
				continue
			}
			if r := rank(words[i]); r > bestRank {
				best, bestRank = i, r
			}
		}
		return best
	}
	first := pick(func(w []string) bool { return hasAny(w, "first", "given", "forename") })
	last := pick(func(w []string) bool { return hasAny(w, "last", "family", "surname") })
	full := pick(func(w []string) bool {
		return !hasAny(w, "first", "given", "forename", "last", "family", "surname") &&
			(hasAny(w, "name", "attendee", "participant", "registrant", "student", "buyer", "customer"))
	})
	switch {
	case first >= 0 && last >= 0 && (full < 0 || rank(words[first]) >= rank(words[full])):
		sg.NameMode, sg.First, sg.Last = "split", first, last
	case full >= 0:
		sg.NameMode, sg.Name = "full", full
	case first >= 0:
		sg.NameMode, sg.Name = "full", first
	default:
		// No telling header: a column of names is still recognisable.
		best := 0.0
		for i := range t.Columns {
			if !excluded(i) && st[i].name >= 0.7 && st[i].name > best {
				sg.NameMode, sg.Name, best = "full", i, st[i].name
			}
		}
	}
	return sg
}
