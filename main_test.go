package main

import (
	"testing"
)

func TestReadValue(t *testing.T) {
	dr := &dataReader{}

	v := dr.readValue(`0-1:24.2.1(180630193501S)(01354.810*m3)`)
	if v != 1354.81 {
		t.Fail()
	}

	v = dr.readValue(`1-0:1.8.2(001427.007*kWh)`)
	if v != 1427.007 {
		t.Fail()
	}
}
