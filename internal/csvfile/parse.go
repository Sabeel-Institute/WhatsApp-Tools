// Package csvfile reads the registration spreadsheets people export from
// payment and sign-up systems: any of the usual delimiters, with or without a
// header row, in UTF-8, UTF-16 or Windows-1252, with the stray whitespace,
// blank lines and quoting those exports contain.
package csvfile

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// MaxFileSize is the largest file accepted.
const MaxFileSize = 10 << 20

// MaxRows is the most data rows accepted.
const MaxRows = 20000

// Error is a problem with the file, explained for the user.
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

func userError(msg string) error { return &Error{Message: msg} }

// Column describes one column.
type Column struct {
	Index int `json:"index"`
	// Label is the header text, or "Column A" when the file has no header.
	Label string `json:"label"`
	// Header is the raw header text ("" without a header row).
	Header string `json:"header"`
	// Samples are up to five different values, for the column picker.
	Samples []string `json:"samples"`
}

// Row is one data row.
type Row struct {
	// Line is the row number as a spreadsheet program shows it.
	Line  int      `json:"line"`
	Cells []string `json:"cells"`
}

// Table is a parsed file.
type Table struct {
	FileName  string   `json:"fileName"`
	Encoding  string   `json:"encoding"`
	Delimiter string   `json:"delimiter"` // "comma", "semicolon", "tab" or "pipe"
	HasHeader bool     `json:"hasHeader"`
	Columns   []Column `json:"columns"`
	Rows      []Row    `json:"rows"`
}

// Headers returns the raw header texts.
func (t *Table) Headers() []string {
	out := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		out[i] = c.Header
	}
	return out
}

var delimiters = []struct {
	r    rune
	name string
}{{',', "comma"}, {';', "semicolon"}, {'\t', "tab"}, {'|', "pipe"}}

// Parse reads a spreadsheet file.
func Parse(fileName string, data []byte) (*Table, error) {
	if len(data) == 0 || len(bytes.TrimSpace(data)) == 0 {
		return nil, userError("This file is empty. Please check that you chose the right file.")
	}
	if len(data) > MaxFileSize {
		return nil, userError("This file is larger than 10 MB, which is far more than a registration list needs. Please check that you chose the right file.")
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	switch {
	case bytes.HasPrefix(data, []byte("PK\x03\x04")) || ext == ".xlsx" || ext == ".numbers" || ext == ".ods":
		return nil, userError("This is a workbook file, not a spreadsheet file (.csv). In Excel or Google Sheets, choose File → Save As (or Download) → CSV, then choose that file here.")
	case bytes.HasPrefix(data, []byte{0xD0, 0xCF, 0x11, 0xE0}) || ext == ".xls":
		return nil, userError("This is an older Excel file (.xls). In Excel, choose File → Save As → CSV, then choose that file here.")
	case bytes.HasPrefix(data, []byte("%PDF")):
		return nil, userError("This is a PDF, not a spreadsheet file (.csv). Export the list from your registration system as CSV instead.")
	}

	text, encoding, err := decode(data)
	if err != nil {
		return nil, err
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	delim := rune(0)
	// Excel's "sep=;" first line names the delimiter.
	if first, rest, ok := strings.Cut(text, "\n"); ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(first)), "sep=") {
		if d, size := utf8.DecodeRuneInString(strings.TrimSpace(first)[4:]); size > 0 {
			delim = d
			text = "\n" + rest // keep line numbers aligned with the file
		}
	}
	if delim == 0 {
		delim = detectDelimiter(text)
	}
	records, err := readAll(text, delim)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, userError("This file has no rows we can read. Please check that you chose the right file.")
	}

	t := &Table{FileName: filepath.Base(fileName), Encoding: encoding, Delimiter: "comma"}
	for _, d := range delimiters {
		if d.r == delim {
			t.Delimiter = d.name
		}
	}

	width := 0
	for _, r := range records {
		width = max(width, len(r.Cells))
	}
	// Drop columns that are empty everywhere (trailing delimiters).
	keep := make([]bool, width)
	for _, r := range records {
		for i, c := range r.Cells {
			if c != "" {
				keep[i] = true
			}
		}
	}
	for i := range records {
		cells := make([]string, 0, width)
		for c := 0; c < width; c++ {
			if !keep[c] {
				continue
			}
			v := ""
			if c < len(records[i].Cells) {
				v = records[i].Cells[c]
			}
			cells = append(cells, v)
		}
		records[i].Cells = cells
	}
	width = len(records[0].Cells)

	t.HasHeader = looksLikeHeader(records[0].Cells, cellsOf(records[1:]))
	data0 := records
	if t.HasHeader {
		data0 = records[1:]
	}
	if len(data0) > MaxRows {
		return nil, userError(fmt.Sprintf("This file has %d rows, more than the %d we can handle at once. Please split it into smaller files.", len(data0), MaxRows))
	}
	t.Rows = data0
	if t.Rows == nil {
		t.Rows = []Row{}
	}

	seen := map[string]int{}
	for i := 0; i < width; i++ {
		col := Column{Index: i, Label: "Column " + columnLetters(i)}
		if t.HasHeader {
			col.Header = records[0].Cells[i]
			if col.Header != "" {
				col.Label = col.Header
			}
		}
		if n := seen[strings.ToLower(col.Label)]; n > 0 {
			col.Label = fmt.Sprintf("%s (%d)", col.Label, n+1)
		}
		seen[strings.ToLower(col.Label)]++
		col.Samples = samples(t.Rows, i, 5)
		t.Columns = append(t.Columns, col)
	}
	return t, nil
}

func cellsOf(rows []Row) [][]string {
	out := make([][]string, len(rows))
	for i, r := range rows {
		out[i] = r.Cells
	}
	return out
}

func samples(rows []Row, col, n int) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, r := range rows {
		v := r.Cells[col]
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
		if len(out) == n {
			break
		}
	}
	return out
}

// columnLetters returns spreadsheet-style column letters: A..Z, AA, AB...
func columnLetters(i int) string {
	s := ""
	for i++; i > 0; i = (i - 1) / 26 {
		s = string(rune('A'+(i-1)%26)) + s
	}
	return s
}

// decode returns the file as text, detecting UTF-16 and UTF-8 byte-order
// marks and falling back to Windows-1252 for text that isn't valid UTF-8.
func decode(data []byte) (string, string, error) {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return string(data[3:]), "UTF-8", nil
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}), bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		dec := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
		out, err := dec.Bytes(data)
		if err != nil {
			return "", "", userError("We couldn't read the text in this file. Please save it again as \"CSV UTF-8\" and try again.")
		}
		return string(out), "UTF-16", nil
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", "", userError("This doesn't look like a spreadsheet file (.csv). Please export the list as CSV and try again.")
	}
	if utf8.Valid(data) {
		return string(data), "UTF-8", nil
	}
	out, err := charmap.Windows1252.NewDecoder().Bytes(data)
	if err != nil {
		return "", "", userError("We couldn't read the text in this file. Please save it again as \"CSV UTF-8\" and try again.")
	}
	return string(out), "Windows-1252", nil
}

// detectDelimiter picks the delimiter that splits the first rows into the
// most consistent number of fields.
func detectDelimiter(text string) rune {
	best, bestScore := ',', -1.0
	for _, d := range delimiters {
		recs, err := readSample(text, d.r, 50)
		if err != nil || len(recs) == 0 {
			continue
		}
		counts := map[int]int{}
		for _, r := range recs {
			counts[len(r)]++
		}
		mode, modeN := 0, 0
		for n, c := range counts {
			if c > modeN || (c == modeN && n > mode) {
				mode, modeN = n, c
			}
		}
		if mode < 2 {
			continue
		}
		score := float64(modeN)/float64(len(recs))*100 + float64(min(mode, 20))
		if score > bestScore {
			best, bestScore = d.r, score
		}
	}
	return best
}

func newReader(text string, delim rune) *csv.Reader {
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delim
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	return r
}

func readSample(text string, delim rune, n int) ([][]string, error) {
	r := newReader(text, delim)
	var out [][]string
	for len(out) < n {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if !blank(rec) {
			out = append(out, rec)
		}
	}
	return out, nil
}

func blank(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func readAll(text string, delim rune) ([]Row, error) {
	r := newReader(text, delim)
	var out []Row
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, userError("Part of this file couldn't be read. It may be damaged; try exporting it again.")
		}
		if blank(rec) {
			continue
		}
		line, _ := r.FieldPos(0)
		cells := make([]string, len(rec))
		for i, c := range rec {
			cells[i] = cleanCell(c)
		}
		out = append(out, Row{Line: line, Cells: cells})
		if len(out) > MaxRows+1 {
			return nil, userError(fmt.Sprintf("This file has more than %d rows, more than we can handle at once. Please split it into smaller files.", MaxRows))
		}
	}
	return out, nil
}

// cleanCell trims whitespace (including non-breaking spaces) and removes
// invisible formatting characters.
func cleanCell(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\u200b', '\u200c', '\u200d', '\u2060', '\ufeff', '\u200e', '\u200f':
			return -1
		case '\u00a0':
			return ' '
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}
