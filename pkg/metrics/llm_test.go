package metrics

import "testing"

func TestLLMModelLabel(t *testing.T) {
	if got := LLMModelLabel("google/gemma-4-31b-it:free"); got != "google_gemma-4-31b-it_free" {
		t.Fatalf("got %q", got)
	}
	if LLMModelLabel("") != "unknown" {
		t.Fatal("empty model")
	}
}

func TestClassifyLLMError(t *testing.T) {
	if ClassifyLLMError(nil) != LLMResultError {
		t.Fatal("nil")
	}
	if ClassifyLLMError(errString("openrouter 429")) != LLMResultRateLimit {
		t.Fatal("429")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
