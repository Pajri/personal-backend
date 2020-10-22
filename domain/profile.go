package domain

type Profile struct {
	ProfileID string  `json:"-"`
	FullName  string  `json:"full_name"`
	AccountID string  `json:"-"`
	Account   Account `json:"-"`
}

type IProfileRepository interface {
	InsertProfile(profile Profile) error
}