package api

import (
  "encoding/json"
  "net/http"
  "github.com/yourorg/yourproject/service"
  "github.com/yourorg/yourproject/model"
)

func RegisterUserHandler(w http.ResponseWriter, r *http.Request) {
  var user model.User
  if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
  }
  if err := user.Validate(); err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
  }
  userService := service.NewUserService()
  createdUser, err := userService.Create(user)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusCreated)
  json.NewEncoder(w).Encode(createdUser)
}

func LoginUserHandler(w http.ResponseWriter, r *http.Request) {
  var user model.User
  if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
  }
  if user.Email == "" || user.Password == "" {
    http.Error(w, "email and password are required", http.StatusBadRequest)
    return
  }
  // Simulate login check
  if user.Email == "john@example.com" && user.Password == "password123" {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"message": "User logged in successfully"})
  } else {
    http.Error(w, "wrong password", http.StatusUnauthorized)
  }
}