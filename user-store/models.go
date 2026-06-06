package userstore

import (
	"errors"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("models: invalid credentials")
	ErrDuplicateUserName  = errors.New("models: duplicate username")
)

type User struct {
	ID           int64
	Name         string
	IsAdmin      bool
	CreatedAtUTC time.Time
	LastLoginUTC time.Time
	Theme        string
}

type UserDatabase struct {
	ID      int64
	UserID  int64
	Path    string
	Version int
}
