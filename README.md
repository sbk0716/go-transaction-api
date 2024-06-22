# Go Transaction API（Bitemporal版）

このリポジトリには、Go言語で書かれた取引処理APIのサンプルコードが含まれています。このAPIは、Bitemporal Data Modelを適用し、送金者から受取人への金額の送金と、取引履歴の記録を行います。

## 前提条件

- Go言語（バージョン1.16以上）がインストールされていること
- PostgreSQL（バージョン12以上）がインストールされていること
- Macが使用されていること

## セットアップ

### HomebrewによるPostgreSQLのインストールと初期設定

1. Homebrewを使用してPostgreSQLをインストールします。

```bash
brew install postgresql@14
```

2. PostgreSQLのデータベースクラスタを初期化します。

```bash
initdb --locale=C -E UTF-8 /opt/homebrew/var/postgresql@14
```

3. PostgreSQLを起動します。

```bash
brew services start postgresql@14
```

4. PostgreSQLが正しくインストールされたことを確認します。

```bash
psql --version
```

### PostgreSQLのタイムゾーン設定

PostgreSQLで日時情報をUTCで保持するように設定します。

1. PostgreSQLの設定ファイルを編集します。通常、`postgresql.conf`は以下のディレクトリにあります：

```bash
nano /opt/homebrew/var/postgresql@14/postgresql.conf
```

2. `timezone`設定を以下のように変更します：

```conf
timezone = 'UTC'
```

3. PostgreSQLを再起動して設定を反映させます。

```bash
brew services restart postgresql@14
```

### プロジェクトのセットアップ

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

6. パスワードを設定します。

```sql
\password
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
# 現在の残高照会
curl http://localhost:8080/balance/user1

# 特定の時点での残高照会
curl http://localhost:8080/balance/user1?as_of=2023-06-22T10:00:00Z
```

4. 取引履歴照会のテスト

```bash
# 全ての取引履歴照会
curl http://localhost:8080/transaction-history/user1

# 特定の時点までの取引履歴照会
curl http://localhost:8080/transaction-history/user1?as_of=2023-06-22T10:00:00Z
```

## 取引処理エンドポイントのエラーシナリオ

取引処理エンドポイント(`/transaction`)に以下のようなデータを送信するとエラーが発生します。

1. 存在しない送金者IDまたは受取人IDを指定した場合

```json
{
  "sender_id": "non_existent_user",
  "receiver_id": "user2",
  "amount": 100,
  "transaction_id": "1234567890",
  "effective_date": "2023-06-22T10:00:00Z"
}
```

このリクエストは、存在しないユーザーIDが指定されているため、エラーとなります。APIは送金者と受取人の両方が実在するユーザーであることを確認します。

2. 送金額が0以下の場合

```json
{
  "sender_id": "user1",
  "receiver_id": "user2",
  "amount": -100,
  "transaction_id": "1234567890",
  "effective_date": "2023-06-22T10:00:00Z"
}
```

このリクエストは、送金額が負の値であるため、エラーとなります。送金額は常に正の値である必要があります。

3. 送金額が送金者の残高を超えている場合

```json
{
  "sender_id": "user1",
  "receiver_id": "user2",
  "amount": 1000000000,
  "transaction_id": "1234567890",
  "effective_date": "2023-06-22T10:00:00Z"
}
```

このリクエストは、送金額が送金者の残高を超えているため、エラーとなります。APIは送金処理前に送金者の残高が十分であることを確認します。

4. effective_dateが現在時刻より前の日時の場合

```json
{
  "sender_id": "user1",
  "receiver_id": "user2",
  "amount": 100,
  "transaction_id": "1234567890",
  "effective_date": "2022-06-22T10:00:00Z"
}
```

このリクエストは、effective_dateが現在時刻より前の日時であるため、エラーとなります。APIはeffective_dateが現在時刻以降の値であることを確認します。

5. 重複したtransaction_idを指定した場合

```json
{
  "sender_id": "user1",
  "receiver_id": "user2",
  "amount": 100,
  "transaction_id": "1234567890",
  "effective_date": "2023-06-22T10:00:00Z"
}
```

このリクエストは、既に使用されたtransaction_idを指定しているため、エラーとなります。APIはtransaction_idの重複を防ぐために、一意のtransaction_idのみを受け入れます。

これらのエラーシナリオは、APIの一貫性と整合性を維持するために重要です。APIは受信したデータを検証し、不正なリクエストを適切に処理します。

## Bitemporal Data Modelについて

このAPIでは、Bitemporal Data Modelを採用しています。これにより、以下の2つの時間軸を管理しています：

1. 有効時間（Effective Time）：取引が実際に有効となる時間
2. システム時間（System Time）：データがシステムに記録された時間

この方式により、過去のある時点での残高状態を再現したり、将来の取引を事前に登録したりすることが可能になります。

## 排他制御と重複リクエスト防止

1. 排他制御：トランザクション内で`SELECT ... FOR UPDATE`を使用し、更新対象のレコードをロックしています。
2. 重複リクエスト防止：`transaction_id`をユニークキーとして使用し、同一のトランザクションIDによる重複リクエストを防いでいます。

## テストの実行

テストを実行するには、以下のコマンドを実行します。

```bash
go test ./...
```

これにより、プロジェクト内のすべてのテストが実行されます。

### テストの仕組み

このAPIのテストは、`main_test.go`ファイルで定義されています。主に以下の2つのテストが行われます：

1. **単一リクエストのテスト**：`TestTransactionEndpoint`関数で実行されます。`testcases.json`ファイルからテストケースを読み込み、各ケースに対してリクエストを送信し、期待されるレスポンスが返ってくるかをチェックします。

2. **並行リクエストのテスト**：`TestConcurrentTransactions`関数で実行されます。複数の並行リクエストを同時に送信し、全てのリクエストが成功するかをチェックします。これにより、APIの並行処理能力とデータの整合性が確認されます。

テストを実行すると、各テストケースの結果が出力されます。全てのテストが成功すれば、APIが正しく動作していることが確認できます。

## 注意事項

- このAPIは、PostgreSQLのトランザクション機能を利用して、原子性を確保しています。
- 本番環境での使用前に、十分なセキュリティ対策とパフォーマンスチューニングを行ってください。

## 貢献

このプロジェクトへの貢献を歓迎します。バグ報告や機能リクエストがある場合は、Issueを作成してください。プルリクエストも歓迎します。