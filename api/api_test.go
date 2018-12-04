package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// String is a helper routine that allocates a new string value
// to store v and returns a pointer to it.
func String(v string) *string { return &v }

func TestGetPublicKey(t *testing.T) {
	client, mux, _, teardown := setup()
	defer teardown()

	mux.HandleFunc("/email/jane@example.com/key", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"armoredPublicKey": "---- BEGIN PGP PUBLIC KEY..."}`)
	})

	armoredPublicKey, response, err := client.GetPublicKey("jane@example.com")

	fmt.Printf("armoredPublicKey: %s\n", armoredPublicKey)

	if err != nil {
		t.Errorf("GetPublicKey returned error: %v\nresponse: %v", err, response)
	}

	want := "---- BEGIN PGP PUBLIC KEY..."
	if armoredPublicKey != want {
		t.Errorf("GetPublicKey(\"jane@example.com\") returned %+v, want %+v", armoredPublicKey, want)
	}
}

// setup sets up a test HTTP server along with a fluidkeysServer.Client that is
// configured to talk to that test server. Tests should register handlers on
// mux which provide mock responses for the API method being tested.
func setup() (client *Client, mux *http.ServeMux, serverURL string, teardown func()) {
	// mux is the HTTP request multiplexer used with the test server.
	mux = http.NewServeMux()
	apiHandler := http.NewServeMux()
	apiHandler.Handle("/", mux)

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(apiHandler)

	// client is the Fluidkeys Server client being tested and is
	// configured to use test server.
	client = NewClient()
	url, _ := url.Parse(server.URL + "/")
	client.BaseURL = url

	return client, mux, server.URL, server.Close
}

func testMethod(t *testing.T, r *http.Request, want string) {
	if got := r.Method; got != want {
		t.Errorf("Request method: %v, want %v", got, want)
	}
}