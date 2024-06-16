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
