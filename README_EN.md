# Go Transaction API

This repository contains sample code for a transaction processing API written in Go. This API handles the transfer of funds from a sender to a recipient and records the transaction history.

## Prerequisites

- Go language is installed
- Mac is being used

## Setup

1. Install PostgreSQL using Homebrew.

```
brew install postgresql
```

2. Initialize PostgreSQL.

```
initdb /usr/local/var/postgres
```

3. Start PostgreSQL.

```
brew services start postgresql
```

4. Clone the repository.

```
git clone https://github.com/your-username/go-transaction-api.git
```

5. Change to the project directory.

```
cd go-transaction-api
```

6. Install dependencies.

```
go mod download
```

7. Create a `.env` file and set the database connection information.

```
DB_USER=your_username
DB_PASSWORD=your_password
DB_NAME=your_database_name
```

## Database Setup

1. Log in to PostgreSQL.

```
psql -U your_username
```

2. Set a password.

```
\password your_username
```

3. Create a database.

```sql
CREATE DATABASE your_database_name;
```

4. Connect to the database.

```
\c your_database_name
```

5. Create the `balances` table.

```sql
CREATE TABLE balances (
  user_id VARCHAR(255) PRIMARY KEY,
  username VARCHAR(255) NOT NULL,
  amount INTEGER NOT NULL
);
```

6. Create the `transaction_history` table.

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

7. Create the `semaphore` table.

```sql
CREATE TABLE semaphore (
  id INTEGER PRIMARY KEY,
  lock BOOLEAN NOT NULL DEFAULT FALSE
);
```

8. Insert a record into the `semaphore` table.

```sql
INSERT INTO semaphore (id, lock) VALUES (1, false);
```

9. Insert test data.

```sql
INSERT INTO balances (user_id, username, amount) VALUES
  ('user1', 'Alice', 1000),
  ('user2', 'Bob', 500);
```

## Running the API

1. Start the API server.

```
go run main.go
```

2. In a separate terminal window, execute the following cURL command to test the transaction processing.

```
curl -X POST -H "Content-Type: application/json" -d '{
  "sender_id": "user1",
  "receiver_id": "user2",
  "amount": 100,
  "transaction_id": "1234567890"
}' http://localhost:8080/transaction
```

## Running Tests

1. To run the tests, execute the following command.

```
go test ./...
```

This will run all the tests in the project.

## Notes

- This API uses a semaphore to control concurrent execution and prevent duplicate requests.
- The transaction processing ensures atomicity by utilizing PostgreSQL's transaction functionality.

## Contributions

Contributions to this project are welcome. If you have any bug reports or feature requests, please create an Issue. Pull requests are also welcome.