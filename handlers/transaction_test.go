package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CustomValidator はEchoのカスタムバリデータです
type CustomValidator struct {
	validator *validator.Validate
}

// Validate は与えられた構造体を検証します
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

func TestHandleTransaction(t *testing.T) {
	// テスト用のデータベースをセットアップ
	db := SetupTestDB(t)
	defer db.Close()

	// Echoのインスタンスを作成
	e := echo.New()
	e.Validator = &CustomValidator{validator: validator.New()}

	// テストデータをJSONファイルから読み込む
	testDataFile, err := os.Open("../testdata/transaction_test_data.json")
	require.NoError(t, err)
	defer testDataFile.Close()

	var testCases []struct {
		Name           string             `json:"name"`
		RequestBody    TransactionRequest `json:"requestBody"`
		ExpectedStatus int                `json:"expectedStatus"`
		ExpectedError  string             `json:"expectedError"`
	}
	err = json.NewDecoder(testDataFile).Decode(&testCases)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// リクエストボディを作成
			reqBody, err := json.Marshal(tc.RequestBody)
			require.NoError(t, err)

			// Echoのリクエストを作成
			req := httptest.NewRequest(http.MethodPost, "/transaction", bytes.NewReader(reqBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// ハンドラーを実行
			err = HandleTransaction(db)(c)

			// アサーション
			if tc.ExpectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.ExpectedError)
				assert.Equal(t, tc.ExpectedStatus, rec.Code)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.ExpectedStatus, rec.Code)
			}
		})
	}
}

func TestHandleTransaction_ConcurrentRequests(t *testing.T) {
	// テスト用のデータベースをセットアップ
	db := SetupTestDB(t)
	defer db.Close()

	// Echoのインスタンスを作成
	e := echo.New()
	e.Validator = &CustomValidator{validator: validator.New()}

	requestBody := TransactionRequest{
		SenderID:      "user1",
		ReceiverID:    "user2",
		Amount:        100,
		TransactionID: "tx_concurrent",
		EffectiveDate: time.Now().Add(time.Hour),
	}

	reqBody, err := json.Marshal(requestBody)
	require.NoError(t, err)

	// 並行リクエストを実行
	concurrency := 10
	var successCount int32
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodPost, "/transaction", bytes.NewReader(reqBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := HandleTransaction(db)(c)
			if err == nil && rec.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	// アサーション
	assert.Equal(t, int32(1), successCount)
}

// SetupTestDB は、テスト用のデータベースを準備します
func SetupTestDB(t *testing.T) *sqlx.DB {
	// .envファイルから環境変数を読み込みます
	err := godotenv.Load("../testdata/.env.test")
	if err != nil {
		t.Fatalf("Error loading .env.test file: %v", err)
	}

	// データベース接続情報を環境変数から取得します
	dbUser := os.Getenv("DB_USER")
	dbName := os.Getenv("DB_NAME")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")

	// テスト用のデータベースに接続します
	db, err := sqlx.Connect("postgres",
		"host="+dbHost+" port="+dbPort+" user="+dbUser+" dbname="+dbName+" sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// コネクションプールの設定
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// テーブルを削除します
	db.MustExec(`DROP TABLE IF EXISTS transaction_history`)
	db.MustExec(`DROP TABLE IF EXISTS balances`)
	db.MustExec(`DROP TABLE IF EXISTS users`)

	// テーブルを作成します
	db.MustExec(`
		CREATE TABLE users (
			user_id VARCHAR(255) PRIMARY KEY,
			username VARCHAR(255) NOT NULL
		)
	`)
	db.MustExec(`
		CREATE TABLE balances (
			user_id VARCHAR(255) REFERENCES users(user_id),
			amount INTEGER NOT NULL,
			valid_from TIMESTAMP NOT NULL,
			valid_to TIMESTAMP NOT NULL,
			PRIMARY KEY (user_id, valid_from)
		)
	`)
	db.MustExec(`
		CREATE TABLE transaction_history (
			id SERIAL PRIMARY KEY,
			sender_id VARCHAR(255) REFERENCES users(user_id),
			receiver_id VARCHAR(255) REFERENCES users(user_id),
			amount INTEGER NOT NULL,
			transaction_id VARCHAR(255) NOT NULL UNIQUE,
			effective_date TIMESTAMP NOT NULL,
			recorded_at TIMESTAMP NOT NULL
		)
	`)

	// テストデータを挿入します
	db.MustExec(`
		INSERT INTO users (user_id, username) VALUES
		('user1', 'User 1'),
		('user2', 'User 2')
	`)
	db.MustExec(`
		INSERT INTO balances (user_id, amount, valid_from, valid_to) VALUES
		('user1', 1000, '2023-01-01 00:00:00', '9999-12-31 23:59:59'),
		('user2', 500, '2023-01-01 00:00:00', '9999-12-31 23:59:59')
	`)

	return db
}
