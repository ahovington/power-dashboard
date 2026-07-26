// cmd/enphase-auth walks through the Enphase OAuth2 authorization code flow
// and prints the resulting access token and refresh token.
//
// Usage:
//
//	go run ./cmd/enphase-auth
//
// Environment:
//
//	ENPHASE_CLIENT_ID      (required)
//	ENPHASE_CLIENT_SECRET  (required)
//	ENPHASE_REDIRECT_URI   (optional, default http://localhost:8081/callback)
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ahovingtonpower-dashboard/pkg/enphase"
)

func main() {
	clientID := os.Getenv("ENPHASE_CLIENT_ID")
	clientSecret := os.Getenv("ENPHASE_CLIENT_SECRET")
	redirectURI := os.Getenv("ENPHASE_REDIRECT_URI")
	if redirectURI == "" {
		redirectURI = "http://localhost:8081/callback"
	}

	if clientID == "" || clientSecret == "" {
		slog.Error("ENPHASE_CLIENT_ID and ENPHASE_CLIENT_SECRET must be set")
		os.Exit(1)
	}

	authURL := enphase.AuthorizeURL(clientID, redirectURI)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Enphase OAuth2 Authorization")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("1. Open this URL in your browser:")
	fmt.Println()
	fmt.Println("  ", authURL)
	fmt.Println()
	fmt.Println("2. Log in with your Enphase account and grant access.")
	fmt.Println("3. You will be redirected to", redirectURI)
	fmt.Println("   (this server will capture the code automatically)")
	fmt.Println()

	// Start a local HTTP server to capture the callback
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error_description")
			if errMsg == "" {
				errMsg = r.URL.Query().Get("error")
			}
			http.Error(w, "No authorization code received: "+errMsg, http.StatusBadRequest)
			errCh <- fmt.Errorf("no code in callback: %s", errMsg)
			return
		}
		fmt.Fprintf(w, "<html><body><h2>Authorization successful!</h2><p>You can close this tab.</p></body></html>")
		codeCh <- code
	})

	srv := &http.Server{Addr: ":8081", Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server: %w", err)
		}
	}()

	fmt.Println("Waiting for callback on :8081 ...")

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		slog.Error("auth failed", "error", err)
		os.Exit(1)
	case <-time.After(5 * time.Minute):
		slog.Error("timed out waiting for authorization callback")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	fmt.Println("Exchanging authorization code for tokens...")
	tokens, err := enphase.ExchangeCode(clientID, clientSecret, code, redirectURI)
	if err != nil {
		slog.Error("token exchange failed", "error", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Success! Add these to your .env file:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("ENPHASE_ACCESS_TOKEN=%s\n", tokens.AccessToken)
	fmt.Printf("ENPHASE_REFRESH_TOKEN=%s\n", tokens.RefreshToken)
	fmt.Printf("\n# Access token expires in %d seconds (~%d hours)\n",
		tokens.ExpiresIn, tokens.ExpiresIn/3600)
	fmt.Println()
}
