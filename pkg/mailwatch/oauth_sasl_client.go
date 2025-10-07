package mailwatch

import (
	"fmt"

	"github.com/emersion/go-sasl"
)

// OAuth2Client implements the sasl.Client interface for OAuth2
type OAuth2Client struct {
	username    string
	accessToken string
	done        bool
}

// NewOAuth2Client creates a new OAuth2 SASL client
func NewOAuth2Client(username, accessToken string) sasl.Client {
	return &OAuth2Client{
		username:    username,
		accessToken: accessToken,
	}
}

// Start begins the OAuth2 SASL authentication process.
func (c *OAuth2Client) Start() (mech string, ir []byte, err error) {
	mech = "XOAUTH2"
	ir = []byte(fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", c.username, c.accessToken))
	return mech, ir, nil
}

// Next handles the server's challenge during the authentication process.
func (c *OAuth2Client) Next(challenge []byte) ([]byte, error) {
	// OAuth2 does not require additional challenge-response exchanges, so we're done.
	if c.done {
		return nil, sasl.ErrUnexpectedServerChallenge
	}
	c.done = true
	return nil, nil
}
