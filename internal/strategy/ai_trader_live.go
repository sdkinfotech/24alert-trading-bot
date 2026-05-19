package strategy

import (
	"fmt"
	"sync"

	"github.com/24alert/trading-bot/pkg/metrics"
)

// Live trading gates (armed_live still blocked; shadow mode for fill quality).
type aiTraderLiveConfig struct {
	mu              sync.RWMutex
	killSwitch      bool
	shadowMode      bool
	maxDailyLossRUB float64
}

var aiTraderLive aiTraderLiveConfig

func (r *Runner) aiTraderKillSwitchActive() bool {
	return aiTraderLive.killSwitchActive()
}

func (r *Runner) SetAITraderKillSwitch(on bool) {
	aiTraderLive.setKillSwitch(on)
	if on {
		metrics.AITraderKillSwitch.Set(1)
	} else {
		metrics.AITraderKillSwitch.Set(0)
	}
}

func (r *Runner) AITraderKillSwitch() bool {
	return r.aiTraderKillSwitchActive()
}

func (r *Runner) SetAITraderShadowMode(on bool) {
	aiTraderLive.setShadowMode(on)
}

func (r *Runner) AITraderShadowMode() bool {
	return aiTraderLive.shadowModeActive()
}

func (lc *aiTraderLiveConfig) setKillSwitch(on bool) {
	lc.mu.Lock()
	lc.killSwitch = on
	lc.mu.Unlock()
}

func (lc *aiTraderLiveConfig) killSwitchActive() bool {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.killSwitch
}

func (lc *aiTraderLiveConfig) setShadowMode(on bool) {
	lc.mu.Lock()
	lc.shadowMode = on
	lc.mu.Unlock()
}

func (lc *aiTraderLiveConfig) shadowModeActive() bool {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.shadowMode
}

// applyLiveRiskGate is called before any future live order (armed_live path).
func applyLiveRiskGate(s *AITraderSession, f *AITraderFeatures) error {
	if s == nil {
		return fmt.Errorf("no session")
	}
	if aiTraderLive.killSwitchActive() {
		return fmt.Errorf("kill switch active")
	}
	if f != nil && f.Stale {
		return fmt.Errorf("stale market data")
	}
	if f != nil && s.Limits.MaxSpreadBPS > 0 && f.SpreadBPS > s.Limits.MaxSpreadBPS {
		return fmt.Errorf("spread too wide: %.1f bps", f.SpreadBPS)
	}
	if s.PaperState != nil {
		net := s.PaperState.RealizedRUB + s.PaperState.UnrealizedRUB - s.PaperState.TotalFeesRUB
		if s.Limits.MaxDailyLossRUB > 0 && net <= -s.Limits.MaxDailyLossRUB {
			return fmt.Errorf("daily loss limit")
		}
	}
	return nil
}

// OrderControl placeholder: validates intent before broker (live path).
type AITraderOrderIntent struct {
	Side     string
	Price    float64
	Quantity int64
	Kind     string // limit | market
}

func validateOrderControl(intent AITraderOrderIntent, s *AITraderSession) error {
	if s == nil {
		return fmt.Errorf("no session")
	}
	if intent.Quantity <= 0 || intent.Quantity > int64(s.Limits.MaxOrderSize) {
		return fmt.Errorf("invalid quantity")
	}
	if intent.Side != "buy" && intent.Side != "sell" {
		return fmt.Errorf("invalid side")
	}
	return nil
}
