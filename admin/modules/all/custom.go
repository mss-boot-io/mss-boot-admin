package all

import (
	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/modules/litellmops"
)

// CustomModules returns hand-written modules that the generator cannot
// express. Keep generated Modules() untouched; append these at the
// composition root.
func CustomModules() []business.Module {
	return []business.Module{
		litellmops.Module(),
	}
}
