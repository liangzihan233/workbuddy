package auth

import (
	"encoding/base64"
	"testing"
)

// mustJWT 用给定 payload 拼一个最小 JWT（header.payload.signature）。
// AccessToken 内容用于证明 token_source 不再触发门控，签名不参与判断。
func mustJWT(t *testing.T, payload string) string {
	t.Helper()
	return "hdr." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
}

// TestIsEnterpriseSessionIgnoresTokenSource 固化 2026-10-26 的上游行为：
// 个人版账号也会拿到 token_source=enterprise_switch，该声明不能再触发门控；
// 真正的企业版仍以非空 enterpriseId 为准。
func TestIsEnterpriseSessionIgnoresTokenSource(t *testing.T) {
	a := &Auth{
		UID:         "u1",
		AccessToken: mustJWT(t, `{"sub":"u1","enterprise_id":"","token_source":"enterprise_switch"}`),
	}
	if a.IsEnterpriseSession() {
		t.Fatal("token_source=enterprise_switch 不应再把个人版账号判为企业上下文")
	}
	if a.IsEnterprise() {
		t.Fatal("仅 token_source=enterprise_switch 不应翻转 IsEnterprise")
	}

	a.EnterpriseID = "e1"
	if !a.IsEnterpriseSession() {
		t.Fatal("enterpriseId 非空时 IsEnterpriseSession 必须为 true")
	}
}
