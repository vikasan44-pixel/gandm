package service

import "testing"

func TestValidAttachmentURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"https", "https://example.com/file.pdf", true},
		{"http", "http://example.com/a?b=c", true},
		{"scheme uppercase", "HTTPS://example.com", true},
		{"javascript", "javascript:alert(document.cookie)", false},
		{"javascript mixed case", "JavaScript:alert(1)", false},
		{"data uri", "data:text/html,<script>alert(1)</script>", false},
		{"vbscript", "vbscript:msgbox(1)", false},
		{"no scheme relative", "/local/path", false},
		{"bare host no scheme", "example.com", false},
		{"http no host", "http:///onlypath", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validAttachmentURL(c.in); got != c.want {
				t.Errorf("validAttachmentURL(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
