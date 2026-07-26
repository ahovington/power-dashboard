package enphase

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const tokenURL = "https://api.enphaseenergy.com/oauth/token"

// TokenResponse is returned by the OAuth2 token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// AuthorizeURL returns the URL the user must visit to grant access.
// redirectURI must match what is registered in the Enphase developer portal.
func AuthorizeURL(clientID, redirectURI string) string {
	return fmt.Sprintf(
		"https://api.enphaseenergy.com/oauth/authorize?response_type=code&client_id=%s&redirect_uri=%s",
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
	)
}

// ExchangeCode exchanges an authorization code for access + refresh tokens.
func ExchangeCode(clientID, clientSecret, code, redirectURI string) (*TokenResponse, error) {
	return postToken(clientID, clientSecret, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
	})
}

// RefreshAccessToken uses a refresh token to obtain a new access token.
func RefreshAccessToken(clientID, clientSecret, refreshToken string) (*TokenResponse, error) {
	return postToken(clientID, clientSecret, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

func postToken(clientID, clientSecret string, body url.Values) (*TokenResponse, error) {
	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("enphase oauth: build request: %w", err)
	}
	creds := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("enphase oauth: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
			Desc  string `json:"error_description"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return nil, fmt.Errorf("enphase oauth: HTTP %d: %s — %s", resp.StatusCode, e.Error, e.Desc)
	}

	var t TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return nil, fmt.Errorf("enphase oauth: decode response: %w", err)
	}
	return &t, nil
}
