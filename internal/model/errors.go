// Package model 定义数字取证时间线冲突解析服务的领域实体、状态与错误。
package model

import (
	"errors"
	"fmt"
	"time"
)

// 领域错误。所有业务规则违反都通过这里的哨兵错误或包装错误表达，
// HTTP 层据此映射为 400/404/409/422。
var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrBadRequest      = errors.New("bad request")
	ErrInvalidState    = errors.New("invalid state transition")
	ErrTimeZoneMissing = errors.New("timestamp without timezone offset is rejected")
	ErrIntervalReversed = errors.New("interval reversed: earliest after latest")
	ErrHashMismatch    = errors.New("same artifact hash with different content")
	ErrCrossCase       = errors.New("cross-case reference is forbidden")
	ErrReportImmutable = errors.New("published report is immutable")
	ErrCycleUnbounded  = errors.New("cycle detection exceeded iteration bound")
)

// Wrap 包装领域错误并附加上下文。
func Wrap(err error, format string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// IsNotFound 判断错误是否为未找到。
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsConflict 判断错误是否为冲突。
func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }

// TimeNow 返回当前 UTC 时间，统一时间来源，便于测试。
var TimeNow = func() time.Time { return time.Now().UTC() }
