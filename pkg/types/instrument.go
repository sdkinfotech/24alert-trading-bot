package types

// InstrumentIdentifier helps work with different instrument identifiers
type InstrumentIdentifier struct {
	UID       string // Unique identifier (primary)
	FIGI      string // Financial Instrument Global Identifier
	Ticker    string // Trading ticker
	ClassCode string // Instrument class (SPBFUT, SPBOPT, etc)
}

// NewInstrumentIdentifier creates a new identifier
func NewInstrumentIdentifier(uid, figi, ticker, classCode string) *InstrumentIdentifier {
	return &InstrumentIdentifier{
		UID:       uid,
		FIGI:      figi,
		Ticker:    ticker,
		ClassCode: classCode,
	}
}

// IsStock checks if instrument is a stock
func (ii *InstrumentIdentifier) IsStock() bool {
	return ii.ClassCode == "SPYF" || ii.ClassCode == "TQBR"
}

// IsFuture checks if instrument is a future
func (ii *InstrumentIdentifier) IsFuture() bool {
	return ii.ClassCode == "SPBFUT"
}

// IsOption checks if instrument is an option
func (ii *InstrumentIdentifier) IsOption() bool {
	return ii.ClassCode == "SPBOPT"
}

// IsBond checks if instrument is a bond
func (ii *InstrumentIdentifier) IsBond() bool {
	return ii.ClassCode == "TQOB"
}

// IsCurrency checks if instrument is a currency pair
func (ii *InstrumentIdentifier) IsCurrency() bool {
	return ii.ClassCode == "CURS"
}

// IsETF checks if instrument is an ETF
func (ii *InstrumentIdentifier) IsETF() bool {
	return ii.ClassCode == "TQTF"
}

// InstrumentType returns a human-readable instrument type
func (ii *InstrumentIdentifier) InstrumentType() string {
	switch {
	case ii.IsStock():
		return "stock"
	case ii.IsFuture():
		return "future"
	case ii.IsOption():
		return "option"
	case ii.IsBond():
		return "bond"
	case ii.IsCurrency():
		return "currency"
	case ii.IsETF():
		return "etf"
	default:
		return "unknown"
	}
}

// PrimaryIdentifier returns the most reliable identifier (UID first, then FIGI, then ticker)
func (ii *InstrumentIdentifier) PrimaryIdentifier() string {
	if ii.UID != "" {
		return ii.UID
	}
	if ii.FIGI != "" {
		return ii.FIGI
	}
	return ii.Ticker
}

// String returns a string representation of the instrument
func (ii *InstrumentIdentifier) String() string {
	if ii.Ticker != "" {
		return ii.Ticker
	}
	return ii.PrimaryIdentifier()
}
