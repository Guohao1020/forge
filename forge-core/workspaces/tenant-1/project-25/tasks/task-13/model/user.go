package model

import "fmt"

type User struct {
  ID       int    `json:"id"`
  Name     string `json:"name"`
  Email    string `json:"email"`
  Password string `json:"password"`
}

func (u *User) Validate() error {
  if u.Name == "" || u.Email == "" || u.Password == "" {
    return fmt.Errorf("all fields are required")
  }
  // Add more validation logic as needed, e.g., email format
  return nil
}