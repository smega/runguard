package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ntfy (ntfy.sh or a self-hosted server). A template of this type holds the
// topic URL, https://<server>/<topic>, the same URL one would curl. RunGuard
// publishes JSON to the server's root instead of a raw body to the topic,
// so the phone shows a title, a priority taken from the job's severity and
// a tag, not a JSON dump. A query string on the topic URL (an access token,
// ?auth=...) is kept on the URL that gets the POST.

// ntfyTarget splits a topic URL into the URL to POST to and the topic.
func ntfyTarget(topicURL string) (postURL, topic string, err error) {
	u, err := url.Parse(strings.TrimSpace(topicURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", "", fmt.Errorf("ntfy: not a URL: %q", topicURL)
	}
	topic = strings.Trim(u.Path, "/")
	if topic == "" || strings.Contains(topic, "/") {
		return "", "", fmt.Errorf("ntfy: URL must be https://<server>/<topic>")
	}
	u.Path = "/"
	return u.String(), topic, nil
}

// ntfyPriority maps a job's severity onto ntfy's 1-5 scale.
func ntfyPriority(severity string) int {
	switch normalizeSeverity(severity) {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityLow:
		return 2
	default:
		return 3
	}
}

func buildNtfyBody(payload map[string]interface{}, topicURL string) ([]byte, error) {
	_, topic, err := ntfyTarget(topicURL)
	if err != nil {
		return nil, err
	}
	title, _ := payload["title"].(string)
	if title == "" {
		title = "RunGuard"
	}
	message, _ := payload["text"].(string)
	severity := ""
	if job, ok := payload["job"].(map[string]interface{}); ok {
		severity, _ = job["severity"].(string)
	}
	tag := "bell"
	switch payload["event"] {
	case "job_late":
		tag = "hourglass"
	case "job_failed":
		tag = "x"
	}
	return json.Marshal(map[string]interface{}{
		"topic":    topic,
		"title":    title,
		"message":  message,
		"priority": ntfyPriority(severity),
		"tags":     []string{tag},
	})
}

// webhookPostURL is where a webhook of the given type is sent: the URL as
// configured, except ntfy, which publishes to the server's root.
func webhookPostURL(tType, configured string) string {
	if strings.ToLower(tType) == WebhookTemplateTypeNtfy {
		if postURL, _, err := ntfyTarget(configured); err == nil {
			return postURL
		}
	}
	return configured
}
