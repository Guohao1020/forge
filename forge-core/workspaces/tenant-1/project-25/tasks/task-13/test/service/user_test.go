package tests

import (
  "testing"
  "github.com/stretchr/testify/assert"
  "github.com/yourorg/yourproject/service"
  "github.com/yourorg/yourproject/model"
)

func TestUserService_Create(t *testing.T) {
  mockService := &service.userService{}
  user := model.User{Name: "John Doe", Email: "john@example.com", Password: "password123"}
  createdUser, err := mockService.Create(user)
  assert.NoError(t, err)
  assert.Equal(t, user.Name, createdUser.Name)
  assert.Equal(t, user.Email, createdUser.Email)
}

func TestUserService_Create_InvalidEmail(t *testing.T) {
  mockService := &service.userService{}
  user := model.User{Name: "John Doe", Email: "invalid-email", Password: "password123"}
  _, err := mockService.Create(user)
  assert.Error(t, err)
}

func TestUserService_Delete(t *testing.T) {
  mockService := &service.userService{}
  err := mockService.Delete(1)
  assert.NoError(t, err)
}

func TestUserService_Delete_NonExistentUser(t *testing.T) {
  mockService := &service.userService{}
  err := mockService.Delete(999)
  assert.Error(t, err)
}