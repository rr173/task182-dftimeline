package artifact

import (
	"errors"
	"testing"

	"task182-dftimeline/internal/model"
)

func TestTimezoneMissingTimestampIsRejected(t *testing.T) {
	_, _, err := parseTimestampWithZone("2026-08-23T10:00:00")
	if err == nil || !errors.Is(err, model.ErrTimeZoneMissing) {
		t.Fatalf("expected timezone-missing error, got %v", err)
	}
}
