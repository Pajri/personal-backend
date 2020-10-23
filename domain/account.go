package domain

type Account struct {
	AccountID  string `json:"-"`
	Password   string `json:"-"`
	Email      string `json:"email"`
	Salt       []byte `json:"-"`
	EmailToken string `json:"-"`
	IsVerified bool   `json:"-"`
}

type IAccountRepository interface {
	GetAccount(filter AccountFilter) (*Account, error)
	InsertAccount(account Account) (*Account, error)
	UpdateIsVerified(accountID string, isVerified bool) error
	UpdateSaltAndPassword(account Account) error
}

type AccountFilter struct {
	Email string
}
