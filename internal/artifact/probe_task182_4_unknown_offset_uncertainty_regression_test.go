package artifact

import "testing"

func TestUnknownOffsetUsesSevenDayUncertainty(t *testing.T) {
	const want int64 = 7 * 24 * 3600 * 1000
	if UnknownOffsetUncertaintyMS != want { t.Fatalf("expected %d ms, got %d", want, UnknownOffsetUncertaintyMS) }
}
