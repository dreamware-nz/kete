package cli

import (
	"github.com/dreamware-nz/kete/internal/store"
)

// withStore opens the default kete database, runs fn with it, and
// guarantees Close runs even if fn panics. The single error path keeps
// every subcommand from re-deriving the open/close dance.
func withStore(fn func(*store.DB) error) (err error) {
	db, openErr := store.OpenDefault()
	if openErr != nil {
		return openErr
	}
	defer func() {
		if cerr := db.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	return fn(db)
}
