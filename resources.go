package monapay

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type KeysResource struct{ client *Client }

func (r *KeysResource) Generate(ctx context.Context, name string) (any, error) {
	if name == "" {
		name = "Default Key"
	}
	data, err := r.client.request(ctx, http.MethodPost, "/api/v1/client-keys/generate", map[string]any{"name": name}, nil)
	if err == nil {
		if object, ok := data.(map[string]any); ok {
			if secret, ok := object["client_secret"].(string); ok && secret != "" {
				_, current := r.client.getAuth()
				if current == "" {
					r.client.SetClientSecret(secret)
				}
			}
		}
	}
	return data, err
}

func (r *KeysResource) List(ctx context.Context) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/client-keys/list", nil, nil)
}

func (r *KeysResource) Destroy(ctx context.Context, keyID string) (any, error) {
	return r.client.request(ctx, http.MethodDelete, "/api/v1/client-keys/destroy/"+segment(keyID), nil, nil)
}

type VirtualAccountsResource struct{ client *Client }

func (r *VirtualAccountsResource) Register(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/virtual-account/registration", body, nil)
}

func (r *VirtualAccountsResource) Verify(ctx context.Context, requestID, code string) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/"+segment(requestID)+"/virtual-account/verification", map[string]any{"code": code}, nil)
}

func (r *VirtualAccountsResource) RegisterNotification(ctx context.Context, vaID string, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/"+segment(vaID)+"/notification/registration", body, nil)
}

func (r *VirtualAccountsResource) VerifyNotification(ctx context.Context, requestID, code string) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/"+segment(requestID)+"/notification/verification", map[string]any{"code": code}, nil)
}

func (r *VirtualAccountsResource) List(ctx context.Context, bankAccountID string) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/acb/"+segment(bankAccountID)+"/virtual-account/retrieve", nil, nil)
}

type BankAccountsResource struct{ client *Client }

func (r *BankAccountsResource) List(ctx context.Context) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/client/bank-accounts", nil, nil)
}

type QRResource struct{ client *Client }

func (r *QRResource) Generate(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/qr-payment/generate", body, nil)
}

func (r *QRResource) Cancel(ctx context.Context, qrCodeID string, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodDelete, "/api/v1/acb/qr-payment/"+segment(qrCodeID)+"/cancellation", body, nil)
}

type TransactionsResource struct{ client *Client }

type TransactionOptions struct {
	Page    int
	Limit   int
	SinceID string
}

func (r *TransactionsResource) List(ctx context.Context, virtualAccountNumber string, options TransactionOptions) (any, error) {
	if virtualAccountNumber == "" {
		return nil, fmt.Errorf("virtualAccountNumber là bắt buộc")
	}
	query := url.Values{}
	query.Set("virtual_account_number", virtualAccountNumber)
	query.Set("page", fmt.Sprint(positive(options.Page, 1)))
	query.Set("limit", fmt.Sprint(positive(options.Limit, 100)))
	// since_id is intentionally client-side: the current backend does not accept it.
	return r.client.request(ctx, http.MethodGet, "/api/v1/acb/virtual-account/transactions", nil, query)
}

func (r *TransactionsResource) Iterate(ctx context.Context, virtualAccountNumber string, options TransactionOptions) *TransactionIterator {
	return &TransactionIterator{
		ctx:      ctx,
		resource: r,
		vaNumber: virtualAccountNumber,
		page:     positive(options.Page, 1),
		limit:    positive(options.Limit, 100),
		sinceID:  options.SinceID,
	}
}

func (r *TransactionsResource) Retry(ctx context.Context, transactionID, targetType, targetID string) (any, error) {
	body := map[string]any{"target_type": targetType}
	if targetID != "" {
		body["target_id"] = targetID
	}
	return r.client.request(ctx, http.MethodPost, "/api/v1/acb/virtual-account/transactions/"+segment(transactionID)+"/retry", body, nil)
}

// TransactionIterator lazily retrieves transaction pages. Value is valid after Next returns true.
type TransactionIterator struct {
	ctx      context.Context
	resource *TransactionsResource
	vaNumber string
	page     int
	limit    int
	sinceID  string
	items    []any
	index    int
	current  any
	done     bool
	err      error
}

func (i *TransactionIterator) Next() bool {
	for {
		if i.index < len(i.items) {
			i.current = i.items[i.index]
			i.index++
			return true
		}
		if i.done || i.err != nil {
			return false
		}
		data, err := i.resource.List(i.ctx, i.vaNumber, TransactionOptions{Page: i.page, Limit: i.limit})
		if err != nil {
			i.err = err
			return false
		}
		page, ok := data.(map[string]any)
		if !ok {
			i.err = fmt.Errorf("response giao dịch không phải object")
			return false
		}
		i.items = i.items[:0]
		i.index = 0
		if rawItems, ok := page["data"].([]any); ok {
			for _, item := range rawItems {
				if i.sinceID != "" && transactionMatches(item, i.sinceID) {
					i.done = true
					break
				}
				i.items = append(i.items, item)
			}
		}
		if !i.done {
			hasNext, known := page["has_next"].(bool)
			if known {
				i.done = !hasNext
			} else {
				i.done = i.page >= intValue(page["last_page"], i.page)
			}
		}
		i.page++
	}
}

func (i *TransactionIterator) Value() any { return i.current }
func (i *TransactionIterator) Err() error { return i.err }

func transactionMatches(item any, sinceID string) bool {
	object, ok := item.(map[string]any)
	if !ok {
		return false
	}
	return fmt.Sprint(object["id"]) == sinceID || fmt.Sprint(object["transaction_code"]) == sinceID
}

type WebhooksResource struct{ client *Client }

func (r *WebhooksResource) List(ctx context.Context) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/client-webhooks", nil, nil)
}
func (r *WebhooksResource) Create(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/client-webhooks", body, nil)
}
func (r *WebhooksResource) Update(ctx context.Context, configID string, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPut, "/api/v1/client-webhooks/"+segment(configID), body, nil)
}
func (r *WebhooksResource) Remove(ctx context.Context, configID string) (any, error) {
	return r.client.request(ctx, http.MethodDelete, "/api/v1/client-webhooks/"+segment(configID), nil, nil)
}
func (r *WebhooksResource) Test(ctx context.Context, body map[string]any) (any, error) {
	return r.client.request(ctx, http.MethodPost, "/api/v1/client-webhooks/test", body, nil)
}

type WebhookLogsResource struct{ client *Client }

type WebhookLogOptions struct {
	Status   string
	FromDate string
	ToDate   string
	Page     int
	Limit    int
}

func webhookLogQuery(options WebhookLogOptions) url.Values {
	query := url.Values{}
	if options.Status != "" {
		query.Set("status", options.Status)
	}
	if options.FromDate != "" {
		query.Set("from_date", options.FromDate)
	}
	if options.ToDate != "" {
		query.Set("to_date", options.ToDate)
	}
	if options.Page > 0 {
		query.Set("page", fmt.Sprint(options.Page))
	}
	if options.Limit > 0 {
		query.Set("limit", fmt.Sprint(options.Limit))
	}
	return query
}

func (r *WebhookLogsResource) List(ctx context.Context, options WebhookLogOptions) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/webhook-logs", nil, webhookLogQuery(options))
}
func (r *WebhookLogsResource) Stats(ctx context.Context, options WebhookLogOptions) (any, error) {
	return r.client.request(ctx, http.MethodGet, "/api/v1/webhook-logs/stats", nil, webhookLogQuery(options))
}
