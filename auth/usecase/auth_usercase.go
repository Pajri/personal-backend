package usecase

import (
	"crypto/rand"
	"errors"
	"fmt"
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
	fmt.Println(string(account.Salt))
	if err != nil {
		return nil, nil, err
	}

	account.Password, err = uc.hashPassword([]byte(account.Password), account.Salt)
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
