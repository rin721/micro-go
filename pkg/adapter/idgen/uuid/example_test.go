// 本文件以外部调用方视角演示 UUID Adapter 只返回项目约定的字符串 ID。
package uuid_test

import (
	"fmt"
	"strings"

	uuidadapter "github.com/rin721/micro-go/pkg/adapter/idgen/uuid"
	"github.com/rin721/micro-go/types/capability/idgen"
)

// Example 验证通过 Generator 契约得到标准 UUID 文本形状。
func Example() {
	// 接口变量证明业务方无需接触 google/uuid 类型。
	var ids idgen.Generator = uuidadapter.New()
	// 每次 New 生成一条新的字符串标识。
	id := ids.New()

	// 只断言稳定格式特征，避免把随机 UUID 内容写入 Example 输出。
	fmt.Println(len(id) == 36, strings.Count(id, "-") == 4)
	// Output: true true
}
