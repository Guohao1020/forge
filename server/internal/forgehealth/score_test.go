package forgehealth

import "testing"

func TestScore(t *testing.T) {
	cases := []struct {
		name   string
		in     ScoreInput
		score  int
		status string
		noAct  bool
	}{
		// StandardsTotal,Checks,ReviewConfigs,Scans, GatePassed,GateFailed, ReviewTotal,ReviewCompleted, OpenFindings,FixPRsOpened
		{"all configured, perfect quality", ScoreInput{1, 1, 1, 1, 3, 0, 2, 2, 0, 1}, 100, "green", false},
		{"configured, no activity", ScoreInput{1, 1, 1, 1, 0, 0, 0, 0, 0, 0}, 100, "green", true},
		{"configured, gate all fail", ScoreInput{1, 1, 1, 1, 0, 3, 0, 0, 0, 0}, 40, "red", false},
		{"nothing", ScoreInput{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, 0, "red", true},
		{"live data", ScoreInput{1, 1, 1, 1, 1, 0, 1, 0, 0, 1}, 80, "green", false},
		{"half configured, half quality", ScoreInput{1, 1, 0, 0, 1, 1, 0, 0, 0, 0}, 50, "yellow", false},
		// entropyControl ratio path: open>0 → fixPRs/(open+fixPRs)=1/2=0.5; coverage=1.0; score=100*(0.4+0.6*0.5)=70.
		{"entropy ratio path", ScoreInput{1, 1, 1, 1, 0, 0, 0, 0, 1, 1}, 70, "yellow", false},
	}
	for _, c := range cases {
		got := Score(c.in)
		if got.Score != c.score || got.Status != c.status || got.NoActivity != c.noAct {
			t.Errorf("%s: got %+v, want score=%d status=%s noAct=%v", c.name, got, c.score, c.status, c.noAct)
		}
	}
}
