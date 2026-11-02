package people

import (
	"testing"
)

func TestReviewDeduplicatesKeepingTheFirst(t *testing.T) {
	rows := []Row{
		{Row: 2, Name: "Aisha Rahman", Phone: "(212) 555-0101"},
		{Row: 3, Name: "", Phone: "+1 212 555 0102"},
		{Row: 4, Name: "Aisha R.", Phone: "212.555.0101"},
		{Row: 5, Name: "Omar Ali", Phone: "12125550102"},
		{Row: 6, Name: "Fatima Noor", Phone: "212-555-0101"},
	}
	r := ReviewRows(rows, "US", "", nil)
	byRow := map[int]Person{}
	for _, p := range r.People {
		byRow[p.Row] = p
	}
	if p := byRow[2]; p.Status != Ready || len(p.AlsoRows) != 2 || p.AlsoRows[0] != 4 || p.AlsoRows[1] != 6 {
		t.Errorf("row 2 = %+v", p)
	}
	if p := byRow[4]; p.Status != Duplicate || p.DuplicateOf != 2 || p.Note == "" {
		t.Errorf("row 4 = %+v", p)
	}
	// The kept row borrows a name from its duplicate when it has none.
	if p := byRow[3]; p.Name != "Omar Ali" {
		t.Errorf("row 3 name = %q, want the duplicate's name", p.Name)
	}
	if r.Counts.Total != 5 || r.Counts.Ready != 2 || r.Counts.Duplicates != 3 {
		t.Errorf("counts = %+v", r.Counts)
	}
}

func TestReviewStatuses(t *testing.T) {
	rows := []Row{
		{Row: 1, Name: "Ready", Phone: "212-555-0101"},
		{Row: 2, Name: "Fixed", Phone: "1.2125550103E+10"},
		{Row: 3, Name: "Damaged", Phone: "1.21256E+10"},
		{Row: 4, Name: "Missing", Phone: ""},
		{Row: 5, Name: "Me", Phone: "+1 212 555 0100"},
		{Row: 6, Name: "No WhatsApp", Phone: "646-555-0106"},
		{Row: 7, Name: "Unchecked", Phone: "646-555-0107"},
	}
	lookup := func(phone string) (bool, bool) {
		switch phone {
		case "+16465550106":
			return false, true
		case "+12125550101", "+12125550103":
			return true, true
		}
		return false, false
	}
	r := ReviewRows(rows, "US", "+12125550100", lookup)
	want := []Status{Ready, Fixed, Attention, Attention, Self, NotOnWhatsApp, Ready}
	for i, p := range r.People {
		if p.Status != want[i] {
			t.Errorf("row %d (%s): status = %s, want %s (%s)", p.Row, p.Name, p.Status, want[i], p.Note)
		}
	}
	if r.People[0].OnWhatsApp == nil || !*r.People[0].OnWhatsApp {
		t.Error("checked number not marked as on WhatsApp")
	}
	if r.People[6].OnWhatsApp != nil {
		t.Error("unchecked number should have OnWhatsApp == nil")
	}
	c := r.Counts
	if c.Ready != 3 || c.Fixed != 1 || c.Attention != 2 || c.NotOnWhatsApp != 1 || c.Unchecked != 1 {
		t.Errorf("counts = %+v", c)
	}
}
