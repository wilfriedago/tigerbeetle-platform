// Deliberate violation for `just lint-depguard-probe`: the depguard rule isolate-access must report it.
// probe: isolate-access
package v1

import _ "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/access/application"
