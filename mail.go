package main

import (
	"time"

	"github.com/cenkalti/backoff"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type Account struct {
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	ImapServer string `mapstructure:"imap_server"`
	Exec       string `mapstructure:"exec"`
}

type MailClient struct {
	account      Account
	unseen       uint32
	Update       chan uint32
	cachedClient *client.Client
}

func (m *MailClient) getCachedImapClient() (*client.Client, error) {
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

	// if the connection to the server is closed create a new one
	go func(m *MailClient) {
		<-m.cachedClient.LoggedOut()
		log.Debug("cached client connection to the server is closed")
		m.cachedClient = nil
		m.getCachedImapClient()
	}(m)

	// return the cached client
	m.cachedClient = c
	m.cachedClient.Noop()
	return m.cachedClient, nil
}

func (m *MailClient) createNewImapClient() (*client.Client, error) {
	// Connect to imap server
	imapServer := "imap.gmail.com:993"
	if m.account.ImapServer != "" {
		imapServer = m.account.ImapServer
	}

	c, err := client.DialTLS(imapServer, nil)
	if err != nil {
		log.WithError(err).Error("cannot connet to imap server")
		return nil, err
	}

	// Login
	err = c.Login(m.account.Username, m.account.Password)
	if err != nil {
		log.WithError(err).Error("cannot login")
		return nil, err
	}

	// Select a mailbox
	if _, err := c.Select("INBOX", false); err != nil {
		log.WithError(err).Debug("cannot select mailbox")
		return nil, err
	}

	return c, nil
}

func (m *MailClient) Listen() error {
	b := backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5)

	for {
		startTime := time.Now()
		err := m.listen()
		if err != nil {
			log.WithError(err).Warn("email client stopped")
		}

		// if the email client is running at least for 5 minutes
		if time.Now().Sub(startTime) > time.Second*5 {
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

func (m *MailClient) listen() error {
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
	done := make(chan error, 1)
	go func() {
		done <- c.Idle(nil, nil)
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

func (m *MailClient) getUnseen(c *client.Client) (uint32, error) {
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

func NewMailClient(account Account) (MailClient, error) {
	mailClient := MailClient{
		account: account,
		Update:  make(chan uint32),
		unseen:  0,
	}
	return mailClient, nil
}
