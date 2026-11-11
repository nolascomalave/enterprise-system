package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoneyString(t *testing.T) {
	tests := map[int64]string{
		0:      "0.00",
		5:      "0.05",
		12345:  "123.45",
		-12345: "-123.45",
		100:    "1.00",
	}
	for cents, want := range tests {
		t.Run(want, func(t *testing.T) {
			assert.Equal(t, want, NewMoney(cents).String())
		})
	}
}

func TestMoneyScan(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		cents  int64
		hasErr bool
	}{
		{name: "string", value: "123.45", cents: 12345},
		{name: "bytes", value: []byte("10.5"), cents: 1050},
		{name: "negative", value: "-0.05", cents: -5},
		{name: "integer cents", value: int64(12), cents: 1200},
		{name: "nil", value: nil, cents: 0},
		{name: "invalid", value: "abc", hasErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m Money
			err := m.Scan(tt.value)

			if tt.hasErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.cents, m.Cents)
		})
	}
}

func TestMoneyValue(t *testing.T) {
	m := NewMoney(12345)
	v, err := m.Value()

	require.NoError(t, err)
	assert.Equal(t, "123.45", v)
}

func TestMoneyComparison(t *testing.T) {
	low := NewMoney(100)
	high := NewMoney(200)

	assert.True(t, low.LessThan(high))
	assert.True(t, high.GreaterThan(low))
	assert.True(t, low.IsNegative() == false)
	assert.True(t, NewMoney(-1).IsNegative())
}

func TestParseMoney(t *testing.T) {
	money, err := ParseMoney("1250.50")
	require.NoError(t, err)
	assert.Equal(t, int64(125050), money.CentsValue())

	money, err = ParseMoney("0")
	require.NoError(t, err)
	assert.Equal(t, int64(0), money.CentsValue())

	_, err = ParseMoney("abc")
	require.Error(t, err)
}
