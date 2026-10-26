package scheduler

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/linguo2625469/workbuddy2api-panel/internal/auth"
	"github.com/linguo2625469/workbuddy2api-panel/internal/pool"
	"github.com/linguo2625469/workbuddy2api-panel/internal/upstream"
)

// sessionTestScheduler 构造两种个人版凭证：switched=true 时 AccessToken 带
// token_source=enterprise_switch，enterpriseId 仍为空。2026-10-26 起该声明
// 不能再阻止个人成长任务。
func sessionTestScheduler(t *testing.T, switched bool) (*Scheduler, *recorderUpstream) {
	t.Helper()
	r := &recorderUpstream{}
	srv := r.server()
	t.Cleanup(srv.Close)

	token := "at"
	if switched {
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"u1","enterprise_id":"","token_source":"enterprise_switch"}`))
		token = "hdr." + payload + ".sig"
	}
	a := &auth.Auth{UID: "u1", AccessToken: token, RefreshToken: "rt", ExpiresAt: 9999999999}
	p := pool.New("")
	p.Add(a)

	up := &upstream.Client{
		HTTP:          srv.Client(),
		ChatBaseCN:    srv.URL,
		BillingBaseCN: srv.URL,
		WebBaseCN:     srv.URL,
	}
	s := New(Config{Pool: p, Upstream: up})
	return s, r
}

// TestEnterpriseSwitchTokenDoesNotGateBehaviorTasks 固化实测修复：
// 个人版全新 OAuth 也会带 token_source=enterprise_switch，行为上报仍正常计分，
// 因此活跃 / 旅行 / 夜猫子 / 连登都不能因该声明跳过。
func TestEnterpriseSwitchTokenDoesNotGateBehaviorTasks(t *testing.T) {
	restoreClock := nowForNightWindow
	nowForNightWindow = func() time.Time { return time.Date(2026, 10, 26, 23, 30, 0, 0, time.Local) }
	t.Cleanup(func() { nowForNightWindow = restoreClock })

	cases := []struct {
		name     string
		run      func(*Scheduler)
		endpoint string
	}{
		{"签到", func(s *Scheduler) { s.RunCheckinNow() }, "/daily-checkin"},
		{"活跃上报", func(s *Scheduler) { s.RunActivityNow() }, "/v2/report"},
		{"猫猫旅行", func(s *Scheduler) { s.RunTravelNow() }, "/activity/growth/buddy/info"},
		{"夜猫子", func(s *Scheduler) { s.RunBlackcatNow() }, "/v2/activity/growth/tasks"},
		{"连登管家", func(s *Scheduler) { s.RunStreakBonusNow() }, "/activity/growth/streak"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, r := sessionTestScheduler(t, true)
			c.run(s)
			if !r.has(c.endpoint) {
				t.Errorf("token_source=enterprise_switch 的个人版应照常发起 %s，实际请求=%v", c.endpoint, r.snapshot())
			}
		})
	}
}

// TestEnterpriseSwitchTokenKeepsKeepalive 保活与成长空间判定无关。
func TestEnterpriseSwitchTokenKeepsKeepalive(t *testing.T) {
	s, r := sessionTestScheduler(t, true)
	s.RunKeepaliveNow()
	if !r.has("/token/refresh") {
		t.Errorf("个人版凭证必须照常保活，实际请求=%v", r.snapshot())
	}
}
