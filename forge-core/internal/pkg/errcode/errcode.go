package errcode

import "net/http"

// Error codes — 4-digit, grouped by category
const (
	// 1xxx: Input / validation
	InvalidInput = 1001
	MissingField = 1002

	// 2xxx: Authentication
	Unauthorized       = 2001
	TokenExpired       = 2002
	TokenRevoked       = 2003
	InvalidCredentials = 2004

	// 3xxx: Authorization
	Forbidden = 3001

	// 4xxx: Resource
	NotFound = 4001
	Conflict = 4002

	// 5xxx: Internal
	InternalError = 5001
	ExternalAPI   = 5002
)

// AppError is a structured application error with a code.
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func (e *AppError) Error() string {
	return e.Message
}

func (e *AppError) HTTPStatus() int {
	switch {
	case e.Code >= 1000 && e.Code < 2000:
		return http.StatusBadRequest
	case e.Code >= 2000 && e.Code < 3000:
		return http.StatusUnauthorized
	case e.Code >= 3000 && e.Code < 4000:
		return http.StatusForbidden
	case e.Code >= 4000 && e.Code < 5000:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
