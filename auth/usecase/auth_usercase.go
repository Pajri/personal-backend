package usecase

import (
	"crypto/rand"
	"errors"
	"io"
	"log"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/pajri/personal-backend/adapter/cerror"
	"github.com/pajri/personal-backend/domain"
	"github.com/pajri/personal-backend/global"
	"golang.org/x/crypto/bcrypt"
)

const SALT_BYTES = 32

type AuthUsecase struct {
	accountRepo domain.IAccountRepository
	profileRepo domain.IProfileRepository
}

func NewAuthUsecase(accountRepository domain.IAccountRepository, profileRepository domain.IProfileRepository) *AuthUsecase {
	return &AuthUsecase{
		accountRepo: accountRepository,
		profileRepo: profileRepository,
	}
}

func (uc AuthUsecase) Login(account domain.Account) (string, error) {
	filter := domain.AccountFilter{Email: account.Email}
	regAccount, err := uc.accountRepo.GetAccount(filter)
	if err != nil {
		return "", err
	}

	_, ok := uc.comparePassword([]byte(account.Password), regAccount.Salt, []byte(regAccount.Password))
	if !ok {
		cerr := cerror.NewAndPrintWithTag("LGU03",
			errors.New("incorrect password for email :"+account.Email),
			global.FRIENDLY_INVALID_USNME_PASSWORD)
		cerr.Type = cerror.TYPE_UNAUTHORIZED

		return "", cerr
	}

	if regAccount != nil {
		claims := jwt.MapClaims{}
		claims["authorized"] = true
		claims["user_id"] = regAccount.Email
		claims["exp"] = time.Now().Add(12 * time.Minute).Unix()

		accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		token, err := accessToken.SignedString([]byte(os.Getenv("JWT_SECRET")))
		if err != nil {
			return "", cerror.NewAndPrintWithTag("LGU01", err, global.FRIENDLY_MESSAGE)
		}

		return token, nil
	}

	userNilErr := cerror.NewAndPrintWithTag("LGU02", errors.New("user nil"), global.FRIENDLY_INVALID_USNME_PASSWORD)
	return "", userNilErr
}

func (uc AuthUsecase) SignUp(account domain.Account, profile domain.Profile) (*domain.Account, *domain.Profile, error) {
	//create salt
	var err error
	account.Salt, err = uc.generateSalt()
	if err != nil {
		return nil, nil, err
	}

	account.Password, err = uc.hashPassword([]byte(account.Password), account.Salt)
	if err != nil {
		return nil, nil, err
	}

	account.EmailToken, err = uc.createEmailVerificationToken(account)
	if err != nil {
		return nil, nil, err
	}

	insertedAccount, err := uc.accountRepo.InsertAccount(account)
	if err != nil {
		return nil, nil, err
	}

	if insertedAccount != nil {
		profile.AccountID = insertedAccount.AccountID
		err = uc.profileRepo.InsertProfile(profile)
	} else {
		log.Println("[SGU00] insertedAccount is nil")
	}

	return insertedAccount, &profile, nil
}

func (uc AuthUsecase) VerifyEmail(token string) error {
	payload, err := uc.parseJWT(token)
	if err != nil {
		return cerror.NewAndPrintWithTag("VEA00", err, global.FRIENDLY_INVALID_TOKEN)
	}

	expire := payload["exp"].(float64)
	email := payload["email"].(string)

	//start convert unix time to time
	unixInt := int64(expire)
	expTime := time.Unix(unixInt, 0)

	if time.Now().After(expTime) {
		return cerror.NewAndPrintWithTag("VEA02", err, global.FRIENDLY_TOKEN_EXPIRED)
	}
	//end convert unix time to time

	//get account
	filter := domain.AccountFilter{Email: email}
	account, err := uc.accountRepo.GetAccount(filter)
	if err != nil {
		return err
	}

	//verify email
	err = uc.accountRepo.UpdateIsVerified(account.AccountID, true)
	if err != nil {
		return err
	}

	return nil
}

func (uc AuthUsecase) generateSalt() ([]byte, error) {
	salt := make([]byte, SALT_BYTES)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil {
		return nil, cerror.NewAndPrintWithTag("GSA00", err, global.FRIENDLY_MESSAGE)
	}
	return salt, nil
}

func (uc AuthUsecase) hashPassword(password, salt []byte) (string, error) {
	var saltedPassword []byte
	saltedPassword = append(saltedPassword, password...)
	saltedPassword = append(saltedPassword, salt...)

	hash, err := bcrypt.GenerateFromPassword(saltedPassword, bcrypt.MinCost)
	if err != nil {
		return "", cerror.NewAndPrintWithTag("HPA00", err, global.FRIENDLY_MESSAGE)
	}

	return string(hash), nil
}

func (uc AuthUsecase) comparePassword(passwordInput, salt, storedPassword []byte) (error, bool) {
	var saltedPassword []byte
	saltedPassword = append(saltedPassword, passwordInput...)
	saltedPassword = append(saltedPassword, salt...)

	err := bcrypt.CompareHashAndPassword(storedPassword, saltedPassword)
	if err != nil {
		return cerror.NewAndPrintWithTag("CPA00", err, ""), false
	}

	return nil, true
}

func (uc AuthUsecase) createEmailVerificationToken(account domain.Account) (string, error) {
	claims := jwt.MapClaims{}
	claims["email"] = account.Email
	claims["exp"] = time.Now().Add(24 * time.Hour).Unix()

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := accessToken.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return "", cerror.NewAndPrintWithTag("CEV99", err, global.FRIENDLY_MESSAGE)
	}

	return token, nil
}

func (uc AuthUsecase) parseJWT(tokenString string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})
	return claims, err
}
