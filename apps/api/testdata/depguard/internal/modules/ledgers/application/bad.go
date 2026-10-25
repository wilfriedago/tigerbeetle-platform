// Deliberate violation for `just lint-depguard-probe`: the depguard rule module-application must report it.
// probe: module-application
package application

import _ "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/infrastructure/postgres"
