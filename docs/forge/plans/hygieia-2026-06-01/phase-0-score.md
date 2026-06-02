## Phase 0 — 后端 Score 纯函数（TDD）+ handler 接线 + 响应字段

**Goal:** `forgehealth.Score` 纯函数(TDD)+ `GetForgeHealth` 响应加 `score`/`status`/`no_activity`。**无迁移**。
**Depends-on:** 无　**Unblocks:** Phase 1
**Completion gate:** `Score` 单测绿;`go build ./...` + vet 通过。

> Go 走 WSL;`git commit` 用原生 Windows git(无 `--no-verify`)。
> `GetForgeHealth` 在 `server/internal/handler/forge_health.go`,末尾 `writeJSON(w, http.StatusOK, out)` 前接线。

---

### Task 0.1: Score 纯函数（TDD）

**Files:**
- Create: `server/internal/forgehealth/score.go`
- Create: `server/internal/forgehealth/score_test.go`

- [ ] **Step 1: 写失败测试**

`server/internal/forgehealth/score_test.go`：
```go
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
	}
	for _, c := range cases {
		got := Score(c.in)
		if got.Score != c.score || got.Status != c.status || got.NoActivity != c.noAct {
			t.Errorf("%s: got %+v, want score=%d status=%s noAct=%v", c.name, got, c.score, c.status, c.noAct)
		}
	}
}
```
> "half configured, half quality" 验算:coverage=2/4=0.5;gatePass=1/2=0.5;其余质量无数据;quals=[0.5] mean=0.5;
> score=100*(0.4*0.5+0.6*0.5)=100*(0.2+0.3)=50 → yellow(50–79)。

- [ ] **Step 2: 运行确认失败**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forgehealth/... 2>&1 | tail -8"`
Expected: 编译失败(`ScoreInput`/`Score` 未定义)。

- [ ] **Step 3: 实现**

`server/internal/forgehealth/score.go`：
```go
// Package forgehealth computes a 0-100 Harness health score from the F5
// aggregate counts. Coverage-aware blend: configured-layer coverage + quality
// (gate pass / review completion / entropy control), with quality sub-scores
// excluded when they have no data — so a configured-but-unexercised Harness
// scores on coverage alone rather than reading as "broken". Pure, no I/O.
package forgehealth

import "math"

type ScoreInput struct {
	StandardsTotal int32
	Checks         int32
	ReviewConfigs  int32
	Scans          int32
	GatePassed     int32
	GateFailed     int32
	ReviewTotal    int32
	ReviewCompleted int32
	OpenFindings   int32
	FixPRsOpened   int32
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
```

- [ ] **Step 4: 运行确认通过**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go test ./internal/forgehealth/... 2>&1 | tail -8"`
Expected: PASS(6 个 case 全过)。

- [ ] **Step 5: Commit**

```bash
git add server/internal/forgehealth/
git commit -m "feat(forge): Harness health Score pure function (coverage-aware blend)"
```

---

### Task 0.2: handler 接线 + 响应字段

**Files:**
- Modify: `server/internal/handler/forge_health.go`

- [ ] **Step 1: 响应 struct 加字段**

在 `ForgeHealthResponse` struct 末尾(`FixPRs` 之后)加:
```go
	Score      int    `json:"score"`
	Status     string `json:"status"`
	NoActivity bool   `json:"no_activity"`
```

- [ ] **Step 2: import + 接线**

import 块加:
```go
	"github.com/multica-ai/multica/server/internal/forgehealth"
```
在 `GetForgeHealth` 的末尾 `writeJSON(w, http.StatusOK, out)` **之前**插入:
```go
	sr := forgehealth.Score(forgehealth.ScoreInput{
		StandardsTotal: out.StandardsTotal, Checks: out.Checks,
		ReviewConfigs: out.ReviewConfigs, Scans: out.Scans,
		GatePassed: out.Gate.Passed, GateFailed: out.Gate.Failed,
		ReviewTotal: out.Review.Total, ReviewCompleted: out.Review.Completed,
		OpenFindings: out.OpenFindings, FixPRsOpened: out.FixPRs.Opened,
	})
	out.Score, out.Status, out.NoActivity = sr.Score, sr.Status, sr.NoActivity
```

- [ ] **Step 3: 编译 + vet**

Run: `wsl -d Ubuntu -- bash -lc "cd /mnt/d/shulex_work/forge/server && go build ./... 2>&1 | tail -8 && go vet ./internal/handler/ ./internal/forgehealth/ 2>&1 | tail -5 && echo OK"`
Expected: 打印 `OK`。

- [ ] **Step 4: Commit**

```bash
git add server/internal/handler/forge_health.go
git commit -m "feat(forge): expose Harness health score on GetForgeHealth"
```

---

## Phase 0 完成检查
- [ ] `Score` 6 个表驱动单测绿
- [ ] `GetForgeHealth` 响应含 `score`/`status`/`no_activity`,build + vet 绿
