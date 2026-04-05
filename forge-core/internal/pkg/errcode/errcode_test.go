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
		{MissingField, 400},
		{Unauthorized, 401},
		{TokenExpired, 401},
		{TokenRevoked, 401},
		{InvalidCredentials, 401},
		{Forbidden, 403},
		{NotFound, 404},
		{Conflict, 404},
		{InternalError, 500},
		{ExternalAPI, 500},
		{9999, 500}, // unknown code defaults to 500
	}
	for _, tt := range tests {
		err := New(tt.code, "test")
		if err.HTTPStatus() != tt.status {
			t.Errorf("code %d: got HTTP %d, want %d", tt.code, err.HTTPStatus(), tt.status)
		}
	}
}

func TestAppErrorConstants(t *testing.T) {
	// Verify all constants are in expected ranges
	if InvalidInput < 1000 || InvalidInput >= 2000 {
		t.Error("InvalidInput should be in 1xxx range")
	}
	if Unauthorized < 2000 || Unauthorized >= 3000 {
		t.Error("Unauthorized should be in 2xxx range")
	}
	if Forbidden < 3000 || Forbidden >= 4000 {
		t.Error("Forbidden should be in 3xxx range")
	}
	if NotFound < 4000 || NotFound >= 5000 {
		t.Error("NotFound should be in 4xxx range")
	}
	if InternalError < 5000 {
		t.Error("InternalError should be in 5xxx range")
	}
}

func TestAppErrorAsError(t *testing.T) {
	var err error = New(NotFound, "not found")
	if err.Error() != "not found" {
		t.Errorf("Error() = %q, want %q", err.Error(), "not found")
	}
}
