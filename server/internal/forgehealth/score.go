// Package forgehealth computes a 0-100 Harness health score from the F5
// aggregate counts. Coverage-aware blend: configured-layer coverage + quality
// (gate pass / review completion / entropy control), with quality sub-scores
// excluded when they have no data — so a configured-but-unexercised Harness
// scores on coverage alone rather than reading as "broken". Pure, no I/O.
package forgehealth

import "math"

type ScoreInput struct {
	StandardsTotal  int32
	Checks          int32
	ReviewConfigs   int32
	Scans           int32
	GatePassed      int32
	GateFailed      int32
	ReviewTotal     int32
	ReviewCompleted int32
	OpenFindings    int32
	FixPRsOpened    int32
}

type ScoreResult struct {
	Score      int    // 0..100
	Status     string // "green" | "yellow" | "red"
	NoActivity bool   // configured but no quality data
}

func Score(in ScoreInput) ScoreResult {
	configured := 0
	for _, c := range []int32{in.StandardsTotal, in.Checks, in.ReviewConfigs, in.Scans} {
		if c > 0 {
			configured++
		}
	}
	coverage := float64(configured) / 4.0

	var quals []float64
	if in.GatePassed+in.GateFailed > 0 {
		quals = append(quals, float64(in.GatePassed)/float64(in.GatePassed+in.GateFailed))
	}
	if in.ReviewTotal > 0 {
		quals = append(quals, float64(in.ReviewCompleted)/float64(in.ReviewTotal))
	}
	if in.OpenFindings+in.FixPRsOpened > 0 {
		ec := 1.0
		if in.OpenFindings > 0 {
			ec = float64(in.FixPRsOpened) / float64(in.OpenFindings+in.FixPRsOpened)
		}
		quals = append(quals, ec)
	}

	noActivity := len(quals) == 0
	var raw float64
	if noActivity {
		raw = 100 * coverage
	} else {
		var sum float64
		for _, q := range quals {
			sum += q
		}
		meanQ := sum / float64(len(quals))
		raw = 100 * (0.4*coverage + 0.6*meanQ)
	}

	s := int(math.Round(raw))
	status := "red"
	switch {
	case s >= 80:
		status = "green"
	case s >= 50:
		status = "yellow"
	}
	return ScoreResult{Score: s, Status: status, NoActivity: noActivity}
}
