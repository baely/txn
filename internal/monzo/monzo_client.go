// Package monzo handles transaction events and webhooks for the Monzo banking service
package monzo

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/baely/txn/internal/common/errors"
)

const monzoBaseURI = "https://api.monzo.com"

// MonzoClient handles API interactions with Monzo
type MonzoClient struct {
	accessToken string
	client      *http.Client
}

// NewMonzoClient creates a new client for the Monzo API
func NewMonzoClient(accessToken string) *MonzoClient {
	return &MonzoClient{
		accessToken: accessToken,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// request makes an API request to the Monzo API
func (c *MonzoClient) request(ctx context.Context, method, endpoint string, payload []byte, ret interface{}) error {
	var body *bytes.Buffer
	if payload != nil {
		body = bytes.NewBuffer(payload)
	} else {
		body = bytes.NewBuffer(nil)
	}

	uri := fmt.Sprintf("%s/%s", monzoBaseURI, endpoint)

	req, err := http.NewRequestWithContext(ctx, method, uri, body)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.accessToken))
	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to execute request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status: %d", resp.StatusCode)
	}

	if ret != nil {
		err = json.NewDecoder(resp.Body).Decode(ret)
		if err != nil {
			return errors.Wrap(err, "failed to decode response")
		}
	}

	return nil
}

// MonzoWebhookEvent represents a webhook event from Monzo
type MonzoWebhookEvent struct {
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data"`
	Account string          `json:"account_id"`
}

// GetAccount retrieves account details from Monzo
func (c *MonzoClient) GetAccount(ctx context.Context, accountID string) (Account, error) {
	var response struct {
		Accounts []struct {
			ID             string    `json:"id"`
			Created        time.Time `json:"created"`
			Description    string    `json:"description"`
			Type           string    `json:"type"`
			Currency       string    `json:"currency"`
			CountryCode    string    `json:"country_code"`
			AccountNumber  string    `json:"account_number"`
			SortCode       string    `json:"sort_code"`
			OwnerID        string    `json:"owner_id"`
			OwnerName      string    `json:"owner_name"`
			Closed         bool      `json:"closed"`
			AccountBalance int       `json:"balance"`
		} `json:"accounts"`
	}

	err := c.request(ctx, http.MethodGet, "accounts", nil, &response)
	if err != nil {
		return Account{}, err
	}

	for _, acc := range response.Accounts {
		if acc.ID == accountID {
			return Account{
				ID:       acc.ID,
				Created:  acc.Created,
				Currency: acc.Currency,
			}, nil
		}
	}

	return Account{}, errors.ErrNotFound
}

// GetTransaction retrieves transaction details from Monzo
func (c *MonzoClient) GetTransaction(ctx context.Context, transactionID string) (Transaction, error) {
	var response struct {
		Transaction struct {
			ID          string    `json:"id"`
			Created     time.Time `json:"created"`
			Description string    `json:"description"`
			Amount      int       `json:"amount"`
			Currency    string    `json:"currency"`
			MerchantID  string    `json:"merchant"`
			Notes       string    `json:"notes"`
			Metadata    struct {
				Category string `json:"category"`
			} `json:"metadata"`
		} `json:"transaction"`
	}

	endpoint := fmt.Sprintf("transactions/%s", transactionID)
	err := c.request(ctx, http.MethodGet, endpoint, nil, &response)
	if err != nil {
		return Transaction{}, err
	}

	tx := response.Transaction
	transaction := Transaction{
		ID:          tx.ID,
		Description: tx.Description,
		Amount:      tx.Amount,
		Created:     tx.Created,
		Category:    tx.Metadata.Category,
		MerchantID:  tx.MerchantID,
	}

	// If there's a merchant, get the merchant details
	if tx.MerchantID != "" {
		merchant, err := c.GetMerchant(ctx, tx.MerchantID)
		if err == nil {
			transaction.Merchant = merchant
		}
	}

	return transaction, nil
}

// GetMerchant retrieves merchant details from Monzo
func (c *MonzoClient) GetMerchant(ctx context.Context, merchantID string) (Merchant, error) {
	var response struct {
		Merchant Merchant `json:"merchant"`
	}

	endpoint := fmt.Sprintf("merchants/%s", merchantID)
	err := c.request(ctx, http.MethodGet, endpoint, nil, &response)
	if err != nil {
		return Merchant{}, err
	}

	return response.Merchant, nil
}

// ValidateWebhookEvent validates the signature of a webhook event
func ValidateWebhookEvent(payload []byte, signature string) bool {
	sig, _ := hex.DecodeString(signature)
	secret := os.Getenv("MONZO_WEBHOOK_SECRET")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	calculatedSignature := mac.Sum(nil)
	return hmac.Equal(sig, calculatedSignature)
}