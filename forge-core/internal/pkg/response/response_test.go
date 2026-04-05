package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	OK(c, gin.H{"id": 42})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result Result
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.Code != 0 {
		t.Errorf("expected code 0, got %d", result.Code)
	}
	if result.Message != "ok" {
		t.Errorf("expected message 'ok', got %s", result.Message)
	}
}

func TestOK_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	OK(c, nil)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result Result
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Code != 0 {
		t.Errorf("expected code 0, got %d", result.Code)
	}
}

func TestFail(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Fail(c, http.StatusBadRequest, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var result Result
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Code != -1 {
		t.Errorf("expected code -1, got %d", result.Code)
	}
	if result.Message != "invalid input" {
		t.Errorf("expected message 'invalid input', got %s", result.Message)
	}
}

func TestFail_ServerError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Fail(c, http.StatusInternalServerError, "database down")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestResultJSON(t *testing.T) {
	r := Result{Code: 0, Message: "ok", Data: map[string]int{"count": 5}}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed Result
	json.Unmarshal(data, &parsed)
	if parsed.Code != 0 || parsed.Message != "ok" {
		t.Error("round-trip failed")
	}
}
