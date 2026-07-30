// Package clock 定义可替换时钟能力。
package clock

import "time"

type Clock interface{ Now() time.Time }
