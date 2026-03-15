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
	DateUnix int64   `json:"date_unix"`
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
	OldPort string `json:"port"`
	Label string `json:"label"`
	Status string `json:"status"`
	UnreadCount int `json:"unread_count"`
	LastMessageTime int64 `json:"last_message_time"`
}

// Pre-compiled regexes to avoid recompilation on every call.
var (
	reStyle  = regexp.MustCompile(`(?s)<style.*?>.*?</style>`)
	reScript = regexp.MustCompile(`(?s)<script.*?>.*?</script>`)
	reTags   = regexp.MustCompile(`<[^>]*>`)
	reCodes  = regexp.MustCompile(`\b\d{6}\b`)
	reCodesDash = regexp.MustCompile(`\b[A-Za-z0-9]{3}-[A-Za-z0-9]{3}-[A-Za-z0-9]{4}\b`)
)

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
	a.trimFields()
	a.resolveLegacy()
	
	if a.Host == "" || a.Port == "" {
		a.lookupIMAP()
	}

	if a.Label == "" {
		a.Label = strings.Split(a.Email, "@")[0]
	}
}

func (a *Account) trimFields() {
	a.Email = strings.TrimSpace(a.Email)
	a.Host = strings.TrimSpace(a.Host)
	a.Port = strings.TrimSpace(a.Port)
	a.OldHost = strings.TrimSpace(a.OldHost)
	a.OldPort = strings.TrimSpace(a.OldPort)
}

func (a *Account) resolveLegacy() {
	if a.Host == "" && a.OldHost != "" {
		a.Host = a.OldHost
	}
	if a.Port == "" && a.OldPort != "" {
		a.Port = a.OldPort
	}
}

func (a *Account) lookupIMAP() {
	parts := strings.Split(a.Email, "@")
	if len(parts) <= 1 {
		return
	}
	domain := strings.ToLower(strings.TrimSpace(parts[1]))
	if cfg, ok := IMAP_MAP[domain]; ok {
		if a.Host == "" { a.Host = cfg[0] }
		if a.Port == "" { a.Port = cfg[1] }
	} else {
		if a.Host == "" { a.Host = "imap." + domain }
		if a.Port == "" { a.Port = "993" }
	}
}

func StripHTML(s string) string {
	s = reStyle.ReplaceAllString(s, "")
	s = reScript.ReplaceAllString(s, "")
	s = reTags.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&zwnj;", "")
	s = strings.ReplaceAll(s, "&#8195;", " ")
	return strings.TrimSpace(s)
}

func ExtractCodes(s string) []string {
	var matches []string
	matches = append(matches, reCodes.FindAllString(s, -1)...)
	matches = append(matches, reCodesDash.FindAllString(s, -1)...)

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
