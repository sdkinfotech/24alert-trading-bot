package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

type apiResponse struct {
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

func apiURL(path string) string {
	return strings.TrimRight(serverURL, "/") + path
}

func doGet(path string) (json.RawMessage, error) {
	resp, err := httpClient.Get(apiURL(path))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	return parseResponse(resp)
}

func doPost(path string, body any) (json.RawMessage, error) {
	return doRequest("POST", path, body)
}

func doPut(path string, body any) (json.RawMessage, error) {
	return doRequest("PUT", path, body)
}

func doDelete(path string) (json.RawMessage, error) {
	req, err := http.NewRequest("DELETE", apiURL(path), nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	return parseResponse(resp)
}

func doRequest(method, path string, body any) (json.RawMessage, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, apiURL(path), bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	return parseResponse(resp)
}

func parseResponse(resp *http.Response) (json.RawMessage, error) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ar apiResponse
	if err := json.Unmarshal(raw, &ar); err == nil && ar.Data != nil {
		if ar.Error != "" {
			return nil, fmt.Errorf("API error: %s", ar.Error)
		}
		return ar.Data, nil
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func mustAccount() string {
	if accountID != "" {
		return accountID
	}
	id, err := autoDetectAccount()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot detect account: %v\nUse --account or TRADE_ACCOUNT\n", err)
		os.Exit(1)
	}
	accountID = id
	return accountID
}

func autoDetectAccount() (string, error) {
	data, err := doGet("/api/v1/accounts")
	if err != nil {
		return "", err
	}
	var accs []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &accs); err != nil {
		return "", err
	}
	if len(accs) == 0 {
		return "", fmt.Errorf("no accounts found")
	}
	return accs[0].ID, nil
}

func prettyJSON(data json.RawMessage) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		fmt.Println(string(data))
		return
	}
	fmt.Println(buf.String())
}

func fatal(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+msg+"\n", args...)
	os.Exit(1)
}
