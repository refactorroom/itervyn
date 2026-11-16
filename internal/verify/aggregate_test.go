package verify

import "testing"

func TestAggregate(t *testing.T) {
	statuses := []Status{Pass, Fail, Inconclusive, Error}
	for _, a := range statuses {
		for _, b := range statuses {
			for _, required := range []bool{true, false} {
				want, action := Pass, Complete
				if a == Fail || required && b == Fail {
					want, action = Fail, Fix
				} else if a != Pass || required && b != Pass {
					want, action = Inconclusive, Investigate
				}
				results := []ProviderResult{{Name: "invaris", Required: true, Status: a}, {Name: "evaris", Required: required, Status: b}}
				got, next := Aggregate(results)
				if got != want || next != action {
					t.Fatalf("%+v: %s/%s want %s/%s", results, got, next, want, action)
				}
				results[0], results[1] = results[1], results[0]
				if got, next = Aggregate(results); got != want || next != action {
					t.Fatal("order changed decision")
				}
			}
		}
	}
	for _, rows := range [][]ProviderResult{nil, {{Name: "invaris", Status: Pass}}, {{Name: "invaris", Required: true, Status: Pass}, {Name: "invaris", Status: Pass}}, {{Name: "other", Required: true, Status: Pass}}, {{Name: "invaris", Required: true, Status: "MAYBE"}}} {
		if s, n := Aggregate(rows); s != Error || n != Stop {
			t.Fatalf("invalid set accepted: %+v", rows)
		}
	}
	if s, n := Aggregate([]ProviderResult{{Name: "invaris", Required: true, Status: Error, Reason: Ptr("CANCELED")}}); s != Inconclusive || n != Stop {
		t.Fatal(s, n)
	}
}
