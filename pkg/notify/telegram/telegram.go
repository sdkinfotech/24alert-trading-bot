package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client sends messages via Telegram Bot API.
type Client struct {
	botToken string
	chatID   string
	http     *http.Client
}

// New returns a client or nil if token/chat are empty.
func New(botToken, chatID string) *Client {
	if strings.TrimSpace(botToken) == "" || strings.TrimSpace(chatID) == "" {
		return nil
	}
	return &Client{
		botToken: strings.TrimSpace(botToken),
		chatID:   strings.TrimSpace(chatID),
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

// SendMessage sends plain text (Markdown not escaped — caller should avoid raw user HTML).
func (c *Client) SendMessage(ctx context.Context, text string) error {
	if c == nil {
		return nil
	}
	u := "https://api.telegram.org/bot" + c.botToken + "/sendMessage"
	form := url.Values{}
	form.Set("chat_id", c.chatID)
	form.Set("text", text)
	form.Set("disable_web_page_preview", "true")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram sendMessage: %s: %s", resp.Status, string(body))
	}
	return nil
}
