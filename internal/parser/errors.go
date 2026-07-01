package parser

import "fmt"

type ErrorCode string

const (
	ErrParseFailed               ErrorCode = "PARSE_FAILED"
	ErrInvalidEncoding           ErrorCode = "PARSE_FAILED_INVALID_ENCODING"
	ErrSkippedSymlinkCycle       ErrorCode = "SKIPPED_SYMLINK_CYCLE"
	ErrSkippedPermission         ErrorCode = "SKIPPED_PERMISSION_DENIED"
	ErrRepoTooLarge              ErrorCode = "REPO_TOO_LARGE"
	ErrPanicRecovered            ErrorCode = "PANIC_RECOVERED"
	ErrCannotStaticallyDetermine ErrorCode = "CANNOT_STATICALLY_DETERMINE"
	ErrUnknownCardinality        ErrorCode = "UNKNOWN_CARDINALITY" // Reviewer only, non-blocking

	// Agent-path state resolution — blocking. Caller must not continue past these.
	ErrStateFileUnreadable ErrorCode = "STATE_FILE_UNREADABLE"
	ErrStateFileMalformed  ErrorCode = "STATE_FILE_MALFORMED"
	ErrResourceNotInState  ErrorCode = "RESOURCE_NOT_IN_STATE"
)

type ParseError struct {
	Code    ErrorCode
	File    string
	Line    int
	Message string
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s: %s:%d: %s", e.Code, e.File, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s: %s", e.Code, e.File, e.Message)
}

func NewParseError(code ErrorCode, file string, line int, message string) *ParseError {
	return &ParseError{Code: code, File: file, Line: line, Message: message}
}
