package app

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	stdmail "net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/remmody/VaultixIMQ/internal/mail"
	"github.com/emersion/go-imap"
)

func (c *Core) FetchBody(emailAddress string, folder string, uid uint32) ([]interface{}, error) {
	mu := c.GetAccountMutex(emailAddress)
	mu.Lock()
	defer mu.Unlock()

	cli, err := c.GetClient(emailAddress)
	if err != nil {
		return nil, err
	}

	if folder == "" {
		folder = "INBOX"
	}
	cli.Select(folder, false)
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)

	section := &imap.BodySectionName{}
	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- cli.UidFetch(seqset, []imap.FetchItem{section.FetchItem()}, messages)
	}()

	var msg *imap.Message
	select {
	case m := <-messages:
		msg = m
	case err := <-done:
		if err != nil {
			return nil, err
		}
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("timeout fetching body")
	}

	if msg == nil {
		return nil, fmt.Errorf("message not found")
	}

	r := msg.GetBody(section)
	if r == nil {
		return nil, fmt.Errorf("could not get body")
	}

	m, err := stdmail.ReadMessage(r)
	if err != nil {
		bodyBytes, _ := io.ReadAll(r)
		return []interface{}{string(bodyBytes), nil}, nil
	}

	var plainBody, htmlBody string
	cidMap := make(map[string]string)
	contentType, params, _ := mime.ParseMediaType(m.Header.Get("Content-Type"))
	encoding := m.Header.Get("Content-Transfer-Encoding")
	charset := params["charset"]

	if strings.HasPrefix(contentType, "multipart/") {
		c.walkMultipart(m.Body, params["boundary"], &plainBody, &htmlBody, cidMap)
	} else {
		if contentType == "text/html" {
			htmlBody, _ = c.decodeBody(m.Body, encoding, charset)
		} else {
			plainBody, _ = c.decodeBody(m.Body, encoding, charset)
		}
	}

	if plainBody == "" && htmlBody != "" {
		plainBody = mail.StripHTML(htmlBody)
	}

	displayBody := ""
	if htmlBody != "" {
		for cid, dataURI := range cidMap {
			cleanCID := strings.Trim(cid, "<>")
			htmlBody = strings.ReplaceAll(htmlBody, "cid:"+cleanCID, dataURI)
		}
		displayBody = htmlBody
		
		// Sanitize
		reScript := regexp.MustCompile(`(?s)<script.*?>.*?</script>`)
		displayBody = reScript.ReplaceAllString(displayBody, "")

		if strings.Contains(strings.ToLower(displayBody), "<head>") {
			displayBody = strings.Replace(displayBody, "<head>", "<head><base target=\"_blank\">", 1)
		} else if strings.Contains(strings.ToLower(displayBody), "<html>") {
			displayBody = strings.Replace(displayBody, "<html>", "<html><head><base target=\"_blank\"></head>", 1)
		} else {
			displayBody = "<html><head><base target=\"_blank\"></head><body style=\"margin:0; padding:20px; font-family:sans-serif;\">" + displayBody + "</body></html>"
		}
	} else if plainBody != "" {
		displayBody = fmt.Sprintf(`<html><head><base target="_blank"></head><body style="margin:0; padding:20px; white-space: pre-wrap; word-break: break-all; font-family:sans-serif; background-color: white; color: black;">%s</body></html>`, plainBody)
	} else {
		displayBody = "<html><body style=\"margin:0; padding:20px; font-family:sans-serif; color: #666;\">(No content)</body></html>"
	}
	codes := mail.ExtractCodes(plainBody)
	return []interface{}{displayBody, codes}, nil
}

func (c *Core) walkMultipart(r io.Reader, boundary string, plainBody, htmlBody *string, cidMap map[string]string) {
	mr := multipart.NewReader(r, boundary)
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		contentType, params, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		encoding := p.Header.Get("Content-Transfer-Encoding")
		charset := params["charset"]
		contentID := p.Header.Get("Content-ID")

		if strings.HasPrefix(contentType, "multipart/") {
			c.walkMultipart(p, params["boundary"], plainBody, htmlBody, cidMap)
		} else if strings.HasPrefix(contentType, "image/") && contentID != "" {
			bodyBytes, err := io.ReadAll(p)
			if err == nil {
				var decoded []byte
				switch strings.ToLower(encoding) {
				case "base64":
					decoded, _ = base64.StdEncoding.DecodeString(string(bodyBytes))
				default:
					decoded = bodyBytes
				}
				
				if len(decoded) > 0 {
					encoded := base64.StdEncoding.EncodeToString(decoded)
					dataURI := fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
					cidMap[contentID] = dataURI
				}
			}
		} else {
			body, _ := c.decodeBody(p, encoding, charset)
			if body != "" {
				if contentType == "text/plain" && *plainBody == "" {
					*plainBody = body
				} else if contentType == "text/html" {
					*htmlBody = body
				}
			}
		}
	}
}
