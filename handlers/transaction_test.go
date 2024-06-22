package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

// CustomValidator はEchoのカスタムバリデータです
type CustomValidator struct {
	validator *validator.Validate
}

// Validate は与えられた構造体を検証します
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

// TestData はテストデータの構造体です
type TestData struct {
	Name           string `json:"name"`
	RequestBody    string `json:"request_body"`
	ExpectedStatus int    `json:"expected_status"`
	ExpectedError  string `json:"expected_error"`
}

func TestHandleTransaction(t *testing.T) {
	// テスト用のデータベースをセットアップ
	db := SetupTestDB()
	defer db.Close()

	// テストデータをJSONファイルから読み込む
	testDataFile, err := os.Open("../testdata/transaction_test_data.json")
	if err != nil {
		t.Fatalf("Failed to open test data file: %v", err)
	}
	defer testDataFile.Close()

	var testCases []TestData
	err = json.NewDecoder(testDataFile).Decode(&testCases)
	if err != nil {
		t.Fatalf("Failed to parse test data: %v", err)
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Echoのセットアップ
			e := echo.New()
			e.Validator = &CustomValidator{validator: validator.New()}

			// リクエストの作成
			req := httptest.NewRequest(http.MethodPost, "/transaction", strings.NewReader(tc.RequestBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// ハンドラーの実行
			err := HandleTransaction(db)(c)

			// アサーション
			if tc.ExpectedError != "" {
				assert.Error(t, err)
				assert.Equal(t, tc.ExpectedStatus, rec.Code)
				var respBody map[string]string
				json.Unmarshal(rec.Body.Bytes(), &respBody)
				assert.Equal(t, tc.ExpectedError, respBody["error"])
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.ExpectedStatus, rec.Code)
			}
		})
	}
}

func TestHandleTransaction_ConcurrentRequests(t *testing.T) {
	// テスト用のデータベースをセットアップ
	db := SetupTestDB()
	defer db.Close()

	// Echoのセットアップ
	e := echo.New()
	e.Validator = &CustomValidator{validator: validator.New()}

	// 並行リクエストの作成
	reqBody := `{
		"sender_id": "user1",
		"receiver_id": "user2",
		"amount": 100,
		"transaction_id": "tx_concurrent",
		"effective_date": "2023-06-01T10:00:00Z"
	}`
	req1 := httptest.NewRequest(http.MethodPost, "/transaction", strings.NewReader(reqBody))
	req1.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)

	reqBody2 := `{
		"sender_id": "user1",
		"receiver_id": "user2",
		"amount": 100,
		"transaction_id": "tx_concurrent",
		"effective_date": "2023-06-01T10:00:00Z"
	}`
	req2 := httptest.NewRequest(http.MethodPost, "/transaction", strings.NewReader(reqBody2))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)

	// ハンドラーを並行実行
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		err1 = HandleTransaction(db)(c1)
	}()
	go func() {
		defer wg.Done()
		err2 = HandleTransaction(db)(c2)
	}()
	wg.Wait()

	// アサーション
	assert.NoError(t, err1)
	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.NoError(t, err2)
	assert.Equal(t, http.StatusInternalServerError, rec2.Code)
	var respBody map[string]string
	json.Unmarshal(rec2.Body.Bytes(), &respBody)
	assert.Equal(t, "Duplicate transaction", respBody["error"])
}

// SetupTestDB は、テスト用のデータベースを準備します
func SetupTestDB() *sqlx.DB {
	// .envファイルから環境変数を読み込みます
	err := godotenv.Load("../testdata/.env.test")
	if err != nil {
		log.Fatalf("Error loading .env.test file: %v", err)
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
		log.Fatalf("Failed to connect to test database: %v", err)
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
