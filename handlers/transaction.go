package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// Balance は残高情報を表す構造体
type Balance struct {
	UserID   string `db:"user_id" json:"user_id"`
	Username string `db:"username" json:"username"`
	Amount   int    `db:"amount" json:"amount"`
}

// TransactionRequest は取引リクエストの情報を表す構造体
type TransactionRequest struct {
	SenderID      string `json:"sender_id" validate:"required"`
	ReceiverID    string `json:"receiver_id" validate:"required"`
	Amount        int    `json:"amount" validate:"required,gt=0"`
	TransactionID string `json:"transaction_id" validate:"required"`
}

// TransactionHistory は取引履歴の情報を表す構造体
type TransactionHistory struct {
	ID            int    `db:"id" json:"id"`
	SenderID      string `db:"sender_id" json:"sender_id"`
	ReceiverID    string `db:"receiver_id" json:"receiver_id"`
	Amount        int    `db:"amount" json:"amount"`
	TransactionID string `db:"transaction_id" json:"transaction_id"`
	CreatedAt     string `db:"created_at" json:"created_at"`
}

// HandleTransaction は取引処理のハンドラー
func HandleTransaction(db *sqlx.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		// リクエストの情報を取得
		var req TransactionRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストが不正です"})
		}
		// リクエストの情報をバリデーション
		if err := c.Validate(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "リクエストデータが無効です"})
		}

		// 取引処理を実行
		if err := processTransaction(db, req); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		// 取引成功のレスポンスを返す
		return c.JSON(http.StatusOK, map[string]string{"message": "取引が成功しました"})
	}
}

// processTransaction は取引処理の実際の実装
func processTransaction(db *sqlx.DB, req TransactionRequest) error {
	// トランザクションを開始
	tx, err := db.Beginx()
	if err != nil {
		return err
	}

	// トランザクション終了時の処理を定義
	defer func() {
		if err != nil {
			// エラーがある場合はロールバック
			tx.Rollback()
			return
		}
		// トランザクションをコミット
		err = tx.Commit()
		if err != nil {
			log.Printf("Failed to commit transaction: %v", err)
			return
		}

		// セマフォを解放
		_, err = db.Exec("UPDATE semaphore SET lock = false WHERE id = 1")
		if err != nil {
			log.Printf("Failed to release semaphore: %v", err)
			return
		}
	}()

	// セマフォを取得
	var lock bool
	err = tx.Get(&lock, "SELECT lock FROM semaphore WHERE id = 1 FOR UPDATE")
	if err != nil {
		log.Printf("Failed to acquire semaphore: %v", err)
		return errors.New("Failed to acquire semaphore")
	}
	if lock {
		log.Printf("Semaphore is already locked")
		return errors.New("Semaphore is already locked")
	}

	// セマフォをロック
	_, err = tx.Exec("UPDATE semaphore SET lock = true WHERE id = 1")
	if err != nil {
		log.Printf("Failed to update semaphore: %v", err)
		return errors.New("Failed to update semaphore")
	}

	// 重複リクエストの判定
	var count int
	err = tx.Get(&count, "SELECT COUNT(*) FROM transaction_history WHERE transaction_id = $1", req.TransactionID)
	if err != nil {
		log.Printf("Failed to check duplicate transaction: %v", err)
		return errors.New("Failed to check duplicate transaction")
	}
	if count > 0 {
		log.Printf("Duplicate transaction detected: %v", req.TransactionID)
		return errors.New("Duplicate transaction")
	}

	// 送金者の残高を減算
	_, err = tx.Exec("UPDATE balances SET amount = amount - $1 WHERE user_id = $2 AND amount >= $1", req.Amount, req.SenderID)
	if err != nil {
		return err
	}

	// 受取人の残高を増額
	_, err = tx.Exec("UPDATE balances SET amount = amount + $1 WHERE user_id = $2", req.Amount, req.ReceiverID)
	if err != nil {
		return err
	}

	// 取引履歴を記録
	_, err = tx.Exec("INSERT INTO transaction_history (sender_id, receiver_id, amount, transaction_id) VALUES ($1, $2, $3, $4)", req.SenderID, req.ReceiverID, req.Amount, req.TransactionID)
	if err != nil {
		return err
	}

	return nil
}
