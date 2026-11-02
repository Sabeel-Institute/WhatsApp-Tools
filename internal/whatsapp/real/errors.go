package real

import (
	"context"
	"errors"

	"go.mau.fi/whatsmeow"

	"github.com/Sabeel-Institute/WhatsApp-Tools/internal/whatsapp"
)

// classify maps a whatsmeow error onto the app's error kinds.
func classify(op string, err error) error {
	if err == nil {
		return nil
	}
	return whatsapp.NewError(kindOf(err), op, err)
}

func kindOf(err error) whatsapp.Kind {
	var disc *whatsmeow.DisconnectedError
	var iq *whatsmeow.IQError
	switch {
	case errors.Is(err, context.Canceled):
		return whatsapp.KindUnknown
	case errors.Is(err, whatsmeow.ErrNotLoggedIn):
		return whatsapp.KindLoggedOut
	case errors.Is(err, whatsmeow.ErrNotConnected), errors.As(err, &disc):
		return whatsapp.KindNotConnected
	case errors.Is(err, whatsmeow.ErrIQTimedOut), errors.Is(err, context.DeadlineExceeded):
		return whatsapp.KindTemporary
	case errors.As(err, &iq):
		return kindOfIQ(iq.Code)
	}
	return whatsapp.KindUnknown
}

// kindOfIQ maps the HTTP-like status codes WhatsApp returns for requests.
func kindOfIQ(code int) whatsapp.Kind {
	switch code {
	case 401, 403:
		return whatsapp.KindNotAdmin
	case 404:
		return whatsapp.KindNotFound
	case 400, 405, 406:
		return whatsapp.KindInvalid
	case 419:
		return whatsapp.KindGroupFull
	case 429:
		return whatsapp.KindRateLimited
	case 408, 500, 502, 503, 504, 530:
		return whatsapp.KindTemporary
	}
	return whatsapp.KindUnknown
}

// outcomeOf maps the per-person status code in a participant update.
func outcomeOf(code int) whatsapp.AddOutcome {
	switch code {
	case 0, 200:
		return whatsapp.AddAdded
	case 403:
		return whatsapp.AddPrivacy
	case 408:
		return whatsapp.AddRecentlyLeft
	case 409:
		return whatsapp.AddAlreadyMember
	case 404:
		return whatsapp.AddNotOnWhatsApp
	}
	return whatsapp.AddFailed
}
