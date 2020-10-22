package domain

type IAuthUsecase interface {
	Login(account Account) (string, error)
	SignUp(account Account, profile Profile) (*Account, *Profile, error)
}