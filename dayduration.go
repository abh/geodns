package main

// Add a function similar to time.Duration.String() to
// pretty print an "uptime duration".

import (
	"time"
)

type DayDuration struct {
	time.Duration
}

func fmtInt(buf []byte, v uint64) int {
	w := len(buf)
	if v == 0 {
		w--
		buf[w] = '0'
	} else {
		for v > 0 {
			w--
			buf[w] = byte(v%10) + '0'
			v /= 10
		}
	}
	return w
}

// DayString returns string version of the time duration without too much precision.
// Copied from time/time.go
func (d DayDuration) DayString() string {
	var buf [32]byte
	w := len(buf)

	u := uint64(d.Nanoseconds())

	neg := d.Nanoseconds() < 0
	if neg {
		u = -u
	}

	if u < uint64(time.Second) {
		// Don't show times less than a second
		w -= 2
		buf[w] = '0'
		buf[w+1] = 's'
	} else {
		w--
		buf[w] = 's'

		// Skip fractional seconds
		u /= uint64(time.Second)

		// u is now integer seconds
		w = fmtInt(buf[:w], u%60)
		u /= 60

		// u is now integer minutes
		if u > 0 {
			w--
			buf[w] = ' '
			w--
			buf[w] = 'm'
			w = fmtInt(buf[:w], u%60)
			u /= 60

			// u is now integer hours
			if u > 0 {
				w--
				buf[w] = ' '
				w--
				buf[w] = 'h'
				w = fmtInt(buf[:w], u%24)
				u /= 24
			}

			// u is now integer days
			if u > 0 {
				w--
				buf[w] = ' '
				w--
				buf[w] = 'd'
				w = fmtInt(buf[:w], u)
			}

		}
	}

	if neg {
		w--
		buf[w] = '-'
	}

	return string(buf[w:])
}
