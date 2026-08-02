package hasher

// Crypto 定义了密码哈希器的接口，支持密码哈希和验证操作。
type Crypto interface {
	Hash(password string) (string, error)
	Verify(hashedPassword, password string) error
}
