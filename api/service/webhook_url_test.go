package service

import "testing"

// IP-literal hosts exercise the classification logic without a real DNS
// lookup, so these stay hermetic and fast.
func TestValidateWebhookURL_AcceptsAPubliclyRoutableIPLiteral(t *testing.T) {
	if err := ValidateWebhookURL("https://93.184.216.34/hook"); err != nil {
		t.Fatalf("expected a public IP literal to be accepted, got: %v", err)
	}
}

func TestValidateWebhookURL_RejectsNonHTTPS(t *testing.T) {
	if err := ValidateWebhookURL("http://93.184.216.34/hook"); err == nil {
		t.Fatal("expected http to be rejected")
	}
}

func TestValidateWebhookURL_RejectsEmbeddedCredentials(t *testing.T) {
	if err := ValidateWebhookURL("https://user:pass@93.184.216.34/hook"); err == nil {
		t.Fatal("expected embedded credentials to be rejected")
	}
}

func TestValidateWebhookURL_RejectsMalformedURL(t *testing.T) {
	if err := ValidateWebhookURL("://not a url"); err == nil {
		t.Fatal("expected a malformed URL to be rejected")
	}
}

func TestValidateWebhookURL_RejectsPrivateAndInternalAddresses(t *testing.T) {
	// A consumer webhook URL is arbitrary and cannot be allowlisted by host,
	// unlike an artifact URL — so this must reject by address class instead.
	cases := []string{
		"https://127.0.0.1/hook",       // loopback
		"https://[::1]/hook",           // loopback (IPv6)
		"https://10.0.0.5/hook",        // private
		"https://192.168.1.1/hook",     // private
		"https://172.16.0.1/hook",      // private
		"https://169.254.169.254/hook", // link-local (cloud metadata endpoint)
		"https://0.0.0.0/hook",         // unspecified
		"https://224.0.0.1/hook",       // multicast
	}
	for _, raw := range cases {
		if err := ValidateWebhookURL(raw); err == nil {
			t.Errorf("expected %q to be rejected as a private/internal address", raw)
		}
	}
}
