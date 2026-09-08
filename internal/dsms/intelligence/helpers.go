package intelligence

import (
	"strconv"
)

// fmtSscan parses a string to a numeric value.
func fmtSscan(s string, f *float64) error {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*f = val
	return nil
}
