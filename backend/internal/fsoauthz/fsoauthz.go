package fsoauthz

type Logger interface {
	Infow(msg string, kv ...interface{})
}
