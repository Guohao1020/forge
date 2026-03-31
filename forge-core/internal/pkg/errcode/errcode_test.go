package errcode

import "testing"

func TestAppErrorImplementsError(t *testing.T) {
	err := New(InvalidInput, "bad field")
	if err.Error() != "bad field" {
		t.Fatalf("got %q, want %q", err.Error(), "bad field")
	}
	if err.Code != InvalidInput {
		t.Fatalf("got code %d, want %d", err.Code, InvalidInput)
	}
}

func TestAppErrorHTTPStatus(t *testing.T) {
	tests := []struct {
		code   int
		status int
	}{
		{InvalidInput, 400},
		{Unauthorized, 401},
		{Forbidden, 403},
		{NotFound, 404},
		{InternalError, 500},
	}
	for _, tt := range tests {
		err := New(tt.code, "test")
		if err.HTTPStatus() != tt.status {
			t.Errorf("code %d: got HTTP %d, want %d", tt.code, err.HTTPStatus(), tt.status)
		}
	}
}
