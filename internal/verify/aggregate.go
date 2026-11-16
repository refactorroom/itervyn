package verify

// Aggregate never infers success from optional, absent, or unknown results.
func Aggregate(results []ProviderResult) (Status, NextAction) {
	seen := map[string]bool{}
	required, failed, missing, canceled := 0, false, false, false
	for _, r := range results {
		if (r.Name != "invaris" && r.Name != "evaris") || seen[r.Name] {
			return Error, Stop
		}
		seen[r.Name] = true
		if r.Status != Pass && r.Status != Fail && r.Status != Inconclusive && r.Status != Error {
			return Error, Stop
		}
		if r.Reason != nil && *r.Reason == "CANCELED" {
			canceled = true
		}
		if !r.Required {
			continue
		}
		required++
		failed = failed || r.Status == Fail
		missing = missing || r.Status != Pass
	}
	if required == 0 {
		return Error, Stop
	}
	if failed {
		return Fail, Fix
	}
	if missing {
		if canceled {
			return Inconclusive, Stop
		}
		return Inconclusive, Investigate
	}
	if canceled {
		return Error, Stop
	}
	return Pass, Complete
}
