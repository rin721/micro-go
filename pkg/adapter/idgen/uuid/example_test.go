package uuid_test

import (
	"fmt"
	"strings"

	uuidadapter "github.com/rin721/micro-go/pkg/adapter/idgen/uuid"
	"github.com/rin721/micro-go/types/capability/idgen"
)

func Example() {
	var ids idgen.Generator = uuidadapter.New()
	id := ids.New()

	fmt.Println(len(id) == 36, strings.Count(id, "-") == 4)
	// Output: true true
}
