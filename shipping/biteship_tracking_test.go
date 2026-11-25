package shipping

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// trackingServer records the path the client actually requested.
func trackingServer(t *testing.T) (*httptest.Server, *string) {
	t.Helper()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"success":true,"id":"bite-1","status":"confirmed"}`)
	}))
	t.Cleanup(server.Close)

	return server, &gotPath
}

func TestTrackingRejectsPathTraversal(t *testing.T) {
	server, gotPath := trackingServer(t)
	client := NewBiteship("secret-key", server.URL, Origin{})

	if _, err := client.Tracking("../../v1/orders"); err == nil {
		t.Fatal("Tracking accepted a traversal id")
	}
	if *gotPath != "" {
		t.Fatalf("provider was called at %q, want no call", *gotPath)
	}
}

func TestTrackingRejectsQueryAndSpaces(t *testing.T) {
	server, gotPath := trackingServer(t)
	client := NewBiteship("secret-key", server.URL, Origin{})

	for _, id := range []string{"abc?x=1", "abc#frag", "abc def", ""} {
		if _, err := client.Tracking(id); err == nil {
			t.Fatalf("Tracking accepted %q", id)
		}
	}
	if *gotPath != "" {
		t.Fatalf("provider was called at %q, want no call", *gotPath)
	}
}

func TestTrackingAcceptsProviderID(t *testing.T) {
	server, gotPath := trackingServer(t)
	client := NewBiteship("secret-key", server.URL, Origin{})

	result, err := client.Tracking("bite_123-abc")
	if err != nil {
		t.Fatalf("Tracking: %v", err)
	}
	if result.ID != "bite-1" {
		t.Fatalf("id = %q", result.ID)
	}
	if want := "/v1/orders/bite_123-abc"; *gotPath != want {
		t.Fatalf("path = %q, want %q", *gotPath, want)
	}
}
