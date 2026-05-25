package mailwatch

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"golang.org/x/sys/unix"
)

const imapServerAddr = "imap.gmail.com:993"

type ConnectionStatus int

const (
	StatusConnected    ConnectionStatus = iota
	StatusDisconnected
	StatusNeedsAuth
)

type UpdateEvent struct {
	Unseen uint32
	Status ConnectionStatus
}

type Client struct {
	account           Account
	unseen            uint32
	Update            chan UpdateEvent
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

	conn, err := net.DialTimeout("tcp", imapServerAddr, 10*time.Second)
	if err != nil {
		return nil, err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)

		if rawConn, err := tcpConn.SyscallConn(); err == nil {
			rawConn.Control(func(fd uintptr) {
				unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPIDLE, 5)
				unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPINTVL, 1)
				unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_KEEPCNT, 3)
				unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, 8000)
			})
		}
	}

	tlsConfig := &tls.Config{}
	if options.TLSConfig != nil {
		tlsConfig = options.TLSConfig.Clone()
	}
	if tlsConfig.ServerName == "" {
		tlsConfig.ServerName = "imap.gmail.com"
	}
	if tlsConfig.NextProtos == nil {
		tlsConfig.NextProtos = []string{"imap"}
	}

	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	c := imapclient.New(tlsConn, options)

	gmailToken := newOAuth2GmailToken(&m.account)
	err = gmailToken.HandleTokenExpiration()
	if err != nil {
		log.WithError(err).Error("cannot handle token expiration")
		if IsNetworkError(err) {
			return nil, fmt.Errorf("cannot connect to auth server: %w", err)
		}
		return nil, ErrNeedsAuth
	}
	token := gmailToken.Token()
	if token == nil {
		return nil, ErrNeedsAuth
	}

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
	b := backoff.NewExponentialBackOff()

	for {
		if !isNetworkAvailable() {
			m.Update <- UpdateEvent{Status: StatusDisconnected}
			wait := b.NextBackOff()
			log.WithField("time", wait.String()).Debug("wait for restart")
			time.Sleep(wait)
			continue
		}

		b.Reset()

		startTime := time.Now()
		err := m.listen()

		if err == nil {
			continue
		}

		log.WithError(err).Warn("email client stopped")

		if time.Since(startTime) > time.Minute*5 {
			b.Reset()
		}

		switch {
		case errors.Is(err, ErrNeedsAuth):
			m.Update <- UpdateEvent{Status: StatusNeedsAuth}
		default:
			m.Update <- UpdateEvent{Status: StatusDisconnected}
		}

		wait := b.NextBackOff()
		log.WithField("time", wait.String()).Debug("wait for restart")
		time.Sleep(wait)
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
			m.Update <- UpdateEvent{Unseen: m.unseen, Status: StatusConnected}
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
	m.Update <- UpdateEvent{Unseen: m.unseen, Status: StatusConnected}

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
		Update:  make(chan UpdateEvent),
		unseen:  0,
	}
	return mailClient, nil
}
