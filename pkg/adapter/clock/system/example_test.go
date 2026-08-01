package system_test

import (
	"fmt"

	"github.com/rin721/micro-go/pkg/adapter/clock/system"
	"github.com/rin721/micro-go/types/capability/clock"
)

func Example() {
	var appClock clock.Clock = system.New()

	fmt.Println(appClock.Now().IsZero())
	// Output: false
}
