// Package config_test 从 Kernel 契约视角验证 Snapshot 的不可变值语义。
package config_test

import (
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/rin721/micro-go/internal/kernel/config"
)

// value 包含可变 map，用于暴露 Snapshot 浅复制问题。
type value struct {
	// Items 是测试会主动修改的嵌套引用值。
	Items map[string]string `json:"items"`
}

// TestSnapshotValueReturnsDeepCopy 修改第一次读取结果后再次读取，证明 Snapshot 内部数据
// 不会被切片、map 或指针别名反向污染。
func TestSnapshotValueReturnsDeepCopy(t *testing.T) {
	// 创建原始值并编码为 Snapshot 接收的规范 JSON。
	original := value{Items: map[string]string{"a": "b"}}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	typeOf := reflect.TypeOf(original)
	// 摘要和 Data 来自同一字节，模拟 Loader 正常输出。
	snapshot := config.NewSnapshot(1, time.Now(), []config.SnapshotEntry{{Type: typeOf, Data: data, Hash: sha256.Sum256(data)}})
	first, err := config.Value[value](snapshot)
	if err != nil {
		t.Fatal(err)
	}
	// 修改第一次解码的新 map，不应影响 Snapshot 保存的 RawMessage。
	first.Items["a"] = "changed"
	second, err := config.Value[value](snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if second.Items["a"] != "b" {
		t.Fatalf("snapshot mutated: %#v", second)
	}
}
