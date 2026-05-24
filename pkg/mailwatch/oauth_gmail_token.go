package mailwatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Define Gmail OAuth2 scope
var gmailScope = "https://mail.google.com/"

type oAuth2GmailToken struct {
	Account *Account
	token   *oauth2.Token
}

func newOAuth2GmailToken(account *Account) *oAuth2GmailToken {
	oauth := &oAuth2GmailToken{
		Account: account,
	}
	token, _ := oauth.tokenFromFile(account.TokenFilePath())
	oauth.token = token
	return oauth
}

func (o *oAuth2GmailToken) config() (*oauth2.Config, error) {
	configData, err := loadClientSecret()
	if err != nil {
		return nil, err
	}
	config, _ := google.ConfigFromJSON(configData, gmailScope)
	return config, nil
}

// Retrieves a token from the file, or gets a new one from the web.
func (o *oAuth2GmailToken) Token() *oauth2.Token {
	tok, err := o.tokenFromFile(o.Account.TokenFilePath())
	if err != nil {
		config, err := o.config()
		if err != nil {
			return nil
		}
		tok = getTokenFromWeb(config)
		o.saveToken(o.Account.TokenFilePath(), tok)
	}
	o.token = tok
	return tok
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Unable to start callback server: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	config.RedirectURL = fmt.Sprintf("http://localhost:%d/", port)
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser:\n%v\n", authURL)

	// Channel to receive the authorization code
	codeChan := make(chan string)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Get the authorization code from the URL parameters
		code := r.URL.Query().Get("code")
		if code != "" {
			// Send the authorization code to the channel
			codeChan <- code

			// Respond to the browser
			fmt.Fprintf(w, "Authorization code received. You can close this window.")
		} else {
			// Handle the case where no code is provided (error handling)
			fmt.Fprintf(w, "No authorization code received.")
		}
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Wait for the authorization code to be received
	authCode := <-codeChan

	// Shutdown the server after receiving the code
	go func() {
		time.Sleep(1 * time.Second) // Delay to let the user see the browser response
		if err := server.Shutdown(context.Background()); err != nil {
			log.Fatalf("Server Shutdown Failed:%+v", err)
		}
	}()

	// Exchange the authorization code for a token
	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}

	return tok
}

// tokenFromFile retrieves a token from a local file.
func (o *oAuth2GmailToken) tokenFromFile(file string) (*oauth2.Token, error) {
	data , err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	decryptedData, err := decrypt(data)
	if err != nil {
		return nil, err
	}

	tok := &oauth2.Token{}

	err = json.NewDecoder(bytes.NewReader(decryptedData)).Decode(tok)
	return tok, err
}

// saveToken saves a token to a local file.
func (o *oAuth2GmailToken) saveToken(path string, token *oauth2.Token) error {
	buffer := bytes.NewBuffer([]byte{})
	err := json.NewEncoder(buffer).Encode(token)
	if err != nil {
		return err
	}

	encryptedData, err := encrypt(buffer.Bytes())
	if err != nil {
		return err
	}

	err = os.WriteFile(path, encryptedData, 0600)
	if err != nil {
		return err
	}

	return nil
}

func (o *oAuth2GmailToken) HandleTokenExpiration() (err error) {
	// Create a TokenSource that automatically handles token expiration
	config, err := o.config()
	if err != nil {
		return err
	}

	tokenSource := config.TokenSource(context.Background(), o.token)

	newToken, err := tokenSource.Token() // This will automatically refresh the token if expired
	if err != nil {
		return err
	}

	if o.token == nil || newToken.AccessToken != o.token.AccessToken {
		o.token = newToken
		// Print current token and expiration time
		log.WithField("token", newToken.AccessToken).WithField("expires", newToken.Expiry).Info("token refreshed")

		// Save the refreshed token to file
		o.saveToken(o.Account.TokenFilePath(), newToken)
	}

	return nil
}

func AuthURLPath(account Account) string {
	return path.Join(configDir(), account.Username+"_auth_url")
}

func CheckAuth(account *Account) error {
	oauth := newOAuth2GmailToken(account)
	return oauth.HandleTokenExpiration()
}

func StartAuthFlow(account *Account) (needsAuth bool, authURL string, tokenChan <-chan *oauth2.Token, err error) {
	oauth := newOAuth2GmailToken(account)
	err = oauth.HandleTokenExpiration()
	if err == nil {
		return false, "", nil, nil
	}

	config, err := oauth.config()
	if err != nil {
		return false, "", nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return false, "", nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port

	config.RedirectURL = fmt.Sprintf("http://localhost:%d/", port)
	authURL = config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)

	ch := make(chan *oauth2.Token, 1)
	codeChan := make(chan string, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			codeChan <- code
			fmt.Fprintf(w, "Authorization code received. You can close this window.")
		} else {
			fmt.Fprintf(w, "No authorization code received.")
		}
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("callback server error")
		}
	}()

	go func() {
		authCode := <-codeChan
		go func() {
			time.Sleep(1 * time.Second)
			server.Shutdown(context.Background())
		}()
		tok, err := config.Exchange(context.Background(), authCode)
		if err != nil {
			log.WithError(err).Error("cannot exchange auth code for token")
			ch <- nil
			return
		}
		oauth.saveToken(account.TokenFilePath(), tok)
		ch <- tok
	}()

	return true, authURL, ch, nil
}

func ClientSecretPath() string {
	return path.Join(configDir(), "client_secret.json")
}

func ImportClientSecret(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	_, err = google.ConfigFromJSON(data, gmailScope)
	if err != nil {
		return err
	}

	encryptedData, err := encrypt(data)
	if err != nil {
		return err
	}

	err = os.WriteFile(ClientSecretPath(), encryptedData, 0644)
	if err != nil {
		return err
	}

	return nil
}

func loadClientSecret() ([]byte, error) {
	data, err := os.ReadFile(ClientSecretPath())
	if err != nil {
		return nil, err
	}

	return decrypt(data)
}
