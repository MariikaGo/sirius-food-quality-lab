package domain

import (
	"encoding/json"
	"testing"
)

func TestMoney(t *testing.T) {
	for _, s := range []string{"0.01", "100.10", "50000"} {
		m, e := ParseMoney(s)
		if e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(m)
		var round Money
		if e = json.Unmarshal(b, &round); e != nil || round != m {
			t.Fatal(string(b), e)
		}
	}
	for _, s := range []string{"1.001", "NaN", "1e100"} {
		if _, e := ParseMoney(s); e == nil {
			t.Fatal("accepted", s)
		}
	}
}
