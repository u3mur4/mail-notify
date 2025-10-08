package mailwatch

import (
	"io"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type Client struct {
	account           Account
	unseen            uint32
	Update            chan uint32
	clientDebugOutput io.Writer
}

func (m *Client) SetDebug(output io.Writer) {
	m.clientDebugOutput = output
}

func (m *Client) createNewImapClient(options *imapclient.Options) (*imapclient.Client, error) {
	if options == nil {
		options = &imapclient.Options{}
	}
	options.DebugWriter = m.clientDebugOutput

	// Connect to imap server
	imapServer := "imap.gmail.com:993"

	c, err := imapclient.DialTLS(imapServer, options)


	if err != nil {
		log.WithError(err).Error("cannot connet to imap server")
		return nil, err
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
	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
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
		log.WithError(err).Warn("email client stopped")

		// if the email client is running at least for 5 minutes
		if time.Since(startTime) > time.Minute*5 {
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
	getUnseenAndUpdate := func() error {
		uc, err := m.createNewImapClient(nil)
		if err != nil {
			log.WithError(err).Debug("cannot create new imap client")
			return err
		}
		defer uc.Close()
		unseen, err := m.getUnseen(uc)
		if err != nil {
			log.WithError(err).Debug("cannot get unseen emails")
			return err
		}
		if m.unseen != unseen {
			log.WithField("unseen", unseen).Debug("unseen mails number changed")
			m.unseen = unseen
			m.Update <- m.unseen
		}
		return nil
	}

	var debounceTimer *time.Timer
	const debounceDelay = 250 * time.Millisecond
	mu := sync.Mutex{}

	debouncedGetUnseenAndUpdate := func() {
		mu.Lock()
		defer mu.Unlock()
		if debounceTimer != nil {
			return
		}
		debounceTimer = time.AfterFunc(debounceDelay, func() {
			getUnseenAndUpdate()
			debounceTimer = nil
		})
	}

	options := imapclient.Options{
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Expunge: func(seqNum uint32) {
				debouncedGetUnseenAndUpdate()
			},
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				debouncedGetUnseenAndUpdate()
			},
			Fetch: func(msg *imapclient.FetchMessageData) {
				debouncedGetUnseenAndUpdate()
			},
		},
	}

	c, err := m.createNewImapClient(&options)
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

	// Start idling
	idleCmd, err := c.Idle()
	if err != nil {
		return err
	}
	log.Debug("start listening for updates")
	return idleCmd.Wait()

}

func (m *Client) getUnseen(c *imapclient.Client) (uint32, error) {
	log.Debug("getting unseen emails")

	searchCmd := c.Search(&imap.SearchCriteria{
		NotFlag: []imap.Flag{imap.FlagSeen},
	}, &imap.SearchOptions{
		ReturnCount: true,
	})

	data, err := searchCmd.Wait()
	if err != nil {
		log.WithError(err).Warning("cannot search for unseen emails")
		return 0, err
	}

	log.WithField("unseen", data.Count).Debug("got unseen emails")
	return data.Count, nil
}

func NewMailClient(account Account) (Client, error) {
	mailClient := Client{
		account: account,
		Update:  make(chan uint32),
		unseen:  0,
	}
	return mailClient, nil
}
