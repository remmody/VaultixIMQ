package app

import (
	"fmt"
	"sort"

	"github.com/remmody/VaultixIMQ/internal/mail"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

func (c *Core) GetClient(email string) (*client.Client, error) {
	c.Mu.Lock()
	if cli, ok := c.Clients[email]; ok {
		c.Mu.Unlock()
		if err := cli.Noop(); err == nil {
			return cli, nil
		}
		cli.Logout()
	} else {
		c.Mu.Unlock()
	}

	target, found := c.Accounts.Find(email)
	if !found {
		return nil, fmt.Errorf("account %s not found", email)
	}

	addr := fmt.Sprintf("%s:%s", target.Host, target.Port)
	nc, err := client.DialTLS(addr, nil)
	if err != nil {
		return nil, err
	}

	if err := nc.Login(target.Email, target.Password); err != nil {
		nc.Logout()
		return nil, err
	}

	c.Mu.Lock()
	c.Clients[email] = nc
	c.Mu.Unlock()

	return nc, nil
}

func (c *Core) FetchInbox(emailAddress string, limit int) ([]mail.Message, error) {
	mu := c.GetAccountMutex(emailAddress)
	mu.Lock()
	defer mu.Unlock()

	cli, err := c.GetClient(emailAddress)
	if err != nil {
		return nil, err
	}

	mbox, err := cli.Select("INBOX", false)
	if err != nil {
		return nil, err
	}

	if mbox.Messages == 0 {
		return []mail.Message{}, nil
	}

	from := uint32(1)
	if mbox.Messages > uint32(limit) {
		from = mbox.Messages - uint32(limit) + 1
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, mbox.Messages)

	messages := make(chan *imap.Message, limit)
	done := make(chan error, 1)
	go func() {
		done <- cli.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid, imap.FetchFlags}, messages)
	}()

	var mails []mail.Message
	for msg := range messages {
		dateStr := msg.Envelope.Date.Format("01/02 15:04")
		fromStr := ""
		if len(msg.Envelope.From) > 0 {
			fromStr = msg.Envelope.From[0].PersonalName
			if fromStr == "" {
				fromStr = msg.Envelope.From[0].MailboxName + "@" + msg.Envelope.From[0].HostName
			}
		}

		seen := false
		for _, f := range msg.Flags {
			if f == imap.SeenFlag {
				seen = true
				break
			}
		}

		mails = append(mails, mail.Message{
			UID:      msg.Uid,
			Subject:  c.DecodeHeader(msg.Envelope.Subject),
			From:     c.DecodeHeader(fromStr),
			Date:     dateStr,
			DateUnix: msg.Envelope.Date.Unix(),
			Seen:     seen,
		})
	}

	if err := <-done; err != nil {
		return nil, err
	}

	sort.Slice(mails, func(i, j int) bool {
		return mails[i].UID > mails[j].UID
	})

	return mails, nil
}

func (c *Core) MarkAsRead(email string, uid uint32) error {
	mu := c.GetAccountMutex(email)
	mu.Lock()
	defer mu.Unlock()

	cli, err := c.GetClient(email)
	if err != nil {
		return err
	}

	_, err = cli.Select("INBOX", false)
	if err != nil {
		return err
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)

	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.SeenFlag}

	return cli.UidStore(seqset, item, flags, nil)
}

func (c *Core) MarkAllAsRead(email string) error {
	mu := c.GetAccountMutex(email)
	mu.Lock()
	defer mu.Unlock()

	cli, err := c.GetClient(email)
	if err != nil {
		return err
	}

	_, err = cli.Select("INBOX", false)
	if err != nil {
		return err
	}

	// Search for all UNSEEN messages
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	uids, err := cli.UidSearch(criteria)
	if err != nil {
		return err
	}

	if len(uids) == 0 {
		return nil
	}

	// Chunk updates to prevent long command syntax errors (especially on Yandex)
	chunkSize := 100
	item := imap.FormatFlagsOp(imap.AddFlags, true)
	flags := []interface{}{imap.SeenFlag}

	for i := 0; i < len(uids); i += chunkSize {
		end := i + chunkSize
		if end > len(uids) {
			end = len(uids)
		}
		
		seqset := new(imap.SeqSet)
		for _, uid := range uids[i:end] {
			seqset.AddNum(uid)
		}

		err = cli.UidStore(seqset, item, flags, nil)
		if err != nil {
			return err
		}
	}

	// Update local cache
	c.Mu.Lock()
	if msgs, ok := c.Cache[email]; ok {
		// Only mark those UIDs that we actually processed as seen
		uidMap := make(map[uint32]bool)
		for _, uid := range uids {
			uidMap[uid] = true
		}
		for i := range msgs {
			if uidMap[msgs[i].UID] {
				msgs[i].Seen = true
			}
		}
	}
	c.Mu.Unlock()

	return nil
}
