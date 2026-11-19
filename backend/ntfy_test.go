package main

import (
	"encoding/json"
	"testing"
)

func TestNtfyTarget(t *testing.T) {
	cases := []struct{ in, post, topic string }{
		{"https://ntfy.sh/my-topic", "https://ntfy.sh/", "my-topic"},
		{"https://ntfy.example.com/alerts/?auth=abc", "https://ntfy.example.com/?auth=abc", "alerts"},
	}
	for _, c := range cases {
		post, topic, err := ntfyTarget(c.in)
		if err != nil || post != c.post || topic != c.topic {
			t.Errorf("ntfyTarget(%q) = %q, %q, %v; want %q, %q", c.in, post, topic, err, c.post, c.topic)
		}
	}
	for _, bad := range []string{"", "ntfy.sh/topic", "https://ntfy.sh/", "https://ntfy.sh/a/b"} {
		if _, _, err := ntfyTarget(bad); err == nil {
			t.Errorf("ntfyTarget(%q): want an error", bad)
		}
	}
}

func TestBuildNtfyBody(t *testing.T) {
	payload := map[string]interface{}{
		"title": "backup failed",
		"text":  "RunGuard: job 'backup' reported a failure.",
		"event": "job_failed",
		"job":   map[string]interface{}{"severity": "critical"},
	}
	b, err := buildNtfyBody(payload, "https://ntfy.sh/ops")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["topic"] != "ops" || got["title"] != "backup failed" || got["priority"] != float64(5) {
		t.Errorf("unexpected body: %s", b)
	}
	if tags, _ := got["tags"].([]interface{}); len(tags) != 1 || tags[0] != "x" {
		t.Errorf("tags = %v, want [x]", got["tags"])
	}
	if webhookPostURL("ntfy", "https://ntfy.sh/ops") != "https://ntfy.sh/" {
		t.Error("ntfy posts to the server root")
	}
	if webhookPostURL("generic", "https://example.com/hook") != "https://example.com/hook" {
		t.Error("other types post to the configured URL")
	}
}
