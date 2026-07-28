// mail.go
package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (a *App) sendMail(to, subject, textBody, htmlBody string) error {
	mg := a.cfg.Mailgun

	if !mg.Enabled {
		log.Printf("[mailgun disabled] would send to=%s subject=%q", to, subject)
		return nil
	}
	if mg.Domain == "" || mg.APIKey == "" || mg.From == "" {
		return fmt.Errorf("mailgun not configured properly")
	}

	base := mg.BaseURL
	if base == "" {
		base = "https://api.mailgun.net/v3"
	}
	base = strings.TrimRight(base, "/")

	apiURL := fmt.Sprintf("%s/%s/messages", base, mg.Domain)

	data := url.Values{}
	data.Set("from", mg.From)
	data.Set("to", to)
	data.Set("subject", subject)
	data.Set("text", textBody)
	if htmlBody != "" {
		data.Set("html", htmlBody)
	}

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	// Mailgun API auth: username "api", password is your API key (exact string)
	req.SetBasicAuth("api", mg.APIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("mailgun returned status %s", resp.Status)
	}

	return nil
}
