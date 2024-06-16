package handlers

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// セマフォ、1つのスロット
var sem = make(chan struct{}, 1)

// ユーザーの残高を表す構造体
type Balance struct {
	UserID   string `db:"user_id" json:"user_id"`
	Username string `db:"username" json:"username"`
	Amount   int    `db:"amount" json:"amount"`
}

// 取引リクエストを表す構造体
type TransactionRequest struct {
	SenderID   string `json:"sender_id" validate:"required"`
	ReceiverID string `json:"receiver_id" validate:"required"`
	Amount     int    `json:"amount" validate:"required,gt=0"`
}

// 取引履歴を表す構造体
type TransactionHistory struct {
	ID         int    `db:"id" json:"id"`
	SenderID   string `db:"sender_id" json:"sender_id"`
	ReceiverID string `db:"receiver_id" json:"receiver_id"`
	Amount     int    `db:"amount" json:"amount"`
	CreatedAt  string `db:"created_at" json:"created_at"`
}

// HandleTransaction handles the transaction process
func HandleTransaction(db *sqlx.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req TransactionRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストが不正です"})
		}
		if err := c.Validate(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストデータが無効です"})
		}

		// セマフォを使用して排他制御
		sem <- struct{}{}        // セマフォのロックを取得(P操作: セマフォの値から1引く操作)
		defer func() { <-sem }() // HandleTransaction関数終了時にセマフォのロックを解放(V操作: セマフォの値に1足す操作)

		if err := processTransaction(db, req); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]string{"message": "取引が成功しました"})
	}
}

func processTransaction(db *sqlx.DB, req TransactionRequest) error {
	tx := db.MustBegin()

	_, err := tx.Exec("UPDATE balances SET amount = amount - $1 WHERE user_id = $2 AND amount >= $1", req.Amount, req.SenderID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("UPDATE balances SET amount = amount + $1 WHERE user_id = $2", req.Amount, req.ReceiverID)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.Exec("INSERT INTO transaction_history (sender_id, receiver_id, amount) VALUES ($1, $2, $3)", req.SenderID, req.ReceiverID, req.Amount)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
