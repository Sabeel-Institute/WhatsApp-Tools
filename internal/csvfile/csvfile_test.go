package csvfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

func mustParse(t *testing.T, name, data string) *Table {
	t.Helper()
	tb, err := Parse(name, []byte(data))
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return tb
}

func labels(tb *Table) []string {
	out := make([]string, len(tb.Columns))
	for i, c := range tb.Columns {
		out[i] = c.Label
	}
	return out
}

func TestDelimiters(t *testing.T) {
	cases := map[string]struct {
		data  string
		delim string
	}{
		"comma":     {"Name,Phone\nAisha,212-555-0101\nOmar,212-555-0102\n", "comma"},
		"semicolon": {"Name;Phone;Amount\nAisha;212-555-0101;12,50\nOmar;212-555-0102;7,00\n", "semicolon"},
		"tab":       {"Name\tPhone\nAisha, Jr.\t212-555-0101\nOmar\t212-555-0102\n", "tab"},
		"pipe":      {"Name|Phone\nAisha|212-555-0101\nOmar|212-555-0102\n", "pipe"},
	}
	for name, c := range cases {
		tb := mustParse(t, name+".csv", c.data)
		if tb.Delimiter != c.delim {
			t.Errorf("%s: delimiter = %s, want %s", name, tb.Delimiter, c.delim)
		}
		if len(tb.Rows) != 2 || tb.Rows[0].Cells[0] == "" {
			t.Errorf("%s: rows = %+v", name, tb.Rows)
		}
	}
}

func TestExcelSepLine(t *testing.T) {
	tb := mustParse(t, "x.csv", "sep=;\nName;Phone\nAisha;212,555,0101\n")
	if tb.Delimiter != "semicolon" || len(tb.Columns) != 2 || tb.Rows[0].Cells[1] != "212,555,0101" {
		t.Fatalf("got %+v", tb)
	}
	if tb.Rows[0].Line != 3 {
		t.Errorf("line = %d, want 3 (as in the spreadsheet)", tb.Rows[0].Line)
	}
}

func TestBOMQuotesBlankLinesWhitespace(t *testing.T) {
	data := "\uFEFF\"Name\", \"Phone\" ,Notes\n\n\"Khan, Aisha\",  (212) 555-0101  ,\"She said \"\"hi\"\"\"\n,,\n  \n\"Omar\",212-555-0102,\"two\nlines\"\n"
	tb := mustParse(t, "x.csv", data)
	if got := strings.Join(labels(tb), "|"); got != "Name|Phone|Notes" {
		t.Errorf("labels = %q", got)
	}
	if len(tb.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (blank rows skipped)", len(tb.Rows))
	}
	r := tb.Rows[0]
	if r.Cells[0] != "Khan, Aisha" || r.Cells[1] != "(212) 555-0101" || r.Cells[2] != `She said "hi"` {
		t.Errorf("row = %q", r.Cells)
	}
	if r.Line != 3 {
		t.Errorf("line = %d, want 3", r.Line)
	}
	if tb.Rows[1].Cells[2] != "two\nlines" {
		t.Errorf("multi-line cell = %q", tb.Rows[1].Cells[2])
	}
}

func TestEncodings(t *testing.T) {
	text := "Name;Telefon\nJosé Muñoz;+34 612 34 56 78\nZoë Weiß;+49 1512 3456789\n"
	cp, err := charmap.Windows1252.NewEncoder().String(text)
	if err != nil {
		t.Fatal(err)
	}
	tb := mustParse(t, "x.csv", cp)
	if tb.Encoding != "Windows-1252" || tb.Rows[0].Cells[0] != "José Muñoz" || tb.Rows[1].Cells[0] != "Zoë Weiß" {
		t.Errorf("windows-1252: %s %+v", tb.Encoding, tb.Rows)
	}

	data, err := os.ReadFile("../../testdata/excel-unicode-text.tsv")
	if err != nil {
		t.Fatal(err)
	}
	tb, err = Parse("excel-unicode-text.tsv", data)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Encoding != "UTF-16" || tb.Delimiter != "tab" || len(tb.Rows) != 4 || tb.Rows[0].Cells[0] != "Safa Rahman" {
		t.Errorf("utf-16: %s %s %+v", tb.Encoding, tb.Delimiter, tb.Rows)
	}
}

func TestNoHeaderRow(t *testing.T) {
	tb := mustParse(t, "x.csv", "Aisha Rahman,(212) 555-0101,aisha@example.com\nOmar Ali,212-555-0102,omar@example.com\n")
	if tb.HasHeader {
		t.Fatal("first row of data taken as a header")
	}
	if got := strings.Join(labels(tb), "|"); got != "Column A|Column B|Column C" {
		t.Errorf("labels = %q", got)
	}
	if len(tb.Rows) != 2 || tb.Rows[0].Line != 1 {
		t.Errorf("rows = %+v", tb.Rows)
	}
	// A header without familiar words is still recognised when the data
	// below it is clearly different.
	tb = mustParse(t, "x.csv", "Wer,Nummer\nAisha,212-555-0101\nOmar,212-555-0102\n")
	if !tb.HasHeader {
		t.Error("header with unfamiliar words not recognised")
	}
}

func TestColumnLetters(t *testing.T) {
	for i, want := range map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 701: "ZZ", 702: "AAA"} {
		if got := columnLetters(i); got != want {
			t.Errorf("columnLetters(%d) = %s, want %s", i, got, want)
		}
	}
}

func TestDuplicateAndEmptyHeaders(t *testing.T) {
	tb := mustParse(t, "x.csv", "Name,Phone,Phone,,Email\nA,212-555-0101,646-555-0102,x,a@example.com\n")
	if got := strings.Join(labels(tb), "|"); got != "Name|Phone|Phone (2)|Column D|Email" {
		t.Errorf("labels = %q", got)
	}
	// Columns empty everywhere (trailing delimiters) are dropped.
	tb = mustParse(t, "x.csv", "Name,Phone,,\nA,212-555-0101,,\n")
	if len(tb.Columns) != 2 {
		t.Errorf("columns = %v", labels(tb))
	}
}

func TestRejectsWrongFiles(t *testing.T) {
	cases := map[string][]byte{
		"empty.csv":    nil,
		"blank.csv":    []byte(" \n\n "),
		"book.xlsx":    []byte("PK\x03\x04rest of a zip"),
		"old.xls":      {0xD0, 0xCF, 0x11, 0xE0, 1, 2},
		"report.pdf":   []byte("%PDF-1.7"),
		"binary.csv":   {'a', 0, 'b', 0, 'c'},
		"misnamed.csv": []byte("PK\x03\x04"),
	}
	for name, data := range cases {
		_, err := Parse(name, data)
		var fe *Error
		if !errors.As(err, &fe) || fe.Message == "" {
			t.Errorf("%s: err = %v, want a user-facing error", name, err)
		}
	}
}

func TestSuggestOnSampleExports(t *testing.T) {
	type want struct {
		phone, mode, name, first, last string
		header                         bool
	}
	cases := map[string]want{
		"eventbrite-attendees.csv":  {phone: "Cell Phone", mode: "split", first: "First Name", last: "Last Name", header: true},
		"stripe-payments.csv":       {phone: "Customer Phone", mode: "full", name: "Customer Name", header: true},
		"google-form-responses.csv": {phone: "WhatsApp number (with country code, please)", mode: "full", name: "Full Name", header: true},
		"semicolon-windows1252.csv": {phone: "Telefon", mode: "full", name: "Name", header: true},
		"no-header.csv":             {phone: "Column B", mode: "full", name: "Column A"},
		"excel-damaged.csv":         {phone: "Mobile", mode: "full", name: "Student Name", header: true},
		"excel-unicode-text.tsv":    {phone: "Phone Number", mode: "full", name: "Attendee", header: true},
		"quoted-utf8-bom.csv":       {phone: "Attendee Phone", mode: "full", name: "Attendee Name", header: true},
		"late-registrants.csv":      {phone: "Phone", mode: "full", name: "Name", header: true},
	}
	files, _ := filepath.Glob("../../testdata/*")
	if len(files) != len(cases) {
		t.Errorf("testdata has %d files, test covers %d", len(files), len(cases))
	}
	for name, w := range cases {
		data, err := os.ReadFile(filepath.Join("../../testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		tb, err := Parse(name, data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if tb.HasHeader != w.header {
			t.Errorf("%s: header = %v, want %v", name, tb.HasHeader, w.header)
		}
		s := Suggest(tb)
		lab := func(i int) string {
			if i < 0 {
				return ""
			}
			return tb.Columns[i].Label
		}
		got := want{phone: lab(s.Phone), mode: s.NameMode, name: lab(s.Name), first: lab(s.First), last: lab(s.Last), header: tb.HasHeader}
		if got != w {
			t.Errorf("%s:\n got %+v\nwant %+v", name, got, w)
		}
	}
}

func TestSuggestNeverPicksEmailsDatesOrAmounts(t *testing.T) {
	// Headers that mention "number" or "contact" but hold other data.
	data := "Contact Email,Order Number,Date,Amount Paid,Zip\n" +
		"a@example.com,10002345,2026-09-01,$1,200.00,10001\n" +
		"b@example.com,10002346,2026-09-02,$45.00,11201\n"
	s := Suggest(mustParse(t, "x.csv", data))
	if s.Phone != -1 {
		t.Errorf("suggested column %d as phone", s.Phone)
	}
	// A phone column with an unhelpful header wins on its values.
	data = "Ref,Info,Email\nA1,(212) 555-0101,a@example.com\nA2,646.555.0102,b@example.com\nA3,+44 7911 123456,c@example.com\n"
	if s := Suggest(mustParse(t, "x.csv", data)); s.Phone != 1 {
		t.Errorf("phone = %d, want 1 (the column of numbers)", s.Phone)
	}
}

func TestHeaderWords(t *testing.T) {
	cases := map[string]string{
		"WhatsApp Number": "whatsapp whats app number",
		"FirstName":       "first name first name",
		"phone_number":    "phone number",
		"Attendee #":      "attendee",
		"E-mail":          "e mail",
	}
	for in, want := range cases {
		if got := strings.Join(headerWords(in), " "); got != want {
			t.Errorf("headerWords(%q) = %q, want %q", in, got, want)
		}
	}
}
