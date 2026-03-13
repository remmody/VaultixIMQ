package mail

import (
	"regexp"
	"strings"
)

type Message struct {
	UID     uint32   `json:"uid"`
	Subject string   `json:"subject"`
	From    string   `json:"from"`
	Date    string   `json:"date"`
	Body    string   `json:"body"`
	Seen    bool     `json:"seen"`
	Codes   []string `json:"codes"`
}

type Account struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Host     string `json:"imap_host"`
	Port     string `json:"imap_port"`
	// Legacy fields for migration
	OldHost string `json:"host,omitempty"`
	OldPort     string `json:"port,omitempty"`
	Label       string `json:"label"`
	UnreadCount int    `json:"unread_count"`
}

var IMAP_MAP = map[string][2]string{
	"gmail.com":      {"imap.gmail.com", "993"},
	"googlemail.com": {"imap.gmail.com", "993"},
	"yahoo.com":      {"imap.mail.yahoo.com", "993"},
	"outlook.com":    {"outlook.office365.com", "993"},
	"hotmail.com":    {"outlook.office365.com", "993"},
	"icloud.com":     {"imap.mail.me.com", "993"},
	"protonmail.com": {"127.0.0.1", "1143"},
	"fastmail.com":   {"imap.fastmail.com", "993"},
	"zoho.com":       {"imap.zoho.com", "993"},
	"mail.ru":        {"imap.mail.ru", "993"},
	"yandex.ru":      {"imap.yandex.ru", "993"},
	"yandex.com":     {"imap.yandex.com", "993"},
}

func (a *Account) Finalize() {
	a.Email = strings.TrimSpace(a.Email)
	a.Host = strings.TrimSpace(a.Host)
	a.Port = strings.TrimSpace(a.Port)
	a.OldHost = strings.TrimSpace(a.OldHost)
	a.OldPort = strings.TrimSpace(a.OldPort)

	if a.Host == "" && a.OldHost != "" {
		a.Host = a.OldHost
	}
	if a.Port == "" && a.OldPort != "" {
		a.Port = a.OldPort
	}
	
	if a.Host == "" || a.Port == "" {
		dom := strings.Split(a.Email, "@")
		if len(dom) > 1 {
			domain := strings.ToLower(strings.TrimSpace(dom[1]))
			if cfg, ok := IMAP_MAP[domain]; ok {
				if a.Host == "" { a.Host = cfg[0] }
				if a.Port == "" { a.Port = cfg[1] }
			} else {
				if a.Host == "" { a.Host = "imap." + domain }
				if a.Port == "" { a.Port = "993" }
			}
		}
	}

	if a.Label == "" {
		a.Label = strings.Split(a.Email, "@")[0]
	}
}

func StripHTML(s string) string {
	reStyle := regexp.MustCompile(`(?s)<style.*?>.*?</style>`)
	s = reStyle.ReplaceAllString(s, "")
	reScript := regexp.MustCompile(`(?s)<script.*?>.*?</script>`)
	s = reScript.ReplaceAllString(s, "")
	reTags := regexp.MustCompile("<[^>]*>")
	s = reTags.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&zwnj;", "")
	s = strings.ReplaceAll(s, "&#8195;", " ")
	return strings.TrimSpace(s)
}

func ExtractCodes(s string) []string {
	re := regexp.MustCompile(`\b\d{6}\b`)
	matches := re.FindAllString(s, -1)
	unique := make(map[string]bool)
	var res []string
	for _, m := range matches {
		if !unique[m] {
			unique[m] = true
			res = append(res, m)
		}
	}
	return res
}
