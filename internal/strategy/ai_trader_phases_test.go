package strategy

import (
	"strings"
	"testing"
)

func TestCollectBufferDigest(t *testing.T) {
	b := newAITraderCollectBuffer()
	b.appendBook(&AITraderFeatures{Mid: 100, SpreadBPS: 5, Imbalance: 0.1})
	b.appendBook(&AITraderFeatures{Mid: 101, SpreadBPS: 4, Imbalance: 0.2})
	d := b.digestForLLM()
	if d == "" || !strings.Contains(d, "book samples") {
		t.Fatalf("unexpected digest: %s", d)
	}
}

func TestShouldRunAITraderLLMBlocksCollecting(t *testing.T) {
	r := &Runner{}
	s := &AITraderSession{
		Phase:         AITraderPhaseCollecting,
		PhaseProgress: defaultAITraderPhaseProgress(),
	}
	if aiTraderLLMEnabled() && r.shouldRunAITraderLLM(s) {
		t.Fatal("expected no LLM during collecting phase")
	}
}
