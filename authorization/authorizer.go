package authorization

import (
	"context"
	"embed"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/open-policy-agent/opa/v1/rego"
)

//go:embed policies/*
var policies embed.FS

type Authorizer struct {
	query rego.PreparedEvalQuery
}

type Caller struct {
	UserID string
	Role   string
}

type Target struct {
	UserID             string
	ConsultationStatus string
	ConsultationType   string
	ScheduledAtPast    bool
	WithinJoinWindow   bool
	// OpinionDeadlinePassed is true once the 24 hours after a video call for the
	// doctor's written opinion are over.
	OpinionDeadlinePassed bool
}

func (a Authorizer) IsAllowed(ctx context.Context, operationID string, caller Caller, target Target) (bool, error) {
	results, err := a.query.Eval(ctx, rego.EvalInput(map[string]interface{}{
		"operation_id": operationID,
		"caller":       caller,
		"target":       target,
	}))
	if err != nil {
		return false, err
	}

	return results.Allowed(), nil
}

func NewAuthorizer(ctx context.Context, db *pgxpool.Pool) (Authorizer, error) {
	modules := [](func(*rego.Rego)){}

	err := fs.WalkDir(policies, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		policy, err := policies.ReadFile(path)
		if err != nil {
			return err
		}

		modules = append(modules, rego.Module(d.Name(), string(policy)))

		return nil
	})
	if err != nil {
		return Authorizer{}, err
	}

	options := append([](func(*rego.Rego)){rego.Query("data.policies.allow")}, modules...)
	query, err := rego.New(options...).PrepareForEval(ctx)
	if err != nil {
		return Authorizer{}, err
	}

	return Authorizer{query: query}, nil
}
