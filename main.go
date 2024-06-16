// main.go
package main

import (
	"log"
	"os"

	"go-transaction-api/handlers"

	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

// グローバル変数
var db *sqlx.DB

// CustomValidator is a custom validator for Echo
type CustomValidator struct {
	validator *validator.Validate
}

// Validate validates a given struct
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	dbUser := os.Getenv("DB_USER")
	// dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	var dbErr error
	// db, dbErr = sqlx.Connect("postgres", "user="+dbUser+" password="+dbPassword+" dbname="+dbName+" sslmode=disable")
	db, dbErr = sqlx.Connect("postgres", "user="+dbUser+" dbname="+dbName+" sslmode=disable")
	if dbErr != nil {
		log.Fatalf("Failed to connect to database: %v", dbErr)
	}
}

func main() {
	e := echo.New()

	// Set custom validator
	e.Validator = &CustomValidator{validator: validator.New()}

	// 取引用のエンドポイントを設定
	e.POST("/transaction", handlers.HandleTransaction(db))

	// サーバーを起動
	e.Start(":8080")
}
