package v1

import (
	"context"

	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/kernel/ledger"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/application"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/modules/ledgers/interfaces/dto"
	"github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi"
	pb "github.com/wilfriedago/tigerbeetle-platform/apps/api/internal/platform/grpcapi/platformv1"
)

// SettingsServer implements SettingsService.
type SettingsServer struct {
	commands application.SettingsCommands
	queries  application.SettingsQueries
}

// NewSettingsServer builds SettingsService.
func NewSettingsServer(app application.Application) *SettingsServer {
	return &SettingsServer{commands: app.SettingsCommands, queries: app.SettingsQueries}
}

// ListRules lists the rules, most specific first, built-in defaults included.
func (s *SettingsServer) ListRules(ctx context.Context, req *pb.ListRulesRequest) (*pb.ListRulesResponse, error) {
	rules, err := s.queries.Rules(ctx, req.GetLedger())
	if err != nil {
		return nil, err
	}
	out := &pb.ListRulesResponse{Rules: make([]*pb.Rule, len(rules))}
	for i := range rules {
		out.Rules[i] = ruleOf(dto.RuleFrom(rules[i]))
	}
	return out, nil
}

// MatchRule reports which rule wins for an address.
func (s *SettingsServer) MatchRule(ctx context.Context, req *pb.MatchRuleRequest) (*pb.MatchRuleResponse, error) {
	m, err := s.queries.MatchRule(ctx, req.GetLedger(), req.GetAddress())
	if err != nil {
		return nil, err
	}
	match := dto.RuleMatchFrom(m)
	out := &pb.MatchRuleResponse{
		Address:    string(match.Address),
		Rule:       ruleOf(match.Rule),
		Candidates: make([]*pb.RuleCandidate, len(match.Candidates)),
		Existing:   make([]string, len(match.Existing)),
	}
	for i, c := range match.Candidates {
		out.Candidates[i] = &pb.RuleCandidate{Rule: ruleOf(c.Rule), Matches: c.Matches}
	}
	for i, a := range match.Existing {
		out.Existing[i] = string(a)
	}
	return out, nil
}

// PutRule creates or replaces the rule for a pattern.
func (s *SettingsServer) PutRule(ctx context.Context, req *pb.PutRuleRequest) (*pb.PutRuleResponse, error) {
	if req.GetCode() < 1 || req.GetCode() > 65535 {
		return nil, ledger.NewProblem(ledger.ErrInvalid, "invalid-request", "code must be between 1 and 65535; TigerBeetle rejects code 0")
	}
	rule, err := s.commands.PutRule(ctx, req.GetLedger(), ledger.Rule{
		Pattern: ledger.Pattern(req.GetPattern()),
		Limit:   ledger.Limit(grpcapi.ValueOf(req.GetLimit())),
		History: req.GetHistory(),
		Code:    uint16(req.GetCode()),
	})
	if err != nil {
		return nil, err
	}
	return &pb.PutRuleResponse{Rule: ruleOf(dto.RuleFrom(rule))}, nil
}

// DeleteRule deletes a stored rule.
func (s *SettingsServer) DeleteRule(ctx context.Context, req *pb.DeleteRuleRequest) (*pb.DeleteRuleResponse, error) {
	if err := s.commands.DeleteRule(ctx, req.GetLedger(), req.GetPattern()); err != nil {
		return nil, err
	}
	return &pb.DeleteRuleResponse{}, nil
}

// ListEventTypes lists the event types, the built-in default first.
func (s *SettingsServer) ListEventTypes(ctx context.Context, req *pb.ListEventTypesRequest) (*pb.ListEventTypesResponse, error) {
	views, err := s.queries.EventTypes(ctx, req.GetLedger())
	if err != nil {
		return nil, err
	}
	out := &pb.ListEventTypesResponse{EventTypes: make([]*pb.EventType, len(views))}
	for i := range views {
		out.EventTypes[i] = eventTypeOf(dto.EventTypeFrom(views[i]))
	}
	return out, nil
}

// CreateEventType registers an event type.
func (s *SettingsServer) CreateEventType(ctx context.Context, req *pb.CreateEventTypeRequest) (*pb.CreateEventTypeResponse, error) {
	e, err := s.commands.CreateEventType(ctx, req.GetLedger(), req.GetName(), int(req.GetCode()))
	if err != nil {
		return nil, err
	}
	return &pb.CreateEventTypeResponse{EventType: eventTypeOf(dto.EventTypeFrom(application.EventTypeView{EventType: e}))}, nil
}

func ruleOf(r dto.Rule) *pb.Rule {
	return &pb.Rule{
		Pattern:   r.Pattern,
		Limit:     grpcapi.EnumOf[pb.BalanceLimit](string(r.Limit)),
		History:   r.History,
		Code:      uint32(r.Code),
		BuiltIn:   r.BuiltIn,
		Segments:  int32(r.Segments),
		CreatedAt: grpcapi.OptionalTimestamp(r.CreatedAt),
		UpdatedAt: grpcapi.OptionalTimestamp(r.UpdatedAt),
	}
}

func eventTypeOf(e dto.EventType) *pb.EventType {
	return &pb.EventType{
		Name:         e.Name,
		Code:         uint32(e.Code),
		BuiltIn:      e.BuiltIn,
		CreatedAt:    grpcapi.OptionalTimestamp(e.CreatedAt),
		Transactions: e.Transactions,
	}
}
