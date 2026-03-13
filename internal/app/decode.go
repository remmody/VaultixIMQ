package app

import (
	"io"
	"mime"
	"mime/quotedprintable"
	"encoding/base64"
	"strings"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

func (c *Core) decodeBody(r io.Reader, encoding, charset string) (string, error) {
	var decoder io.Reader
	switch strings.ToLower(encoding) {
	case "quoted-printable":
		decoder = quotedprintable.NewReader(r)
	case "base64":
		decoder = base64.NewDecoder(base64.StdEncoding, r)
	default:
		decoder = r
	}

	if charset != "" && !strings.EqualFold(charset, "utf-8") && !strings.EqualFold(charset, "us-ascii") {
		e, err := htmlindex.Get(charset)
		if err == nil {
			decoder = transform.NewReader(decoder, e.NewDecoder())
		}
	}

	b, err := io.ReadAll(decoder)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *Core) DecodeHeader(s string) string {
	if s == "" {
		return ""
	}
	dec := mime.WordDecoder{
		CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
			e, err := htmlindex.Get(charset)
			if err != nil {
				return input, nil
			}
			return transform.NewReader(input, e.NewDecoder()), nil
		},
	}
	res, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return res
}
