package real

import (
	"context"
	"fmt"
	"testing"

	"go.mau.fi/whatsmeow"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

func TestErrorClassification(t *testing.T) {
	cases := map[error]whatsapp.Kind{
		whatsmeow.ErrNotConnected:                        whatsapp.KindNotConnected,
		whatsmeow.ErrIQDisconnected:                      whatsapp.KindNotConnected,
		whatsmeow.ErrIQTimedOut:                          whatsapp.KindTemporary,
		context.DeadlineExceeded:                         whatsapp.KindTemporary,
		whatsmeow.ErrNotLoggedIn:                         whatsapp.KindLoggedOut,
		whatsmeow.ErrIQRateOverLimit:                     whatsapp.KindRateLimited,
		whatsmeow.ErrIQForbidden:                         whatsapp.KindNotAdmin,
		whatsmeow.ErrIQNotFound:                          whatsapp.KindNotFound,
		whatsmeow.ErrIQNotAcceptable:                     whatsapp.KindInvalid,
		whatsmeow.ErrIQServiceUnavailable:                whatsapp.KindTemporary,
		whatsmeow.ErrIQInternalServerError:               whatsapp.KindTemporary,
		whatsmeow.ErrIQResourceLimit:                     whatsapp.KindGroupFull,
		fmt.Errorf("wrapped: %w", whatsmeow.ErrIQLocked): whatsapp.KindUnknown,
	}
	for err, want := range cases {
		if got := whatsapp.KindOf(classify("op", err)); got != want {
			t.Errorf("%v: kind = %s, want %s", err, got, want)
		}
	}
	if classify("op", nil) != nil {
		t.Error("nil error classified")
	}
}

func TestParticipantOutcomes(t *testing.T) {
	cases := map[int]whatsapp.AddOutcome{
		0: whatsapp.AddAdded, 200: whatsapp.AddAdded, 403: whatsapp.AddPrivacy, 408: whatsapp.AddRecentlyLeft,
		409: whatsapp.AddAlreadyMember, 404: whatsapp.AddNotOnWhatsApp, 401: whatsapp.AddFailed, 500: whatsapp.AddFailed,
	}
	for code, want := range cases {
		if got := outcomeOf(code); got != want {
			t.Errorf("code %d: %s, want %s", code, got, want)
		}
	}
}

func TestVersionAndDisplayName(t *testing.T) {
	if n := pairDisplayName(); n == "" || n[:6] != "Chrome" {
		t.Errorf("pair display name %q", n)
	}
}
