package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

type TestCase struct {
	Name         string                 `json:"name"`
	RequestBody  map[string]interface{} `json:"requestBody"`
	ExpectedCode int                    `json:"expectedCode"`
	ExpectedErr  string                 `json:"expectedErr"`
}

func TestTransactionEndpoint(t *testing.T) {
	// テストケースをJSONファイルから読み込む
	testCasesFile, err := os.Open("testcases.json")
	if err != nil {
		t.Fatalf("Failed to open test cases file: %v", err)
	}
	defer testCasesFile.Close()

	testCasesData, err := io.ReadAll(testCasesFile)
	if err != nil {
		t.Fatalf("Failed to read test cases file: %v", err)
	}

	var testCases []TestCase
	err = json.Unmarshal(testCasesData, &testCases)
	if err != nil {
		t.Fatalf("Failed to parse test cases: %v", err)
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			requestBody, err := json.Marshal(tc.RequestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}

			resp, err := http.Post("http://localhost:8080/transaction", "application/json", bytes.NewBuffer(requestBody))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.ExpectedCode {
				t.Errorf("Expected status code %d but got %d", tc.ExpectedCode, resp.StatusCode)
			}

			var result map[string]string
			err = json.NewDecoder(resp.Body).Decode(&result)
			if err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if tc.ExpectedErr != "" {
				if errMsg, ok := result["error"]; !ok || errMsg != tc.ExpectedErr {
					t.Errorf("Expected error message '%s' but got '%s'", tc.ExpectedErr, errMsg)
				}
			} else {
				if _, ok := result["error"]; ok {
					t.Errorf("Unexpected error: %s", result["error"])
				}
			}
		})
	}
}

func TestConcurrentTransactions(t *testing.T) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []string

	// 並行リクエストの数を設定
	concurrentRequests := 10

	// 指定された数の並行リクエストを実行
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// 現在時刻のタイムスタンプ情報を取得
			timestamp := time.Now().Format("20060102150405")

			// リクエストボディを作成
			requestBody := fmt.Sprintf(`{"sender_id": "user1", "receiver_id": "user2", "amount": 100, "transaction_id": "test_concurrent_%d_%s"}`, index, timestamp)

			// リクエストを送信
			resp, err := http.Post("http://localhost:8080/transaction", "application/json", bytes.NewBuffer([]byte(requestBody)))
			if err != nil {
				// リクエストの送信に失敗した場合、エラーメッセージを追加
				mu.Lock()
				errors = append(errors, fmt.Sprintf("Request %d failed: %v", index, err))
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			// レスポンスを解析
			var result map[string]string
			err = json.NewDecoder(resp.Body).Decode(&result)
			if err != nil {
				// レスポンスの解析に失敗した場合、エラーメッセージを追加
				mu.Lock()
				errors = append(errors, fmt.Sprintf("Request %d response decoding failed: %v", index, err))
				mu.Unlock()
				return
			}

			// レスポンスのステータスコードをチェック
			if resp.StatusCode != http.StatusOK {
				// ステータスコードが200以外の場合、エラーメッセージを追加
				mu.Lock()
				errors = append(errors, fmt.Sprintf("Request %d failed with status code %d: %s", index, resp.StatusCode, result["error"]))
				mu.Unlock()
			}
		}(i)
	}

	// 全てのリクエストが完了するまで待機
	wg.Wait()

	// エラーが発生した場合、テストを失敗させる
	if len(errors) > 0 {
		t.Errorf("Errors occurred during concurrent transactions:")
		for _, err := range errors {
			t.Errorf("- %s", err)
		}
	}
}
