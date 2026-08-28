package monapay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

type WebhookResult struct {
	OK      bool
	Reason  string
	Payload any
}

// VerifyWebhook checks timestamp + HMAC against the exact request bytes and parses JSON.
// toleranceSeconds defaults to 300 when omitted.
func VerifyWebhook(rawBody []byte, timestamp, signature, secret string, toleranceSeconds ...int) (WebhookResult, error) {
	tolerance := 300
	if len(toleranceSeconds) > 1 {
		return WebhookResult{}, errors.New("chỉ được truyền một tolerance")
	}
	if len(toleranceSeconds) == 1 {
		tolerance = toleranceSeconds[0]
	}
	if tolerance < 0 {
		return WebhookResult{}, errors.New("tolerance phải là số không âm")
	}
	if timestamp == "" {
		return WebhookResult{Reason: "missing_timestamp"}, nil
	}
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || strings.HasPrefix(timestamp, "+") || strings.HasPrefix(timestamp, "-") {
		return WebhookResult{Reason: "invalid_timestamp"}, nil
	}
	now := time.Now().Unix()
	drift := now - unix
	if drift < 0 {
		drift = -drift
	}
	if drift > int64(tolerance) {
		return WebhookResult{Reason: "timestamp_out_of_tolerance"}, nil
	}
	if signature == "" {
		return WebhookResult{Reason: "missing_signature"}, nil
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(rawBody)
	expected := mac.Sum(nil)
	supplied := make([]byte, sha256.Size)
	validFormat := strings.HasPrefix(signature, "sha256=") && len(signature) == 71
	if validFormat {
		decoded, decodeErr := hex.DecodeString(signature[7:])
		if decodeErr == nil && len(decoded) == sha256.Size {
			copy(supplied, decoded)
		} else {
			validFormat = false
		}
	}
	if !hmac.Equal(expected, supplied) || !validFormat {
		return WebhookResult{Reason: "invalid_signature"}, nil
	}
	var payload any
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return WebhookResult{Reason: "invalid_json"}, nil
	}
	return WebhookResult{OK: true, Payload: payload}, nil
}
