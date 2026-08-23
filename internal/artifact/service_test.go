package artifact

import (
	"errors"
	"testing"

	"task182-dftimeline/internal/model"
)

func TestParseTimestampWithZoneRequiresOffset(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{"offset +08:00", "2026-08-20T10:00:00+08:00", nil},
		{"zulu Z", "2026-08-20T10:00:00Z", nil},
		{"negative offset -05:00", "2026-08-20T10:00:00-05:00", nil},
		{"fractional with offset", "2026-08-20T10:00:00.123+08:00", nil},
		// 回归核心：不带时区的时间戳必须被拒绝，不得当作本地时间/UTC 接受。
		{"missing tz rejected", "2026-08-20T10:00:00", model.ErrTimeZoneMissing},
		{"missing tz with fractional rejected", "2026-08-20T10:00:00.000", model.ErrTimeZoneMissing},
		{"missing tz space-form rejected", "2026-08-20 10:00:00", model.ErrTimeZoneMissing},
		{"empty rejected", "", model.ErrTimeZoneMissing},
		{"garbage rejected", "not-a-timestamp", model.ErrTimeZoneMissing},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ms, offMin, err := parseTimestampWithZone(c.raw)
			if c.wantErr != nil {
				if err == nil {
					t.Fatalf("parseTimestampWithZone(%q): expected error, got ms=%d offMin=%d nil err", c.raw, ms, offMin)
				}
				if !errors.Is(err, c.wantErr) {
					t.Fatalf("parseTimestampWithZone(%q): expected %v, got %v", c.raw, c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTimestampWithZone(%q): unexpected error %v", c.raw, err)
			}
			if ms == 0 {
				t.Fatalf("parseTimestampWithZone(%q): got zero unix ms", c.raw)
			}
		})
	}
}

func TestParseTimestampWithZoneOffsetValues(t *testing.T) {
	// +08:00 -> 480 min
	if _, off, err := parseTimestampWithZone("2026-08-20T10:00:00+08:00"); err != nil || off != 480 {
		t.Fatalf("+08:00: off=%d err=%v want 480", off, err)
	}
	// Z -> 0 min
	if _, off, err := parseTimestampWithZone("2026-08-20T10:00:00Z"); err != nil || off != 0 {
		t.Fatalf("Z: off=%d err=%v want 0", off, err)
	}
	// -05:00 -> -300 min
	if _, off, err := parseTimestampWithZone("2026-08-20T10:00:00-05:00"); err != nil || off != -300 {
		t.Fatalf("-05:00: off=%d err=%v want -300", off, err)
	}
	// 同一时刻不同时区表示应得到相同的 Unix 毫秒。
	zMS, _, err := parseTimestampWithZone("2026-08-20T02:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	pMS, _, err := parseTimestampWithZone("2026-08-20T10:00:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	if zMS != pMS {
		t.Fatalf("Z and +08:00 of same instant differ: %d vs %d", zMS, pMS)
	}
}
