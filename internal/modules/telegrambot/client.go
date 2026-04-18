package telegrambot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	botToken   string
	httpClient *http.Client
}

func NewClient(baseURL, botToken string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		botToken: botToken,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && c.botToken != ""
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	if !c.Enabled() {
		return nil
	}
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	return c.call(ctx, "sendMessage", payload)
}

func (c *Client) SendWebAppButton(ctx context.Context, chatID int64, text, buttonText, webAppURL string) error {
	if !c.Enabled() {
		return nil
	}
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
		"reply_markup": map[string]any{
			"inline_keyboard": [][]map[string]any{{
				{
					"text": buttonText,
					"web_app": map[string]any{
						"url": webAppURL,
					},
				},
			}},
		},
	}
	return c.call(ctx, "sendMessage", payload)
}

func (c *Client) AnswerPreCheckoutQuery(ctx context.Context, queryID string, ok bool, errorMessage string) error {
	if !c.Enabled() {
		return nil
	}
	payload := map[string]any{
		"pre_checkout_query_id": queryID,
		"ok":                    ok,
	}
	if !ok && errorMessage != "" {
		payload["error_message"] = errorMessage
	}
	return c.call(ctx, "answerPreCheckoutQuery", payload)
}

func (c *Client) call(ctx context.Context, method string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/%s", c.baseURL, c.botToken, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram API returned status %d for method %s", resp.StatusCode, method)
	}

	return nil
}
