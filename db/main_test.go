// main_test.go
//go:build go1.24

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"testing"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// roundTripFunc lets us stub out http.Client.Transport.
type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestReadSheetRows_PropagateProjectStatus(t *testing.T) {
	// Arrange: fake sheet data with header + rows where Project/Status go blank
	fakeValues := [][]interface{}{
		{"Project", "Status", "Other"},
		{"P1", "S1", "X1"},
		{"", "S2", "X2"},
		{"", "", "X3"},
		{"P2", "", "X4"},
	}
	vr := &sheets.ValueRange{Values: fakeValues}
	payload, err := json.Marshal(vr)
	if err != nil {
		t.Fatalf("failed to marshal fake ValueRange: %v", err)
	}

	// Stub HTTP client to always return our fake ValueRange JSON
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(payload)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}
		}),
	}

	// Create a Sheets service using our fake client
	srv, err := sheets.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatalf("sheets.NewService() error: %v", err)
	}

	// Act
	rows, err := readSheetRows(context.Background(), srv, "ignoredID", "ignoredRange")
	if err != nil {
		t.Fatalf("readSheetRows returned error: %v", err)
	}

	// Assert: blank cells should inherit previous values
	want := []map[string]string{
		{"Project": "P1", "Status": "S1", "Other": "X1"},
		{"Project": "P1", "Status": "S2", "Other": "X2"},
		{"Project": "P1", "Status": "S2", "Other": "X3"},
		{"Project": "P2", "Status": "S2", "Other": "X4"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("readSheetRows = %#v; want %#v", rows, want)
	}
}
