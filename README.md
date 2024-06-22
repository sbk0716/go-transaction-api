# Go Transaction API（Bitemporal版）

このリポジトリには、Go言語で書かれた取引処理APIのサンプルコードが含まれています。このAPIは、Bitemporal Data Modelを適用し、送金者から受取人への金額の送金と、取引履歴の記録を行います。

## 前提条件

- Go言語（バージョン1.16以上）がインストールされていること
- PostgreSQL（バージョン12以上）がインストールされていること
- Macが使用されていること

## セットアップ

1. リポジトリをクローンします。

```bash
git clone https://github.com/your-username/go-transaction-api.git
```

2. プロジェクトディレクトリに移動します。

```bash
cd go-transaction-api
```

3. 依存関係をインストールします。

```bash
go mod download
```

4. `.env`ファイルを作成し、データベースの接続情報を設定します。

```
DB_USER=your_username
DB_PASSWORD=your_password
DB_NAME=your_database_name
DB_HOST=localhost
DB_PORT=5432
```

## データベースのセットアップ

1. PostgreSQLにログインします。

```bash
psql -U your_username
```

2. データベースを作成します。

```sql
CREATE DATABASE your_database_name;
```

3. データベースに接続します。

```
\c your_database_name
```

4. 必要なテーブルを作成します。

```sql
-- ユーザーテーブルの作成
CREATE TABLE users (
  user_id VARCHAR(255) PRIMARY KEY,
  username VARCHAR(255) NOT NULL
);

-- 残高テーブルの作成
CREATE TABLE balances (
  user_id VARCHAR(255) NOT NULL,
  amount INTEGER NOT NULL,
  valid_from TIMESTAMP NOT NULL,
  valid_to TIMESTAMP NOT NULL,
  PRIMARY KEY (user_id, valid_from),
  FOREIGN KEY (user_id) REFERENCES users(user_id)
);

-- 取引履歴テーブルの作成
CREATE TABLE transaction_history (
  id SERIAL PRIMARY KEY,
  sender_id VARCHAR(255) NOT NULL,
  receiver_id VARCHAR(255) NOT NULL,
  amount INTEGER NOT NULL,
  transaction_id VARCHAR(255) NOT NULL,
  effective_date TIMESTAMP NOT NULL,
  recorded_at TIMESTAMP NOT NULL,
  FOREIGN KEY (sender_id) REFERENCES users(user_id),
  FOREIGN KEY (receiver_id) REFERENCES users(user_id)
);

-- インデックスの作成
CREATE INDEX idx_balances_user_id_valid_to ON balances(user_id, valid_to);
CREATE INDEX idx_transaction_history_sender_id ON transaction_history(sender_id);
CREATE INDEX idx_transaction_history_receiver_id ON transaction_history(receiver_id);
CREATE INDEX idx_transaction_history_transaction_id ON transaction_history(transaction_id);
```

5. テストデータを挿入します。

```sql
-- ユーザーデータの挿入
INSERT INTO users (user_id, username) VALUES
  ('user1', 'Alice'),
  ('user2', 'Bob');

-- 初期残高データの挿入
INSERT INTO balances (user_id, amount, valid_from, valid_to) VALUES
  ('user1', 10000000, '2023-01-01 00:00:00', '9999-12-31 23:59:59'),
  ('user2', 10000000, '2023-01-01 00:00:00', '9999-12-31 23:59:59');
```

## APIの実行

1. APIサーバーを起動します。

```bash
go run main.go
```

2. 別のターミナルウィンドウで、以下のCURLコマンドを実行して取引処理をテストします。

```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "sender_id": "user1",
  "receiver_id": "user2",
  "amount": 100,
  "transaction_id": "1234567890",
  "effective_date": "2023-06-22T10:00:00Z"
}' http://localhost:8080/transaction
```

3. 残高照会のテスト

```bash
curl http://localhost:8080/balance/user1
```

4. 取引履歴照会のテスト

```bash
curl http://localhost:8080/transaction-history/user1
```

## Bitemporal Data Modelについて

このAPIでは、Bitemporal Data Modelを採用しています。これにより、以下の2つの時間軸を管理しています：

1. 有効時間（Effective Time）：取引が実際に有効となる時間
2. システム時間（System Time）：データがシステムに記録された時間

この方式により、過去のある時点での残高状態を再現したり、将来の取引を事前に登録したりすることが可能になります。

## 排他制御と多重リクエスト防止

1. 排他制御：トランザクション内で`SELECT ... FOR UPDATE`を使用し、更新対象のレコードをロックしています。
2. 多重リクエスト防止：`transaction_id`をユニークキーとして使用し、同一のトランザクションIDによる重複リクエストを防いでいます。

## テストの実行

テストを実行するには、以下のコマンドを実行します。

```bash
go test ./...
```

これにより、プロジェクト内のすべてのテストが実行されます。

## 注意事項

- このAPIは、PostgreSQLのトランザクション機能を利用して、原子性を確保しています。
- 本番環境での使用前に、十分なセキュリティ対策とパフォーマンスチューニングを行ってください。

## 貢献

このプロジェクトへの貢献を歓迎します。バグ報告や機能リクエストがある場合は、Issueを作成してください。プルリクエストも歓迎します。