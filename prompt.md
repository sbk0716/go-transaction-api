# コード
## main.go
```go
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

```
## handlers/transaction.go
```go
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

```


---
# プロンプト
上記のコードはGo言語で書かれた取引処理APIのサンプルコードです。
このAPIは、送金者から受取人への金額の送金と、取引履歴の記録を行います。
正常に動作することは確認できています。

Artifactsを使用して、このコードに以下のタスクを実行し、改修したコードの全量を表示してください。

## タスク１
Bitemporal Data Modelを適用したコードに改修してください。
コードやDBの構造は大幅に変更して問題ないです。
ただし、Go言語やBitemporal Data Modelのベストプラクティスに則ってコードを改修してください。

## タスク２
お金の入出金に関連するAPIなので、DBの不整合は絶対に許されないです。
その為、排他制御・多重リクエスト防止などのDBの不整合を防ぐための仕組みは絶対に追加してください。
また、一般的な勘定系システムのベストプラクティスに則ってコードを改修してください。


## タスク３
IT初心者でも理解できるように、改修したコードには可能な限り日本語でコメントを入れてください。


## タスク４
IT初心者でも理解できるように、改修したコードを動作確認するためのREADMEを生成してください。
現在のREADMEは以下の通りです。

↓

---
# Go Transaction API

このリポジトリには、Go言語で書かれた取引処理APIのサンプルコードが含まれています。このAPIは、送金者から受取人への金額の送金と、取引履歴の記録を行います。

## 前提条件

- Go言語がインストールされていること
- Macが使用されていること

## セットアップ

1. Homebrewを使用してPostgreSQLをインストールします。

```
brew install postgresql
```

2. PostgreSQLを初期化します。

```
initdb /usr/local/var/postgres
```

3. PostgreSQLを起動します。

```
brew services start postgresql
```

4. リポジトリをクローンします。

```
git clone https://github.com/your-username/go-transaction-api.git
```

5. プロジェクトディレクトリに移動します。

```
cd go-transaction-api
```

6. 依存関係をインストールします。

```
go mod download
```

7. `.env`ファイルを作成し、データベースの接続情報を設定します。

```
DB_USER=your_username
DB_PASSWORD=your_password
DB_NAME=your_database_name
```

## データベースのセットアップ

1. PostgreSQLにログインします。

```
psql -U your_username
```

2. パスワードを設定します。

```
\password your_password
```

3. データベースを作成します。

```sql
CREATE DATABASE your_database_name;
```

4. データベースに接続します。

```
\c your_database_name
```

5. `balances`テーブルを作成します。

```sql
CREATE TABLE balances (
  user_id VARCHAR(255) PRIMARY KEY,
  username VARCHAR(255) NOT NULL,
  amount INTEGER NOT NULL
);
```

6. `transaction_history`テーブルを作成します。

```sql
CREATE TABLE transaction_history (
  id SERIAL PRIMARY KEY,
  sender_id VARCHAR(255) NOT NULL,
  receiver_id VARCHAR(255) NOT NULL,
  amount INTEGER NOT NULL,
  transaction_id VARCHAR(255) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (sender_id) REFERENCES balances(user_id),
  FOREIGN KEY (receiver_id) REFERENCES balances(user_id)
);
```

7. `semaphore`テーブルを作成します。

```sql
CREATE TABLE semaphore (
  id INTEGER PRIMARY KEY,
  lock BOOLEAN NOT NULL DEFAULT FALSE
);
```

8. `semaphore`テーブルにレコードを挿入します。

```sql
INSERT INTO semaphore (id, lock) VALUES (1, false);
```

9. テストデータを挿入します。

```sql
INSERT INTO balances (user_id, username, amount) VALUES
  ('user1', 'Alice', 1000),
  ('user2', 'Bob', 500);
```

## APIの実行

1. APIサーバーを起動します。

```
go run main.go
```

2. 別のターミナルウィンドウで、以下のCURLコマンドを実行して取引処理をテストします。

```
curl -X POST -H "Content-Type: application/json" -d '{
  "sender_id": "user1",
  "receiver_id": "user2",
  "amount": 100,
  "transaction_id": "1234567890"
}' http://localhost:8080/transaction
```

## テストの実行

1. テストを実行するには、以下のコマンドを実行します。

```
go test ./...
```

これにより、プロジェクト内のすべてのテストが実行されます。

## 注意事項

- このAPIは、セマフォを使用して同時実行を制御し、重複リクエストを防止します。
- トランザクション処理は、PostgreSQLのトランザクション機能を利用して、原子性を確保しています。

## 貢献

このプロジェクトへの貢献を歓迎します。バグ報告や機能リクエストがある場合は、Issueを作成してください。プルリクエストも歓迎します。

---

