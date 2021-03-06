package mysql

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/go-sql-driver/mysql"
	"github.com/pajri/personal-backend/adapter/cerror"
	"github.com/pajri/personal-backend/domain"
	"github.com/pajri/personal-backend/global"
	"github.com/pajri/personal-backend/util"
)

func NewMySqlAccountRepository(db *sql.DB) domain.IAccountRepository {
	return &MySqlUserRepository{
		Db: db,
	}
}

type MySqlUserRepository struct {
	Db *sql.DB
}

func (ur MySqlUserRepository) GetAccount(filter domain.AccountFilter) (*domain.Account, error) {
	query := sq.Select("account_id, password, email, salt, email_token, is_verified, password_token").
		From("account")

	if filter.Email != "" {
		query = query.Where(sq.Eq{"email": filter.Email})
	}

	if filter.AccountID != "" {
		query = query.Where(sq.Eq{"account_id": filter.AccountID})
	}

	sqlString, args, err := query.ToSql()
	if err != nil {
		return nil, cerror.NewAndPrintWithTag("GA00", err, global.FRIENDLY_MESSAGE)
	}

	row := ur.Db.QueryRow(sqlString, args...)

	account := new(domain.Account)
	err = row.Scan(
		&account.AccountID,
		&account.Password,
		&account.Email,
		&account.Salt,
		&account.EmailToken,
		&account.IsVerified,
		&account.PasswordToken,
	)
	if err != nil {
		return nil, cerror.NewAndPrintWithTag("GA02", err, global.FRIENDLY_MESSAGE)
	}

	return account, nil
}

func (ur MySqlUserRepository) InsertAccount(account domain.Account) (*domain.Account, error) {
	if account.AccountID == "" {
		account.AccountID = util.GenerateUUID()
	}

	query := sq.Insert("account").
		Columns(`
			account_id, 
			email, 
			password, 
			salt, 
			email_token, 
			is_verified,
			password_token`).
		Values(
			account.AccountID,
			account.Email,
			account.Password,
			account.Salt,
			account.EmailToken,
			account.IsVerified,
			account.PasswordToken,
		)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, cerror.NewAndPrintWithTag("IA00", err, global.FRIENDLY_MESSAGE)
	}

	tx, err := ur.Db.Begin()
	if err != nil {
		return nil, cerror.NewAndPrintWithTag("IA01", err, global.FRIENDLY_MESSAGE)
	}

	stmt, err := tx.Prepare(sql)
	if err != nil {
		tx.Rollback()
		return nil, cerror.NewAndPrintWithTag("IA02", err, global.FRIENDLY_MESSAGE)
	}
	defer stmt.Close()

	_, err = tx.Exec(sql, args...)
	if err != nil {
		errMySQL, ok := err.(*mysql.MySQLError)
		if ok && errMySQL.Number == 1062 {
			tx.Rollback()
			return nil, cerror.NewAndPrintWithTag("IA03", err, global.FRIENDLY_DUPLICATE_EMAIL)
		}
		tx.Rollback()
		return nil, cerror.NewAndPrintWithTag("IA05", err, global.FRIENDLY_MESSAGE)
	}

	err = tx.Commit()
	if err != nil {
		return nil, cerror.NewAndPrintWithTag("IA04", err, global.FRIENDLY_MESSAGE)
	}

	return &account, nil
}

func (ur MySqlUserRepository) UpdateIsVerified(accountId string, isVerified bool) error {
	/*start create query*/
	query := sq.Update("account").
		Set("is_verified", isVerified).
		Where(sq.Eq{"account_id": accountId})

	sqlString, args, err := query.ToSql()
	if err != nil {
		return cerror.NewAndPrintWithTag("UIV00", err, global.FRIENDLY_MESSAGE)
	}
	/*start create query*/

	tx, err := ur.Db.Begin()
	if err != nil {
		return cerror.NewAndPrintWithTag("UIV01", err, global.FRIENDLY_MESSAGE)
	}

	stmt, err := tx.Prepare(sqlString)
	if err != nil {
		tx.Rollback()
		return cerror.NewAndPrintWithTag("UIV02", err, global.FRIENDLY_MESSAGE)
	}
	defer stmt.Close()

	_, err = tx.Exec(sqlString, args...)
	if err != nil {
		tx.Rollback()
		return cerror.NewAndPrintWithTag("UIV03", err, global.FRIENDLY_MESSAGE)
	}

	err = tx.Commit()
	if err != nil {
		return cerror.NewAndPrintWithTag("UIV04", err, global.FRIENDLY_MESSAGE)
	}
	return nil
}

func (ur MySqlUserRepository) UpdateSaltAndPassword(account domain.Account) error {
	/*start create query*/
	query := sq.Update("account").
		Set("salt", account.Salt).
		Set("password", account.Password).
		Where(sq.Eq{"account_id": account.AccountID})

	sqlString, args, err := query.ToSql()
	if err != nil {
		return cerror.NewAndPrintWithTag("USP00", err, global.FRIENDLY_MESSAGE)
	}
	/*start create query*/

	tx, err := ur.Db.Begin()
	if err != nil {
		return cerror.NewAndPrintWithTag("USP01", err, global.FRIENDLY_MESSAGE)
	}

	stmt, err := tx.Prepare(sqlString)
	if err != nil {
		tx.Rollback()
		return cerror.NewAndPrintWithTag("USP02", err, global.FRIENDLY_MESSAGE)
	}
	defer stmt.Close()

	_, err = tx.Exec(sqlString, args...)
	if err != nil {
		tx.Rollback()
		return cerror.NewAndPrintWithTag("USP03", err, global.FRIENDLY_MESSAGE)
	}

	err = tx.Commit()
	if err != nil {
		return cerror.NewAndPrintWithTag("USP04", err, global.FRIENDLY_MESSAGE)
	}
	return nil
}

func (ur MySqlUserRepository) UpdatePasswordToken(account domain.Account) error {
	query := sq.Update("account").
		Set("password_token", account.PasswordToken).
		Where(sq.Eq{"account_id": account.AccountID})

	sqlString, args, err := query.ToSql()
	if err != nil {
		return cerror.NewAndPrintWithTag("UPT00", err, global.FRIENDLY_MESSAGE)
	}

	tx, err := ur.Db.Begin()
	if err != nil {
		return cerror.NewAndPrintWithTag("UPT01", err, global.FRIENDLY_MESSAGE)
	}

	stmt, err := tx.Prepare(sqlString)
	if err != nil {
		tx.Rollback()
		return cerror.NewAndPrintWithTag("UPT02", err, global.FRIENDLY_MESSAGE)
	}
	defer stmt.Close()

	_, err = tx.Exec(sqlString, args...)
	if err != nil {
		tx.Rollback()
		return cerror.NewAndPrintWithTag("UPT03", err, global.FRIENDLY_MESSAGE)
	}

	err = tx.Commit()
	if err != nil {
		return cerror.NewAndPrintWithTag("UPT04", err, global.FRIENDLY_MESSAGE)
	}
	return nil
}
