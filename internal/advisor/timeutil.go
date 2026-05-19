package advisor

import "time"

var mskLoc *time.Location

func init() {
	var err error
	mskLoc, err = time.LoadLocation("Europe/Moscow")
	if err != nil {
		mskLoc = time.FixedZone("MSK", 3*3600)
	}
}

func MSK() *time.Location {
	return mskLoc
}

// AlignPeriodEnd returns the end of the period containing t (exclusive boundary for [start,end)).
func AlignPeriodEnd(t time.Time, tf Timeframe) time.Time {
	t = t.In(mskLoc)
	d := tf.Duration()
	if d >= 24*time.Hour {
		y, m, day := t.Date()
		return time.Date(y, m, day, 0, 0, 0, 0, mskLoc).Add(24 * time.Hour)
	}
	unix := t.Unix()
	secs := int64(d.Seconds())
	aligned := (unix / secs) * secs
	end := time.Unix(aligned, 0).In(mskLoc)
	if !end.After(t) {
		end = end.Add(d)
	}
	return end.UTC()
}

func PeriodStart(end time.Time, tf Timeframe) time.Time {
	return end.Add(-tf.Duration())
}

// LastClosedPeriodEnd is the most recent period boundary strictly before now (MSK-aligned).
func LastClosedPeriodEnd(now time.Time, tf Timeframe) time.Time {
	end := AlignPeriodEnd(now, tf)
	if now.Before(end) {
		return end.Add(-tf.Duration()).UTC()
	}
	return end.UTC()
}

// DuePeriodEnds returns period ends that closed since lastEnd up to now (exclusive now alignment).
func DuePeriodEnds(now time.Time, tf Timeframe, lastEnd time.Time) []time.Time {
	now = now.UTC()
	var out []time.Time
	if lastEnd.IsZero() {
		end := AlignPeriodEnd(now, tf)
		if end.Before(now) || end.Equal(now) {
			out = append(out, end)
		}
		return out
	}
	cursor := lastEnd
	for {
		cursor = cursor.Add(tf.Duration())
		end := AlignPeriodEnd(cursor, tf)
		if !end.After(lastEnd) {
			end = end.Add(tf.Duration())
		}
		if end.After(now) {
			break
		}
		if end.After(lastEnd) {
			out = append(out, end)
			lastEnd = end
		}
		if len(out) > 48 {
			break
		}
	}
	return out
}
