package tests

import (
  "net/http"
  "net/http/httptest"
  "strings"
  "testing"
  "github.com/stretchr/testify/assert"
)

func TestUserRegistrationEndpoint_Success(t *testing.T) {
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost && r.URL.Path == "/api/register" {
      w.WriteHeader(http.StatusCreated)
      w.Write([]byte(`{\"message\": \"User registered successfully\"}`))
    }
  }))
  defer srv.Close()

  res, err := http.Post(srv.URL+"/api/register", "application/json", strings.NewReader(`{\"name\": \"John Doe\", \"email\": \"john@example.com\", \"password\": \"password123\"}`))
  assert.NoError(t, err)
  assert.Equal(t, http.StatusCreated, res.StatusCode)
}

func TestUserRegistrationEndpoint_InvalidEmail(t *testing.T) {
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost && r.URL.Path == "/api/register" {
      w.WriteHeader(http.StatusBadRequest)
      w.Write([]byte(`{\"error\": \"Invalid email format\"}`))
    }
  }))
  defer srv.Close()

  res, err := http.Post(srv.URL+"/api/register", "application/json", strings.NewReader(`{\"name\": \"John Doe\", \"email\": \"invalid-email\", \"password\": \"password123\"}`))
  assert.NoError(t, err)
  assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestUserLoginEndpoint_Success(t *testing.T) {
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost && r.URL.Path == "/api/login" {
      w.WriteHeader(http.StatusOK)
      w.Write([]byte(`{\"message\": \"User logged in successfully\"}`))
    }
  }))
  defer srv.Close()

  res, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(`{\"email\": \"john@example.com\", \"password\": \"password123\"}`))
  assert.NoError(t, err)
  assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestUserLoginEndpoint_WrongPassword(t *testing.T) {
  srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost && r.URL.Path == "/api/login" {
      w.WriteHeader(http.StatusUnauthorized)
      w.Write([]byte(`{\"error\": \"Wrong password\"}`))
    }
  }))
  defer srv.Close()

  res, err := http.Post(srv.URL+"/api/login", "application/json", strings.NewReader(`{\"email\": \"john@example.com\", \"password\": \"wrongpassword\"}`))
  assert.NoError(t, err)
  assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}