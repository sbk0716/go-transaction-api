package handlers

import (
	"net/http"
	"time"
)

var transactionTests = []struct {
	name           string
	request        TransactionRequest
	expectedStatus int
	expectedError  string
}{
	{
		name: "Valid transaction",
		request: TransactionRequest{
			SenderID:      "user1",
			ReceiverID:    "user2",
			Amount:        100,
			TransactionID: "test-transaction-1",
			EffectiveDate: time.Now().Add(time.Hour),
		},
		expectedStatus: http.StatusOK,
	},
	{
		name: "Non-existent sender",
		request: TransactionRequest{
			SenderID:      "nonexistent",
			ReceiverID:    "user2",
			Amount:        100,
			TransactionID: "test-transaction-2",
			EffectiveDate: time.Now().Add(time.Hour),
		},
		expectedStatus: http.StatusInternalServerError,
		expectedError:  "User does not exist",
	},
	{
		name: "Insufficient balance",
		request: TransactionRequest{
			SenderID:      "user1",
			ReceiverID:    "user2",
			Amount:        2000,
			TransactionID: "test-transaction-3",
			EffectiveDate: time.Now().Add(time.Hour),
		},
		expectedStatus: http.StatusInternalServerError,
		expectedError:  "Insufficient balance",
	},
}
