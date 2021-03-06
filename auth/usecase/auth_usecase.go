package usecase

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/pajri/personal-backend/adapter/cerror"
	"github.com/pajri/personal-backend/config"
	"github.com/pajri/personal-backend/domain"
	"github.com/pajri/personal-backend/global"
	"github.com/pajri/personal-backend/helper"
	"golang.org/x/crypto/bcrypt"
)

const SALT_BYTES = 32

type AuthUsecase struct {
	accountRepo domain.IAccountRepository
	profileRepo domain.IProfileRepository
	mailHelper  helper.IEMail
}

func NewAuthUsecase(accountRepository domain.IAccountRepository,
	profileRepository domain.IProfileRepository,
	_mailHelper helper.IEMail) domain.IAuthUsecase {
	return &AuthUsecase{
		accountRepo: accountRepository,
		profileRepo: profileRepository,
		mailHelper:  _mailHelper,
	}
}

func (uc AuthUsecase) Login(account domain.Account) (*helper.JWTWrapper, error) {
	filter := domain.AccountFilter{Email: account.Email}
	regAccount, err := uc.accountRepo.GetAccount(filter)
	if err != nil {
		return nil, err
	}

	err, ok := uc.comparePassword([]byte(account.Password), regAccount.Salt, []byte(regAccount.Password))
	if !ok || err != nil {
		cerr := cerror.NewAndPrintWithTag("LGU03",
			errors.New("incorrect password for email :"+account.Email),
			global.FRIENDLY_INVALID_USNME_PASSWORD)
		cerr.Type = cerror.TYPE_UNAUTHORIZED

		return nil, cerr
	}

	if regAccount != nil {
		if !regAccount.IsVerified {
			err := fmt.Errorf("email %s has not been verified", regAccount.Email)
			cerr := cerror.NewAndPrintWithTag("LGU04", err, global.FRIENDLY_EMAIL_NOT_VERIFIED)
			return nil, cerr
		}

		filterProfile := domain.ProfileFilter{AccountID: regAccount.AccountID}
		regProfile, err := uc.profileRepo.GetProfile(filterProfile)
		if err != nil {
			return nil, err
		}

		token, err := uc.createTokenPair(*regAccount, *regProfile)
		if err != nil {
			return nil, err
		}

		return token, nil
	}

	userNilErr := cerror.NewAndPrintWithTag("LGU02", errors.New("user nil"), global.FRIENDLY_INVALID_USNME_PASSWORD)
	return nil, userNilErr
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

	emailTokenClaims := jwt.MapClaims{}
	emailTokenClaims["account_id"] = uuid.New().String()
	emailTokenClaims["email"] = account.Email
	emailTokenClaims["exp"] = time.Now().Add(15 * time.Minute).Unix()

	jwtHelper := helper.JWTHelper{}
	account.EmailToken, err = jwtHelper.CreateToken(emailTokenClaims)
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
		if err != nil {
			return nil, nil, err
		}

	} else {
		log.Println("[SGU00] insertedAccount is nil")
	}

	msg := fmt.Sprintf(global.VERIFY_EMAIL_TEMPLATE, uc.generateEmailConfirmationUrl(*insertedAccount))
	to := []string{insertedAccount.Email}
	subject := config.Config.EmailVerification.Subject
	err = uc.mailHelper.SendMail(to, subject, msg)
	if err != nil {
		return nil, nil, err
	}

	return insertedAccount, &profile, nil
}

func (uc AuthUsecase) RefreshToken(refreshToken string) (*helper.JWTWrapper, error) {
	jwtHelper := helper.JWTHelper{}
	token, err := jwtHelper.ParseToken(refreshToken)
	if err != nil {
		//including expiration error
		//so, no need further check for token expiration
		//just handle in auth delivery
		return nil, err
	}
	mapClaims := token.Claims.(jwt.MapClaims)

	//validate token in redis
	rtRedis, _ := helper.RedisHelper.Get(mapClaims["refresh_uuid"].(string))
	if rtRedis == "" {
		//token is expired
		cerr := cerror.NewAndPrintWithTag("RTU00", errors.New("token_expired"), global.FRIENDLY_TOKEN_EXPIRED)
		cerr.Type = cerror.TYPE_EXPIRED
		return nil, cerr
	}

	//get account
	accountID := mapClaims["account_id"].(string)
	filter := domain.AccountFilter{AccountID: accountID}
	account, err := uc.accountRepo.GetAccount(filter)
	if err != nil {
		return nil, err
	}

	filterProfile := domain.ProfileFilter{AccountID: accountID}
	profile, err := uc.profileRepo.GetProfile(filterProfile)
	if err != nil {
		return nil, err
	}

	//create token
	tokenPair, err := uc.createTokenPair(*account, *profile)
	if err != nil {
		return nil, err
	}
	return tokenPair, nil
}

func (uc AuthUsecase) VerifyEmail(token string) error {
	jwtHelper := helper.JWTHelper{}
	parsedToken, err := jwtHelper.ParseToken(token)
	if err != nil {
		return cerror.NewAndPrintWithTag("VEA00", err, global.FRIENDLY_INVALID_TOKEN)
	}
	payload := parsedToken.Claims.(jwt.MapClaims)

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

func (uc AuthUsecase) ResetPassword(email string) error {
	claims := jwt.MapClaims{}
	claims["email"] = email
	claims["exp"] = time.Now().Add(24 * time.Hour).Unix()

	jwtHelper := helper.JWTHelper{}
	token, err := jwtHelper.CreateToken(claims)
	if err != nil {
		return err
	}

	filter := domain.AccountFilter{Email: email}
	account, err := uc.accountRepo.GetAccount(filter)
	if account != nil {
		if !account.IsVerified {
			cerr := cerror.NewAndPrintWithTag("RPA01", fmt.Errorf("email %s has not been verified", account.Email), global.FRIENDLY_EMAIL_NOT_VERIFIED)
			return cerr
		}

		var updateTokenInput domain.Account
		updateTokenInput.AccountID = account.AccountID
		updateTokenInput.PasswordToken = token
		err := uc.accountRepo.UpdatePasswordToken(updateTokenInput)
		if err != nil {
			return err
		}

		url := uc.generateResetPasswordUrl(token)

		msg := fmt.Sprintf(global.RESET_PASSWORD_TEMPLATE, url)
		to := []string{email}
		subject := config.Config.ResetPassword.Subject
		err = uc.mailHelper.SendMail(to, subject, msg)
		if err != nil {
			return err
		}

		return nil
	}

	err = fmt.Errorf("email %s is not found", email)
	cerr := cerror.NewAndPrintWithTag("RPA00", err, "")
	cerr.Type = cerror.TYPE_NOT_FOUND
	return cerr
}

func (uc AuthUsecase) ChangePassword(token, password string) error {
	jwtHelper := helper.JWTHelper{}
	parsedToken, err := jwtHelper.ParseToken(token)
	if err != nil {
		return cerror.NewAndPrintWithTag("CPW00", err, global.FRIENDLY_INVALID_TOKEN)
	}
	payload := parsedToken.Claims.(jwt.MapClaims)

	expire := payload["exp"].(float64)
	email := payload["email"].(string)

	//start convert unix time to time
	unixInt := int64(expire)
	expTime := time.Unix(unixInt, 0)

	if time.Now().After(expTime) {
		return cerror.NewAndPrintWithTag("CPW01", err, global.FRIENDLY_TOKEN_EXPIRED)
	}
	//end convert unix time to time

	//get account
	filter := domain.AccountFilter{Email: email}
	account, err := uc.accountRepo.GetAccount(filter)
	if err != nil {
		cerr := cerror.NewAndPrintWithTag("CPW02", err, global.FRIENDLY_INVALID_EMAIL)
		cerr.Type = cerror.TYPE_NOT_FOUND
		return cerr
	}

	if token != account.PasswordToken {
		errMsg := fmt.Sprintf("token is different from token from database. email : %s", account.Email)
		cerr := cerror.NewAndPrintWithTag("CPW03", errors.New(errMsg), global.FRIENDLY_INVALID_TOKEN)
		return cerr
	}

	account.Salt, err = uc.generateSalt()
	if err != nil {
		return err
	}

	account.Password, err = uc.hashPassword([]byte(password), account.Salt)
	if err != nil {
		return err
	}

	err = uc.accountRepo.UpdateSaltAndPassword(*account)
	if err != nil {
		return err
	}

	var updateTokenInput domain.Account
	updateTokenInput.AccountID = account.AccountID
	updateTokenInput.PasswordToken = ""
	err = uc.accountRepo.UpdatePasswordToken(updateTokenInput)
	if err != nil {
		return err
	}

	return nil
}

func (uc AuthUsecase) SignOut(accessToken, refreshToken *jwt.Token) error {
	atClaims := accessToken.Claims.(jwt.MapClaims)
	atUUID := atClaims["access_uuid"].(string)
	err := helper.RedisHelper.Delete(atUUID)
	if err != nil {
		return cerror.NewAndPrintWithTag("SOU00", err, global.FRIENDLY_MESSAGE)
	}

	rtClaims := refreshToken.Claims.(jwt.MapClaims)
	rtUUID := rtClaims["refresh_uuid"].(string)
	err = helper.RedisHelper.Delete(rtUUID)
	if err != nil {
		return cerror.NewAndPrintWithTag("SOU01", err, global.FRIENDLY_MESSAGE)
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
		return cerror.NewAndPrintWithTag("CPA00", err, global.FRIENDLY_INVALID_USNME_PASSWORD), false
	}

	return nil, true
}

func (uc AuthUsecase) generateEmailConfirmationUrl(account domain.Account) string {
	url := fmt.Sprintf("%s/email_confirmation?token=%s", config.Config.FEHost, account.EmailToken)
	return url
}

func (uc AuthUsecase) generateResetPasswordUrl(token string) string {
	url := fmt.Sprintf("%s/change_password?token=%s", config.Config.FEHost, token)
	return url
}

func (uc AuthUsecase) createTokenPair(account domain.Account, profile domain.Profile) (*helper.JWTWrapper, error) {
	accessTokenClaims := jwt.MapClaims{}
	accessTokenClaims["authorized"] = true
	accessTokenClaims["account_id"] = account.AccountID
	accessTokenClaims["access_uuid"] = uuid.New().String()
	accessTokenClaims["email"] = account.Email
	accessTokenClaims["exp"] = time.Now().Add(15 * time.Minute).Unix()
	accessTokenClaims["full_name"] = profile.FullName

	refreshTokenClaims := jwt.MapClaims{}
	refreshTokenClaims["account_id"] = account.AccountID
	refreshTokenClaims["refresh_uuid"] = uuid.New().String()

	rtExp := time.Now().Add(1 * time.Hour)
	refreshTokenClaims["exp"] = rtExp.Unix()

	jwtHelper := helper.JWTHelper{}
	token, err := jwtHelper.CreateTokenPair(accessTokenClaims, refreshTokenClaims)
	if err != nil {
		return nil, err
	}
	token.RefreshTokenExpTime = rtExp

	err = helper.RedisHelper.Set(accessTokenClaims["access_uuid"].(string), token.AccessToken, accessTokenClaims["exp"].(int64))
	if err != nil {
		return nil, err
	}

	err = helper.RedisHelper.Set(refreshTokenClaims["refresh_uuid"].(string), token.RefreshToken, refreshTokenClaims["exp"].(int64))
	if err != nil {
		return nil, err
	}

	return token, nil
}
