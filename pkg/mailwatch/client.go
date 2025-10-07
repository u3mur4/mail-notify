package mailwatch

import (
	"io"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type Client struct {
	account           Account
	unseen            uint32
	Update            chan uint32
	cachedClient      *client.Client
	clientDebugOutput io.Writer
}

func (m *Client) getCachedImapClient() (*client.Client, error) {
	log.Debug("getting cached client")
	// if theres is a cached client return
	if m.cachedClient != nil {
		log.Debug("cached client found")
		m.cachedClient.Noop()
		return m.cachedClient, nil
	}

	// no cache client, create a new one
	c, err := m.createNewImapClient()
	if err != nil {
		log.WithError(err).Warn("cannot create a new cached client")
		return nil, err
	}
	
	m.cachedClient = c
	m.cachedClient.Noop()

	// if the connection to the server is closed create a new one
	go func(m *Client) {
		<-m.cachedClient.LoggedOut()
		log.Debug("cached client connection to the server is closed")
		m.cachedClient = nil
		m.getCachedImapClient()
	}(m)

	// return the cached client
	return m.cachedClient, nil
}

func (m *Client) SetDebug(output io.Writer) {
	m.clientDebugOutput = output
	if m.cachedClient != nil {
		m.cachedClient.SetDebug(output)
	}
}

func (m *Client) createNewImapClient() (*client.Client, error) {
	// Connect to imap server
	imapServer := "imap.gmail.com:993"

	c, err := client.DialTLS(imapServer, nil)
	if err != nil {
		log.WithError(err).Error("cannot connet to imap server")
		return nil, err
	}

	if m.clientDebugOutput != nil {
		c.SetDebug(m.clientDebugOutput)
	}

	gmailToken := newOAuth2GmailToken(&m.account)
	gmailToken.Token()
	err = gmailToken.HandleTokenExpiration()
	if err != nil {
		log.WithError(err).Error("cannot handle token expiration")
	}
	token := gmailToken.Token()

	sc := NewOAuth2Client(m.account.Username, token.AccessToken)

	err = c.Authenticate(sc)
	if err != nil {
		log.WithError(err).Error("cannot authenticate")
		return nil, err
	}

	// Select a mailbox
	if _, err := c.Select("INBOX", false); err != nil {
		log.WithError(err).Debug("cannot select mailbox")
		return nil, err
	}

	return c, nil
}

func (m *Client) Listen() error {
	b := backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5)

	for {
		startTime := time.Now()
		err := m.listen()
		if err != nil {
			log.WithError(err).Warn("email client stopped")
		}

		// if the email client is running at least for 5 minutes
		if time.Since(startTime) > time.Second*5 {
			b.Reset()
		}

		var next time.Duration
		if next = b.NextBackOff(); next == backoff.Stop {
			return err
		}
		log.WithField("time", next.String()).Debug("wait for restart")
		time.Sleep(next)
		log.Debug("restaring mail client")
	}
}

func (m *Client) listen() error {
	c, err := m.createNewImapClient()
	if err != nil {
		log.WithError(err).Debug("cannot create new imap client")
		return err
	}

	// Get initial value
	log.Debug("get initial unseen mails")
	unseen, err := m.getUnseen(c)
	if err != nil {
		return err
	}
	m.unseen = unseen
	m.Update <- m.unseen

	// Create a channel to receive mailbox updates
	updates := make(chan client.Update)
	c.Updates = updates

	// Start idling
	done := make(chan error)
	go func() {
		err := c.Idle(nil, nil)
		log.WithError(err).Debug("stop idling")
		done <- err
	}()

	// make sure the cached imap client is exists
	m.getCachedImapClient()

	// Listen for updates
	log.Debug("start listening for updates")
	for {
		select {
		case <-updates:
			log.Debug("new mailbox event")
			uc, err := m.getCachedImapClient()
			if err != nil {
				log.WithError(err).Debug("cannot create new imap client")
				return err
			}
			unseen, err := m.getUnseen(uc)
			if err != nil {
				log.WithError(err).Debug("cannot get unseen emails")
			}
			if m.unseen != unseen {
				log.WithField("unseen", unseen).Debug("unseen mails number changed")
				m.unseen = unseen
				m.Update <- m.unseen
			}
		case err := <-done:
			log.WithError(err).Debug("stop listening for updates")
			return err
		}
	}
}

func (m *Client) getUnseen(c *client.Client) (uint32, error) {
	log.Debug("getting unseen emails")

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	ids, err := c.Search(criteria)
	if err != nil {
		log.WithError(err).Warning("cannot search for unseen emails")
		return 0, err
	}

	log.WithField("unseen", len(ids)).Debug("got unseen emails")

	return uint32(len(ids)), nil
}

func NewMailClient(account Account) (Client, error) {
	mailClient := Client{
		account: account,
		Update:  make(chan uint32),
		unseen:  0,
	}
	return mailClient, nil
}
