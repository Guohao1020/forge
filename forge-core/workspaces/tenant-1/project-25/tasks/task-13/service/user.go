package service

import (
  "errors"
  "fmt"
  "github.com/yourorg/yourproject/model"
)

type UserService interface {
  Create(user model.User) (model.User, error)
  Delete(id int) error
}

type userService struct {}

var (
  ErrUserNotFound = errors.New("user not found")
)

func NewUserService() UserService {
  return &userService{}
}

func (s *userService) Create(user model.User) (model.User, error) {
  if err := user.Validate(); err != nil {
    return model.User{}, err
  }
  // Save user to the database
  // For now, we just simulate the save operation
  user.ID = 1 // Simulate auto-incremented ID
  return user, nil
}

func (s *userService) Delete(id int) error {
  // Check if user exists and delete
  if id == 999 {
    return ErrUserNotFound
  }
  // For now, we just simulate the delete operation
  return nil
}