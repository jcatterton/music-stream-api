package service

type ExtHandler interface {
	ValidateToken(token string) error
}
