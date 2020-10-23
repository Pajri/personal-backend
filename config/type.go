package config

type Configuration struct {
	DB                DBConfig
	SMTP              SMTP
	Host              string
	EmailVerification EmailVerificationConfig
	ResetPassword     ResetPasswordConfig
}

type DBConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	DbName   string
}

type SMTP struct {
	From     string
	Host     string
	Port     int
	Username string
	Password string
}

type EmailVerificationConfig struct {
	Subject string
}

type ResetPasswordConfig struct {
	Subject string
}
