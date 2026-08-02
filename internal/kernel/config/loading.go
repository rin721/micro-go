// 本文件定义 Loader 完成一次配置加载后交给 Runtime 的成对结果。
package config

import "reflect"

// Loaded 把不可变 Snapshot 与同一次解码产生的构造注入值放在一起。
// Values 只在 Build 期间交给构造引擎，运行期组件只能通过 Snapshot 参与 Reload。
type Loaded struct {
	// Snapshot 是已验证并可发布的配置事实。
	Snapshot Snapshot
	// Values 是与 Snapshot 完全一致的强类型反射值。
	Values map[reflect.Type]reflect.Value
}
