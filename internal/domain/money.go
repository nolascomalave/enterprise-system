package domain

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// Money is an exact monetary amount stored in cents. It avoids float for money
// and is used by Ent as a driver.Valuer/sql.Scanner over numeric(14,2).
type Money struct {
	Cents int64
}

func NewMoney(cents int64) Money {
	return Money{Cents: cents}
}

func (m Money) CentsValue() int64 {
	return m.Cents
}

func (m Money) IsNegative() bool {
	return m.Cents < 0
}

func (m Money) LessThan(other Money) bool {
	return m.Cents < other.Cents
}

func (m Money) GreaterThan(other Money) bool {
	return m.Cents > other.Cents
}

func (m Money) String() string {
	sign := ""
	cents := m.Cents
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// Value implements driver.Valuer.
func (m *Money) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return m.String(), nil
}

// Scan implements sql.Scanner.
func (m *Money) Scan(value any) error {
	if m == nil {
		return fmt.Errorf("money: scan into nil")
	}
	if value == nil {
		m.Cents = 0
		return nil
	}

	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	case int64:
		m.Cents = v * 100
		return nil
	case float64:
		m.Cents = int64(v*100 + 0.5)
		return nil
	default:
		return fmt.Errorf("money: unsupported scan type %T", value)
	}

	cents, err := parseMoneyCents(raw)
	if err != nil {
		return err
	}
	m.Cents = cents
	return nil
}

func parseMoneyCents(value string) (int64, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, fmt.Errorf("money: empty amount")
	}

	negative := strings.HasPrefix(raw, "-")
	raw = strings.TrimPrefix(raw, "-")
	raw = strings.TrimPrefix(raw, "+")

	parts := strings.SplitN(raw, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("money: invalid amount %q: %w", value, err)
	}

	var fraction int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 2 {
			frac = frac[:2]
		}
		for len(frac) < 2 {
			frac += "0"
		}
		fraction, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("money: invalid fraction %q: %w", value, err)
		}
	}

	cents := whole*100 + fraction
	if negative {
		cents = -cents
	}
	return cents, nil
}
