// 本文件验证全局门禁配置的有效性、隔离边界和快照不可变性。
package gateconfig

import "testing"

// TestCurrentPolicyIsValid 保证仓库提交的唯一策略能够被所有门禁安全消费。
func TestCurrentPolicyIsValid(t *testing.T) {
	if err := Validate(Current()); err != nil {
		t.Fatal(err)
	}
}

// TestCurrentReturnsDeepCopy 保证调用方修改快照不会污染后续门禁执行。
func TestCurrentReturnsDeepCopy(t *testing.T) {
	first := Current()
	first.Source.ScanRoots[0] = "changed"
	first.Quarantines[0].OriginalRoots[0] = "changed"
	second := Current()
	if second.Source.ScanRoots[0] == "changed" || second.Quarantines[0].OriginalRoots[0] == "changed" {
		t.Fatal("Current() shares mutable slices with the global policy")
	}
}

// TestPolicyExcludesOnlyDirectoryBoundaries 防止相似前缀目录被隔离规则意外放过门禁。
func TestPolicyExcludesOnlyDirectoryBoundaries(t *testing.T) {
	policy := Current()
	cases := []struct {
		// path 是待判断的仓库相对路径。
		path string
		// want 表示该路径是否应被门禁忽略。
		want bool
	}{
		{path: "_quarantine/password", want: true},
		{path: "_quarantine/password/pkg/adapter/password/crypto.go", want: true},
		{path: "_quarantine/password-other/example.go", want: false},
		{path: "pkg/adapter/password/example.go", want: false},
	}
	for _, test := range cases {
		if got := policy.Excludes(test.path); got != test.want {
			t.Errorf("Excludes(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}

// TestValidateRejectsInvalidPolicy 锁定路径、篇幅和隔离治理元数据的最低要求。
func TestValidateRejectsInvalidPolicy(t *testing.T) {
	cases := []struct {
		// name 是失败场景名称。
		name string
		// change 制造单一非法策略事实。
		change func(*Policy)
	}{
		{name: "absolute path", change: func(policy *Policy) { policy.Source.TypesRoot = "/types" }},
		{name: "backslash path", change: func(policy *Policy) { policy.Source.TypesRoot = `types\contracts` }},
		{name: "duplicate path", change: func(policy *Policy) { policy.Source.ScanRoots = []string{"types", "types"} }},
		{name: "line limit", change: func(policy *Policy) { policy.Documentation.MaxTopicLines = 0 }},
		{name: "quarantine metadata", change: func(policy *Policy) { policy.Quarantines[0].RestoreWhen = "" }},
		{name: "quarantine go discovery", change: func(policy *Policy) { policy.Quarantines[0].Root = "quarantine/password" }},
		{name: "duplicate heading", change: func(policy *Policy) { policy.Documentation.PackageREADMEHeadings = []string{"## 职责", "## 职责"} }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			policy := Current()
			test.change(&policy)
			if err := Validate(policy); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}
