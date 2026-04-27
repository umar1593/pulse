package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeIngester struct {
	gotEvent Event
	err      error
}

func (f *fakeIngester) Insert(_ context.Context, e Event) error {
	f.gotEvent = e
	return f.err
}

func TestHandler_Ingest_Accepts202(t *testing.T) {
	ing := &fakeIngester{}
	h := NewHandler(ing)

	body := []byte(`{"user_id":"u-1","event_type":"page_view","properties":{"path":"/x"}}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))

	h.Ingest(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status: got %d want %d", rr.Code, http.StatusAccepted)
	}
	if ing.gotEvent.UserID != "u-1" || ing.gotEvent.EventType != "page_view" {
		t.Fatalf("event not propagated: %+v", ing.gotEvent)
	}
	var props map[string]string
	if err := json.Unmarshal(ing.gotEvent.Properties, &props); err != nil {
		t.Fatalf("properties: %v", err)
	}
	if props["path"] != "/x" {
		t.Fatalf("properties.path: got %q want %q", props["path"], "/x")
	}
}

func TestHandler_Ingest_RejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"empty body", ``},
		{"missing user_id", `{"event_type":"x"}`},
		{"missing event_type", `{"user_id":"u"}`},
		{"unknown field", `{"user_id":"u","event_type":"x","extra":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(&fakeIngester{})
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader([]byte(tc.body)))
			h.Ingest(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d want %d", rr.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandler_Ingest_PropagatesRepoError(t *testing.T) {
	ing := &fakeIngester{err: errors.New("boom")}
	h := NewHandler(ing)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/events",
		bytes.NewReader([]byte(`{"user_id":"u","event_type":"x"}`)))
	h.Ingest(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d", rr.Code, http.StatusInternalServerError)
	}
}
