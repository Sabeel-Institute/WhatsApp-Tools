// Package people is the shared "who is on the list" layer every workflow
// builds on: it normalizes and de-duplicates the rows picked from a
// spreadsheet, and remembers which numbers have WhatsApp.
package people

import (
	"fmt"
	"strings"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/phone"
)

// Row is one person as picked from the spreadsheet.
type Row struct {
	Row   int    `json:"row"`   // the spreadsheet row number, for the user
	Name  string `json:"name"`  // may be empty
	Phone string `json:"phone"` // as written in the file (or edited)
}

// Status of a reviewed person.
type Status string

const (
	Ready         Status = "ready"     // usable as written
	Fixed         Status = "fixed"     // usable after an automatic repair
	Attention     Status = "attention" // needs the user to fix the number
	Duplicate     Status = "duplicate" // same number as an earlier row
	Self          Status = "self"      // the connected account itself
	NotOnWhatsApp Status = "not_on_whatsapp"
)

// Usable reports whether people with this status can be added.
func (s Status) Usable() bool { return s == Ready || s == Fixed }

// Person is a reviewed row.
type Person struct {
	Row     int    `json:"row"`
	Name    string `json:"name"`
	Raw     string `json:"raw"`
	Phone   string `json:"phone,omitempty"`   // international format
	Display string `json:"display,omitempty"` // formatted for reading
	Status  Status `json:"status"`
	// Note explains the status in plain words.
	Note string `json:"note,omitempty"`
	// DuplicateOf is the row this one repeats.
	DuplicateOf int `json:"duplicateOf,omitempty"`
	// AlsoRows lists later rows merged into this one.
	AlsoRows []int `json:"alsoRows,omitempty"`
	// OnWhatsApp is nil until checked.
	OnWhatsApp *bool `json:"onWhatsApp"`
}

// Counts summarize a review. Ready includes Fixed.
type Counts struct {
	Total         int `json:"total"`
	Ready         int `json:"ready"`
	Fixed         int `json:"fixed"`
	Attention     int `json:"attention"`
	Duplicates    int `json:"duplicates"`
	NotOnWhatsApp int `json:"notOnWhatsApp"`
	Unchecked     int `json:"unchecked"` // usable but not yet checked
}

// Review is the result of reviewing a list.
type Review struct {
	People []Person `json:"people"`
	Counts Counts   `json:"counts"`
}

// Lookup returns what is known about a number's WhatsApp account.
type Lookup func(phone string) (onWhatsApp bool, known bool)

// ReviewRows normalizes rows with region as the default country, merges
// duplicates (keeping the first), flags the connected account's own number,
// and applies known WhatsApp checks.
func ReviewRows(rows []Row, region, selfPhone string, lookup Lookup) Review {
	out := Review{People: make([]Person, 0, len(rows))}
	first := map[string]int{} // phone -> index in out.People
	for _, r := range rows {
		n := phone.Normalize(r.Phone, region)
		p := Person{Row: r.Row, Name: strings.TrimSpace(r.Name), Raw: r.Phone}
		switch {
		case !n.OK():
			p.Status, p.Note = Attention, n.Message
		default:
			p.Phone, p.Display = n.E164, n.Display
			p.Status = Ready
			if len(n.Fixes) > 0 {
				p.Status, p.Note = Fixed, n.Message
			}
		}
		if p.Phone != "" {
			if i, dup := first[p.Phone]; dup {
				kept := &out.People[i]
				kept.AlsoRows = append(kept.AlsoRows, p.Row)
				if kept.Name == "" && p.Name != "" {
					kept.Name = p.Name
				}
				p.Status, p.DuplicateOf = Duplicate, kept.Row
				p.Note = fmt.Sprintf("Same number as row %d", kept.Row)
				if kept.Name != "" {
					p.Note += " (" + kept.Name + ")"
				}
				p.Note += ". Only the first is kept."
			} else {
				first[p.Phone] = len(out.People)
			}
		}
		out.People = append(out.People, p)
	}
	for i := range out.People {
		p := &out.People[i]
		if p.Status.Usable() && selfPhone != "" && p.Phone == selfPhone {
			p.Status, p.Note = Self, "This is the WhatsApp account this app is connected with. It will be in the group already."
		}
		if p.Status.Usable() && lookup != nil {
			if on, known := lookup(p.Phone); known {
				v := on
				p.OnWhatsApp = &v
				if !on {
					p.Status, p.Note = NotOnWhatsApp, "This number doesn't have a WhatsApp account."
				}
			}
		}
		out.Counts.Total++
		switch p.Status {
		case Ready:
			out.Counts.Ready++
		case Fixed:
			out.Counts.Ready++
			out.Counts.Fixed++
		case Attention:
			out.Counts.Attention++
		case Duplicate:
			out.Counts.Duplicates++
		case NotOnWhatsApp:
			out.Counts.NotOnWhatsApp++
		}
		if p.Status.Usable() && p.OnWhatsApp == nil {
			out.Counts.Unchecked++
		}
	}
	return out
}
