package dto

import (
	"fmt"
	"net/http"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/bookkeeping/domain"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/problem"
)

// notRaw are the results no problem is named after: a success, the link failure every other
// posting of a declined chain reports, and a burned ID, which becomes transaction-declined or
// hold-declined.
var notRaw = map[string]bool{
	domain.ResultExists:            true,
	domain.ResultLinkedEventFailed: true,
	domain.ResultIDAlreadyFailed:   true,
}

// ProblemTypes documents the problem types a declined chain reports under the name of its
// TigerBeetle result, such as pending-transfer-not-found, for the catalogue in docs/problems.md.
// The results the platform names itself, such as exceeds-credits, are left to package problem.
func ProblemTypes() []problem.Definition {
	named := map[string]bool{}
	for _, d := range problem.Definitions() {
		named[d.Reason] = true
	}
	out := make([]problem.Definition, 0, len(domain.TransferResults))
	for _, result := range domain.TransferResults {
		reason := domain.ResultReason(result)
		if notRaw[result] || named[reason] {
			continue
		}
		out = append(out, problem.Definition{
			Reason: reason,
			Title:  problem.TitleFor(reason),
			Status: http.StatusUnprocessableEntity,
			Description: fmt.Sprintf("TigerBeetle declined a posting with the result `%s`. Nothing was applied: `posting` is "+
				"the index of the posting that failed and `results` holds the result of every posting. "+
				"TigerBeetle documents the result at https://docs.tigerbeetle.com/reference/requests/create_transfers/#%s.",
				result, result),
		})
	}
	return out
}
