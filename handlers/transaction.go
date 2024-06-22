package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// User はユーザー情報を表す構造体です
type User struct {
	UserID   string `db:"user_id" json:"user_id"`
	Username string `db:"username" json:"username"`
}

// Balance は残高情報を表す構造体です
type Balance struct {
	UserID    string    `db:"user_id" json:"user_id"`
	Amount    int       `db:"amount" json:"amount"`
	ValidFrom time.Time `db:"valid_from" json:"valid_from"`
	ValidTo   time.Time `db:"valid_to" json:"valid_to"`
}

// TransactionRequest は取引リクエストの情報を表す構造体です
type TransactionRequest struct {
	SenderID      string    `json:"sender_id" validate:"required"`
	ReceiverID    string    `json:"receiver_id" validate:"required"`
	Amount        int       `json:"amount" validate:"required,gt=0"`
	TransactionID string    `json:"transaction_id" validate:"required"`
	EffectiveDate time.Time `json:"effective_date" validate:"required"`
}

// TransactionHistory は取引履歴の情報を表す構造体です
type TransactionHistory struct {
	ID            int       `db:"id" json:"id"`
	SenderID      string    `db:"sender_id" json:"sender_id"`
	ReceiverID    string    `db:"receiver_id" json:"receiver_id"`
	Amount        int       `db:"amount" json:"amount"`
	TransactionID string    `db:"transaction_id" json:"transaction_id"`
	EffectiveDate time.Time `db:"effective_date" json:"effective_date"`
	RecordedAt    time.Time `db:"recorded_at" json:"recorded_at"`
}

// HandleTransaction は取引処理のハンドラーです
func HandleTransaction(db *sqlx.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		// リクエストの情報を取得します
		var req TransactionRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストが不正です"})
		}
		// リクエストの情報をバリデーションします
		if err := c.Validate(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストデータが無効です"})
		}

		// effective_dateが現在時刻より前の場合はエラーを返します
		if req.EffectiveDate.Before(time.Now()) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "effective_dateは現在時刻以降の値を指定してください"})
		}

		// 取引処理を実行します
		if err := processTransaction(db, req); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		// 取引成功のレスポンスを返します
		return c.JSON(http.StatusOK, map[string]string{"message": "取引が成功しました"})
	}
}

// processTransaction は取引処理の実際の実装です
func processTransaction(db *sqlx.DB, req TransactionRequest) error {
	// トランザクションを開始します
	tx, err := db.Beginx()
	if err != nil {
		return err
	}

	// トランザクション終了時の処理を定義します
	defer func() {
		if err != nil {
			// エラーがある場合はロールバックします
			tx.Rollback()
			return
		}
		// トランザクションをコミットします
		err = tx.Commit()
		if err != nil {
			log.Printf("Failed to commit transaction: %v", err)
			return
		}
	}()

	// ユーザーの存在を確認します
	if err := checkUserExists(tx, req.SenderID); err != nil {
		return err
	}
	if err := checkUserExists(tx, req.ReceiverID); err != nil {
		return err
	}

	// 排他ロックを取得します
	if err := acquireLock(tx, req.SenderID, req.ReceiverID); err != nil {
		return err
	}

	// 重複リクエストの判定を行います
	if err := checkDuplicateTransaction(tx, req.TransactionID); err != nil {
		return err
	}

	// 送金者の残高を更新します
	if err := updateBalance(tx, req.SenderID, -req.Amount, req.EffectiveDate); err != nil {
		return err
	}

	// 受取人の残高を更新します
	if err := updateBalance(tx, req.ReceiverID, req.Amount, req.EffectiveDate); err != nil {
		return err
	}

	// 取引履歴を記録します
	if err := recordTransaction(tx, req); err != nil {
		return err
	}

	return nil
}

// checkUserExists はユーザーの存在を確認します
func checkUserExists(tx *sqlx.Tx, userID string) error {
	var count int
	err := tx.Get(&count, "SELECT COUNT(*) FROM users WHERE user_id = $1", userID)
	if err != nil {
		return errors.New("Failed to check user existence")
	}
	if count == 0 {
		return errors.New("User does not exist")
	}
	return nil
}

// acquireLock は排他ロックを取得します
func acquireLock(tx *sqlx.Tx, senderID, receiverID string) error {
	// 送金者と受取人のIDを昇順にソートしてロックを取得します
	// これにより、デッドロックを防ぎます
	ids := []string{senderID, receiverID}
	if senderID > receiverID {
		ids[0], ids[1] = receiverID, senderID
	}

	for _, id := range ids {
		_, err := tx.Exec("SELECT * FROM balances WHERE user_id = $1 FOR UPDATE", id)
		if err != nil {
			return errors.New("Failed to acquire lock")
		}
	}

	return nil
}

// checkDuplicateTransaction は重複リクエストをチェックします
func checkDuplicateTransaction(tx *sqlx.Tx, transactionID string) error {
	var count int
	err := tx.Get(&count, "SELECT COUNT(*) FROM transaction_history WHERE transaction_id = $1", transactionID)
	if err != nil {
		return errors.New("Failed to check duplicate transaction")
	}
	if count > 0 {
		return errors.New("Duplicate transaction")
	}
	return nil
}

// updateBalance は残高を更新します
func updateBalance(tx *sqlx.Tx, userID string, amount int, effectiveDate time.Time) error {
	// 現在の有効な残高レコードを取得します
	var currentBalance Balance
	err := tx.Get(&currentBalance, `
    SELECT * FROM balances 
    WHERE user_id = $1 AND valid_to = '9999-12-31 23:59:59'
  `, userID)
	if err != nil {
		return errors.New("Failed to get current balance")
	}

	// 新しい残高を計算します
	newAmount := currentBalance.Amount + amount
	if newAmount < 0 {
		return errors.New("Insufficient balance")
	}

	// 現在のレコードの有効期間を更新します
	_, err = tx.Exec(`
    UPDATE balances 
    SET valid_to = $1 
    WHERE user_id = $2 AND valid_to = '9999-12-31 23:59:59'
  `, effectiveDate, userID)
	if err != nil {
		return errors.New("Failed to update current balance record")
	}

	// 新しい残高レコードを挿入します
	_, err = tx.Exec(`
    INSERT INTO balances (user_id, amount, valid_from, valid_to) 
    VALUES ($1, $2, $3, '9999-12-31 23:59:59')
  `, userID, newAmount, effectiveDate)
	if err != nil {
		return errors.New("Failed to insert new balance record")
	}

	return nil
}

// recordTransaction は取引履歴を記録します
func recordTransaction(tx *sqlx.Tx, req TransactionRequest) error {
	_, err := tx.Exec(`
    INSERT INTO transaction_history (sender_id, receiver_id, amount, transaction_id, effective_date, recorded_at) 
    VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
  `, req.SenderID, req.ReceiverID, req.Amount, req.TransactionID, req.EffectiveDate)
	if err != nil {
		return errors.New("Failed to record transaction history")
	}
	return nil
}

// HandleGetBalance は残高照会のハンドラーです
func HandleGetBalance(db *sqlx.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := c.Param("userId")
		asOf := c.QueryParam("as_of")

		var balance Balance
		var err error

		if asOf == "" {
			// as_ofパラメータが指定されていない場合は現在の残高を取得
			err = db.Get(&balance, `
        SELECT * FROM balances
        WHERE user_id = $1 AND valid_to = '9999-12-31 23:59:59'
      `, userID)
		} else {
			// as_ofパラメータが指定されている場合はその時点での残高を取得
			err = db.Get(&balance, `
        SELECT * FROM balances
        WHERE user_id = $1 AND valid_from <= $2 AND valid_to > $2
      `, userID, asOf)
		}

		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get balance"})
		}
		return c.JSON(http.StatusOK, balance)
	}
}

// HandleGetTransactionHistory は取引履歴照会のハンドラーです
func HandleGetTransactionHistory(db *sqlx.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := c.Param("userId")
		asOf := c.QueryParam("as_of")

		var history []TransactionHistory
		var err error

		if asOf == "" {
			// as_ofパラメータが指定されていない場合は全ての取引履歴を取得
			err = db.Select(&history, `
        SELECT * FROM transaction_history
        WHERE sender_id = $1 OR receiver_id = $1
        ORDER BY effective_date DESC, recorded_at DESC
      `, userID)
		} else {
			// as_ofパラメータが指定されている場合はその時点までの取引履歴を取得
			err = db.Select(&history, `
        SELECT * FROM transaction_history
        WHERE (sender_id = $1 OR receiver_id = $1) AND effective_date <= $2
        ORDER BY effective_date DESC, recorded_at DESC
      `, userID, asOf)
		}

		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get transaction history"})
		}
		return c.JSON(http.StatusOK, history)
	}
}
