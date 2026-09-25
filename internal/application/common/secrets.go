package common

type SecretBox interface {
	Seal(plain string) ([]byte, error)
	Open(sealed []byte) (string, error)
}
