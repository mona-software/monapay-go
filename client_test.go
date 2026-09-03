package monapay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func envelope(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestClientCachesTokenAndSendsHeaders(t *testing.T) {
	var mu sync.Mutex
	loginCount := 0
	var requests []*http.Request
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		mu.Lock()
		requests = append(requests, r.Clone(r.Context()))
		mu.Unlock()
		if r.URL.Path == "/api/v1/oauth/token" {
			loginCount++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["grant_type"] != "client_credentials" || body["client_id"] != "client-id" || body["client_secret"] != "secret" {
				t.Errorf("oauth body = %#v", body)
			}
			envelope(w, 200, map[string]any{"success": true, "data": map[string]any{"access_token": "token-1", "expires_in": 3600}})
			return w.Result(), nil
		}
		envelope(w, 200, map[string]any{"success": true, "data": map[string]any{"id": "ok"}})
		return w.Result(), nil
	})

	client, err := NewClient(Config{ClientID: "client-id", ClientSecret: "secret", BaseURL: "https://example.test", HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Webhooks.Create(context.Background(), map[string]any{"name": "Shop"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Me(context.Background()); err != nil {
		t.Fatal(err)
	}
	if loginCount != 1 {
		t.Fatalf("login count = %d", loginCount)
	}
	if got := requests[1].Header.Get("Authorization"); got != "Bearer token-1" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := requests[1].Header.Get("X-Client-Secret"); got != "secret" {
		t.Fatalf("X-Client-Secret = %q", got)
	}
	if got := requests[2].Header.Get("X-Client-Secret"); got != "" {
		t.Fatalf("GET must not send secret: %q", got)
	}
}

func TestClientRefreshesOnceAfter401(t *testing.T) {
	loginCount, meCount := 0, 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		if r.URL.Path == "/api/v1/client/login" {
			loginCount++
			envelope(w, 200, map[string]any{"success": true, "data": map[string]any{"access_token": fmt.Sprintf("token-%d", loginCount)}})
			return w.Result(), nil
		}
		meCount++
		if meCount == 1 {
			envelope(w, 401, map[string]any{"detail": "expired"})
			return w.Result(), nil
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token-2" {
			t.Errorf("Authorization = %q", got)
		}
		envelope(w, 200, map[string]any{"success": true, "data": map[string]any{"username": "user"}})
		return w.Result(), nil
	})
	client, _ := NewClient(Config{Username: "user", Password: "pass", BaseURL: "https://example.test", HTTPClient: &http.Client{Transport: transport}})
	if _, err := client.Me(context.Background()); err != nil {
		t.Fatal(err)
	}
	if loginCount != 2 || meCount != 2 {
		t.Fatalf("login=%d me=%d", loginCount, meCount)
	}
}

func TestIteratorPaginatesAndStopsAtSinceID(t *testing.T) {
	pages := []string{}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		w := httptest.NewRecorder()
		if r.URL.Path == "/api/v1/client/login" {
			envelope(w, 200, map[string]any{"success": true, "data": map[string]any{"access_token": "token"}})
			return w.Result(), nil
		}
		if got := r.URL.Query().Get("virtual_account_number"); got != "MONA 01" {
			t.Errorf("VA = %q", got)
		}
		if got := r.URL.Query().Get("since_id"); got != "" {
			t.Errorf("unsupported since_id leaked to API: %q", got)
		}
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		items := []any{map[string]any{"id": "tx-3"}, map[string]any{"id": "tx-2"}}
		if page == "2" {
			items = []any{map[string]any{"id": "tx-1"}}
		}
		envelope(w, 200, map[string]any{"success": true, "data": map[string]any{"data": items, "last_page": 2}})
		return w.Result(), nil
	})
	client, _ := NewClient(Config{Username: "user", Password: "pass", BaseURL: "https://example.test", HTTPClient: &http.Client{Transport: transport}})
	iterator := client.Transactions.Iterate(context.Background(), "MONA 01", TransactionOptions{Limit: 2, SinceID: "tx-1"})
	ids := []string{}
	for iterator.Next() {
		ids = append(ids, iterator.Value().(map[string]any)["id"].(string))
	}
	if err := iterator.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(ids, ",") != "tx-3,tx-2" {
		t.Fatalf("ids = %v", ids)
	}
	if strings.Join(pages, ",") != "1,2" {
		t.Fatalf("pages = %v", pages)
	}
}

func TestVerifyWebhook(t *testing.T) {
	raw := []byte(`{"amount":2500000,"transaction_code":"FT1"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write([]byte(timestamp + "."))
	mac.Write(raw)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	result, err := VerifyWebhook(raw, timestamp, signature, "test-secret")
	if err != nil || !result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	bad, err := VerifyWebhook(raw, timestamp, "sha256="+strings.Repeat("0", 64), "test-secret")
	if err != nil || bad.OK || bad.Reason != "invalid_signature" {
		t.Fatalf("bad=%+v err=%v", bad, err)
	}
	old, err := VerifyWebhook(raw, strconv.FormatInt(time.Now().Unix()-301, 10), signature, "test-secret", 300)
	if err != nil || old.Reason != "timestamp_out_of_tolerance" {
		t.Fatalf("old=%+v err=%v", old, err)
	}
}
