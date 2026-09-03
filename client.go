package monapay

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const DefaultBaseURL = "https://api.monapay.vn"

// Config configures a MONA Pay client.
type Config struct {
	ClientID     string
	Username     string
	Password     string
	ClientSecret string
	BaseURL      string
	HTTPClient   *http.Client
}

// APIError describes an error response or an invalid response from MONA Pay.
type APIError struct {
	Status int
	Body   any
	Msg    string
}

func (e *APIError) Error() string { return e.Msg }

// Client is a synchronous, safe-for-concurrent-use MONA Pay API client.
type Client struct {
	baseURL      string
	clientID     string
	username     string
	password     string
	httpClient   *http.Client
	mu           sync.Mutex
	accessToken  string
	tokenExpires time.Time
	clientSecret string

	Keys              *KeysResource
	VA                *VirtualAccountsResource
	BankAccounts      *BankAccountsResource
	PaymentProfile    *PaymentProfileResource
	Checkouts         *CheckoutsResource
	QR                *QRResource
	Transactions      *TransactionsResource
	Webhooks          *WebhooksResource
	WebhookLogs       *WebhookLogsResource
	Sandbox           *SandboxResource
	EmailConfigs      *EmailConfigsResource
	EmailLogs         *EmailLogsResource
	EmailSuppressions *EmailSuppressionsResource
}

func NewClient(config Config) (*Client, error) {
	hasClientCredentials := strings.TrimSpace(config.ClientID) != "" && strings.TrimSpace(config.ClientSecret) != ""
	hasPasswordCredentials := strings.TrimSpace(config.Username) != "" && strings.TrimSpace(config.Password) != ""
	if !hasClientCredentials && !hasPasswordCredentials {
		return nil, errors.New("cần client ID + client secret hoặc username + password; không dùng password cho AI agent vì sẽ gãy khi bật 2FA")
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("base URL không hợp lệ: %w", err)
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	c := &Client{
		baseURL:      baseURL,
		clientID:     config.ClientID,
		username:     config.Username,
		password:     config.Password,
		clientSecret: config.ClientSecret,
		httpClient:   httpClient,
	}
	c.Keys = &KeysResource{client: c}
	c.VA = &VirtualAccountsResource{client: c}
	c.BankAccounts = &BankAccountsResource{client: c}
	c.PaymentProfile = &PaymentProfileResource{client: c}
	c.Checkouts = &CheckoutsResource{client: c}
	c.QR = &QRResource{client: c}
	c.Transactions = &TransactionsResource{client: c}
	c.Webhooks = &WebhooksResource{client: c}
	c.WebhookLogs = &WebhookLogsResource{client: c}
	c.Sandbox = &SandboxResource{client: c}
	c.EmailConfigs = &EmailConfigsResource{client: c}
	c.EmailLogs = &EmailLogsResource{client: c}
	c.EmailSuppressions = &EmailSuppressionsResource{client: c}
	return c, nil
}

// NewClientFromEnv creates a client from MONAPAY_CLIENT_ID,
// MONAPAY_CLIENT_SECRET and optional MONAPAY_BASE_URL. Username/password are a
// legacy fallback only and should not be used by AI agents because 2FA breaks it.
func NewClientFromEnv() (*Client, error) {
	return NewClient(Config{
		ClientID: os.Getenv("MONAPAY_CLIENT_ID"), ClientSecret: os.Getenv("MONAPAY_CLIENT_SECRET"),
		Username: os.Getenv("MONAPAY_USERNAME"), Password: os.Getenv("MONAPAY_PASSWORD"),
		BaseURL: os.Getenv("MONAPAY_BASE_URL"),
	})
}

func (c *Client) Me(ctx context.Context) (any, error) {
	return c.request(ctx, http.MethodGet, "/api/v1/client/me", nil, nil)
}

func (c *Client) SetClientSecret(secret string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clientSecret = secret
}

func (c *Client) getAuth() (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accessToken, c.clientSecret
}

func (c *Client) login(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpires) {
		return nil
	}
	usingClientCredentials := c.clientID != "" && c.clientSecret != ""
	path := "/api/v1/client/login"
	body := map[string]any{"username": c.username, "password": c.password}
	if usingClientCredentials {
		path = "/api/v1/oauth/token"
		body = map[string]any{"grant_type": "client_credentials", "client_id": c.clientID, "client_secret": c.clientSecret}
	}
	data, err := c.send(ctx, http.MethodPost, path, body, nil, "", "")
	if err != nil {
		return err
	}
	object, ok := data.(map[string]any)
	if !ok {
		return &APIError{Msg: "Response đăng nhập không có access_token", Body: data}
	}
	token, _ := object["access_token"].(string)
	if token == "" {
		return &APIError{Msg: "Response đăng nhập không có access_token", Body: data}
	}
	c.accessToken = token
	expiresIn := 86400
	if usingClientCredentials {
		expiresIn = 3600
	}
	expiresIn = intValue(object["expires_in"], expiresIn)
	refreshIn := time.Duration(expiresIn)*time.Second - time.Minute
	if refreshIn < 0 {
		refreshIn = 0
	}
	c.tokenExpires = time.Now().Add(refreshIn)
	return nil
}

func (c *Client) request(ctx context.Context, method, path string, body any, query url.Values) (any, error) {
	return c.requestWithHeaders(ctx, method, path, body, query, nil)
}

func (c *Client) requestWithHeaders(ctx context.Context, method, path string, body any, query url.Values, headers http.Header) (any, error) {
	if err := c.login(ctx); err != nil {
		return nil, err
	}
	token, secret := c.getAuth()
	data, err := c.send(ctx, method, path, body, query, token, secret, headers)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		return data, err
	}

	// Refresh exactly once. Do not erase a token another goroutine already refreshed.
	c.mu.Lock()
	if c.accessToken == token {
		c.accessToken = ""
		c.tokenExpires = time.Time{}
	}
	c.mu.Unlock()
	if err := c.login(ctx); err != nil {
		return nil, err
	}
	token, secret = c.getAuth()
	return c.send(ctx, method, path, body, query, token, secret, headers)
}

func (c *Client) send(ctx context.Context, method, path string, body any, query url.Values, token, secret string, customHeaders ...http.Header) (any, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("không mã hóa được request JSON: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("không tạo được request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if token != "" && method != http.MethodGet && secret != "" {
		req.Header.Set("X-Client-Secret", secret)
	}
	if len(customHeaders) > 0 {
		for name, values := range customHeaders[0] {
			for _, value := range values {
				req.Header.Add(name, value)
			}
		}
	}
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("không kết nối được MONA Pay: %w", err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("không đọc được response MONA Pay: %w", err)
	}
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, &APIError{
			Status: response.StatusCode,
			Body:   string(raw),
			Msg:    fmt.Sprintf("MONA Pay trả response không phải JSON (HTTP %d)", response.StatusCode),
		}
	}
	success, hasSuccess := envelope["success"].(bool)
	if response.StatusCode < 200 || response.StatusCode >= 300 || (hasSuccess && !success) {
		message, _ := envelope["message"].(string)
		if message == "" {
			message, _ = envelope["detail"].(string)
		}
		if message == "" {
			message = fmt.Sprintf("MONA Pay API lỗi HTTP %d", response.StatusCode)
		}
		return nil, &APIError{Status: response.StatusCode, Body: envelope, Msg: message}
	}
	return envelope["data"], nil
}

func segment(value string) string { return url.PathEscape(value) }

func idempotencyKey(value string) (string, error) {
	if value != "" {
		return value, nil
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("không tạo được Idempotency-Key: %w", err)
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

func positive(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func intValue(value any, fallback int) int {
	switch number := value.(type) {
	case float64:
		return int(number)
	case json.Number:
		parsed, err := strconv.Atoi(number.String())
		if err == nil {
			return parsed
		}
	}
	return fallback
}
