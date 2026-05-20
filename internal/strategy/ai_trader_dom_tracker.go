package strategy

import (
	"fmt"
	"strings"
	"time"
)

const aiTraderDOMTrackDepth = 20

// AITraderDOMEvent describes limit order book changes (add/pull/resize).
type AITraderDOMEvent struct {
	Time      string  `json:"time"`
	Side      string  `json:"side"` // bid | ask
	Price     float64 `json:"price"`
	Kind      string  `json:"kind"` // add | pull | increase | decrease
	PrevQty   int64   `json:"prev_qty"`
	NewQty    int64   `json:"new_qty"`
	Note      string  `json:"note,omitempty"`
}

type domLevelMap map[string]int64

func domKey(side string, price float64) string {
	return side + ":" + formatPriceKey(price)
}

func bookToLevelMap(side string, levels []AITraderBookLevel, depth int) domLevelMap {
	out := make(domLevelMap, depth)
	for i, lv := range levels {
		if i >= depth {
			break
		}
		if lv.Quantity > 0 {
			out[domKey(side, lv.Price)] = lv.Quantity
		}
	}
	return out
}

func (st *aiTraderContextState) trackDOMChangesLocked(book *AITraderDOMBook, now time.Time) {
	if book == nil {
		return
	}
	curBids := bookToLevelMap("bid", book.Bids, aiTraderDOMTrackDepth)
	curAsks := bookToLevelMap("ask", book.Asks, aiTraderDOMTrackDepth)
	ts := now.UTC().Format(time.RFC3339)

	emit := func(side string, price float64, kind string, prev, new int64, note string) {
		st.domEvents = append(st.domEvents, AITraderDOMEvent{
			Time: ts, Side: side, Price: price, Kind: kind,
			PrevQty: prev, NewQty: new, Note: note,
		})
		st.addSceneNoteLocked(fmt.Sprintf("DOM %s %s %.4f %d→%d %s", side, kind, price, prev, new, note))
	}

	compareSide := func(side string, cur, prev domLevelMap) {
		seen := map[string]bool{}
		for k, newQ := range cur {
			seen[k] = true
			oldQ := prev[k]
			_, price := parseDomKey(k)
			switch {
			case oldQ == 0 && newQ > 0:
				emit(side, price, "add", 0, newQ, "new limit")
			case oldQ > 0 && newQ == 0:
				emit(side, price, "pull", oldQ, 0, "level removed")
			case newQ > oldQ*2 && newQ-oldQ > 30:
				emit(side, price, "increase", oldQ, newQ, "size up")
			case oldQ > newQ*2 && oldQ-newQ > 30:
				emit(side, price, "decrease", oldQ, newQ, "size down")
			}
		}
		for k, oldQ := range prev {
			if seen[k] {
				continue
			}
			_, price := parseDomKey(k)
			emit(side, price, "pull", oldQ, 0, "level gone")
		}
	}

	if st.domPrevBids != nil {
		compareSide("bid", curBids, st.domPrevBids)
	}
	if st.domPrevAsks != nil {
		compareSide("ask", curAsks, st.domPrevAsks)
	}
	st.domPrevBids = curBids
	st.domPrevAsks = curAsks
	if len(st.domEvents) > 40 {
		st.domEvents = st.domEvents[len(st.domEvents)-40:]
	}
}

func parseDomKey(k string) (side string, price float64) {
	parts := strings.SplitN(k, ":", 2)
	if len(parts) != 2 {
		return "", 0
	}
	side = parts[0]
	fmt.Sscanf(parts[1], "%f", &price)
	return side, price
}
