package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	c := Cursor{TS: time.Date(2026, 10, 1, 8, 0, 0, 123456000, time.UTC), ID: 42}
	enc := c.Encode()
	if enc != "eyJwIjoiMjAyNi0xMC0wMVQwODowMDowMC4xMjM0NTZaIiwiaSI6NDJ9" {
		t.Fatalf("encoded cursor = %s", enc)
	}
	got, err := ParseCursor(httptest.NewRequest("GET", "/?cursor="+enc, nil))
	if err != nil || !got.TS.Equal(c.TS) || got.ID != 42 {
		t.Fatalf("parse = %+v, %v", got, err)
	}
	for _, bad := range []string{"!!!", "bm90LWpzb24", "eyJwIjoieCIsImkiOjF9"} {
		if _, err := ParseCursor(httptest.NewRequest("GET", "/?cursor="+bad, nil)); err == nil {
			t.Errorf("cursor %q should be rejected", bad)
		}
	}
}

func TestIfMatch(t *testing.T) {
	req := func(v string) *http.Request {
		r := httptest.NewRequest("PUT", "/", nil)
		if v != "" {
			r.Header.Set("If-Match", v)
		}
		return r
	}
	now := time.Now()
	if err := ParseIfMatch(req("")).Require(); err == nil {
		t.Fatal("missing If-Match should be required")
	}
	if err := ParseIfMatch(req("")).Check(3, now); err != nil {
		t.Fatal("absent If-Match passes Check")
	}
	for _, ok := range []string{`"v3"`, `"v1", "v3"`, `*`} {
		if err := ParseIfMatch(req(ok)).Check(3, now); err != nil {
			t.Errorf("%s should match v3: %v", ok, err)
		}
	}
	err := ParseIfMatch(req(`"v2"`)).Check(3, now)
	var e *Error
	if !errors.As(err, &e) || e.Status != http.StatusPreconditionFailed || e.Extra["current_etag"] != `"v3"` {
		t.Fatalf("stale If-Match: %v", err)
	}
}

func TestTimeFormat(t *testing.T) {
	ts := time.Date(2026, 10, 17, 8, 30, 0, 0, time.FixedZone("CST", 8*3600))
	b, _ := NewTime(ts).MarshalJSON()
	if string(b) != `"2026-10-17T00:30:00.000000Z"` {
		t.Fatalf("time = %s", b)
	}
}

func TestLikePattern(t *testing.T) {
	if got := LikePattern(`50%_off\`); got != `%50\%\_off\\%` {
		t.Fatalf("LikePattern = %q", got)
	}
}
