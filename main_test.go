package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

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
			requestBody := fmt.Sprintf(`{"sender_id": "user1", "receiver_id": "user2", "amount": 100, "transaction_id": "test_%d_%s"}`, index, timestamp)

			// リクエストを送信
			resp, err := http.Post("http://localhost:8080/transaction", "application/json", strings.NewReader(requestBody))
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
