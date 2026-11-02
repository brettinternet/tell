package delivery

import "testing"

// Herdr exits 0 when it declines to show; only its reason keeps the cascade
// moving. Shown and rate_limited lines are captured from Herdr 0.9.5.
func TestHerdrShowResult(t *testing.T) {
	for name, test := range map[string]struct {
		code            int
		stdout          string
		outcome, reason string
	}{
		"shown":                {0, `{"id":"cli:notification:show","result":{"reason":"shown","shown":true,"type":"notification_show"}}` + "\n", "accepted", ""},
		"rate limited":         {0, `{"id":"cli:notification:show","result":{"reason":"rate_limited","shown":false,"type":"notification_show"}}` + "\n", "not_sent", "rate_limited"},
		"disabled":             {0, `{"result":{"reason":"disabled","shown":false,"type":"notification_show"}}`, "not_sent", "disabled"},
		"no foreground client": {0, `{"result":{"reason":"no_foreground_client","shown":false,"type":"notification_show"}}`, "not_sent", "no_foreground_client"},
		"busy":                 {0, `{"result":{"reason":"busy","shown":false,"type":"notification_show"}}`, "not_sent", "busy"},
		"unknown not shown":    {0, `{"result":{"reason":"later","shown":false,"type":"notification_show"}}`, "not_sent", "not_shown"},
		"unknown shown":        {0, `{"result":{"reason":"later","shown":true,"type":"notification_show"}}`, "accepted", ""},
		"empty":                {0, "", "accepted", ""},
		"unparseable":          {0, "raw-provider-output\n", "accepted", ""},
		"exit failure":         {1, `{"result":{"reason":"shown","shown":true}}`, "failed", "provider_failure"},
	} {
		t.Run(name, func(t *testing.T) {
			output := &probeOutput{}
			_, _ = output.Write([]byte(test.stdout))
			result := herdrShowResult(test.code, output)
			reason := ""
			if result.Failure != nil {
				reason = result.Failure.Reason
			}
			if result.Outcome != test.outcome || reason != test.reason {
				t.Fatalf("got %s/%s want %s/%s", result.Outcome, reason, test.outcome, test.reason)
			}
		})
	}
}
