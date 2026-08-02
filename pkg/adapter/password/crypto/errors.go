package crypto

// 本文件属于密码哈希组件，约束算法配置、密码长度校验、哈希生成和验证失败模式。

import "errors"

// 预定义错误（Sentinel Errors）
// 可使用 errors.Is() 判断
var (
	// ErrInvalidPassword 无效密码错误
	ErrInvalidPassword = errors.New("invalid password")

	// ErrPasswordTooShort 密码过短错误
	ErrPasswordTooShort = errors.New("password too short")

	// ErrPasswordTooLong 密码过长错误
	ErrPasswordTooLong = errors.New("password too long")

	// ErrHashingFailed 密码哈希失败错误
	ErrHashingFailed = errors.New("hashing failed")

	// ErrVerificationFailed 密码验证失败错误
	ErrVerificationFailed = errors.New("verification failed")

	// ErrInvalidConfig 无效配置错误
	ErrInvalidConfig = errors.New("invalid config")

	// ErrInvalidAlgorithm 无效算法错误
	ErrInvalidAlgorithm = errors.New("invalid algorithm")
)

// 错误消息模板常量
// 用于 fmt.Errorf() 包装错误
const (
	// ErrMsgHashingFailed 哈希失败消息模板
	ErrMsgHashingFailed = "failed to hash password: %w"

	// ErrMsgVerificationFailed 验证失败消息模板
	ErrMsgVerificationFailed = "failed to verify password: %w"

	// ErrMsgInvalidConfig 无效配置消息模板
	ErrMsgInvalidConfig = "invalid config: %w"

	// ErrMsgPasswordTooShort 密码过短消息模板
	ErrMsgPasswordTooShort = "password too short: minimum length is %d"

	// ErrMsgPasswordTooLong 密码过长消息模板
	ErrMsgPasswordTooLong = "password too long: maximum length is %d"

	// ErrMsgInvalidAlgorithm 无效算法消息模板
	ErrMsgInvalidAlgorithm = "invalid algorithm %q: supported algorithms are %v"
)
