package main

// jsonReport is the machine-readable rendering of a deltaResult, behind the
// -json flag: a future wave (see this command's package doc) can drive a
// scaffolding pass off these fields directly rather than transcribing the
// human report by hand. Field names are snake_case because this is the one
// artefact of this tool meant to be consumed by another program, not read on
// a terminal.
type jsonReport struct {
	Before  string           `json:"before"`
	After   string           `json:"after"`
	Counts  jsonCounts       `json:"counts"`
	Domains []jsonDomainWork `json:"domains"`
}

type jsonCounts struct {
	BeforeOperations   int `json:"before_operations"`
	AfterOperations    int `json:"after_operations"`
	Added              int `json:"added"`
	Removed            int `json:"removed"`
	Changed            int `json:"changed"`
	ChangedTouchStruct int `json:"changed_touching_struct"`
	Unresolvable       int `json:"unresolvable"`
}

type jsonOpRef struct {
	OperationID string `json:"operation_id"`
	Method      string `json:"method"`
	Path        string `json:"path"`
}

type jsonFieldChange struct {
	Field  string `json:"field"`
	Kind   string `json:"kind"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type jsonChangedOp struct {
	OperationID   string            `json:"operation_id"`
	Method        string            `json:"method"`
	Path          string            `json:"path"`
	TouchesStruct bool              `json:"touches_struct"`
	Changes       []jsonFieldChange `json:"changes"`
}

type jsonUnresolvedOp struct {
	OperationID string `json:"operation_id"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Reason      string `json:"reason"`
}

// jsonDomainWork is one domain's work list, in the same five buckets
// buildReport prints, so a program consuming this JSON can walk it in the
// identical judgement-first order a human reading the report does.
type jsonDomainWork struct {
	Domain          string             `json:"domain"`
	Added           []jsonOpRef        `json:"added,omitempty"`
	Removed         []jsonOpRef        `json:"removed,omitempty"`
	Unresolvable    []jsonUnresolvedOp `json:"unresolvable,omitempty"`
	ChangedStruct   []jsonChangedOp    `json:"changed_struct,omitempty"`
	ChangedCosmetic []jsonChangedOp    `json:"changed_cosmetic,omitempty"`
}

// toJSONReport converts result into its JSON rendering. A pure conversion,
// deliberately: computeDelta decides what belongs in the work list and in
// which order, this function only relabels it for encoding/json, so the
// human report and the machine report can never disagree about which
// operations are in scope or how they are classified.
func toJSONReport(before, after string, result *deltaResult) jsonReport {
	report := jsonReport{
		Before: before,
		After:  after,
		Counts: jsonCounts{
			BeforeOperations:   result.BeforeCount,
			AfterOperations:    result.AfterCount,
			Added:              result.AddedCount,
			Removed:            result.RemovedCount,
			Changed:            result.ChangedCount,
			ChangedTouchStruct: result.ChangedStructCount,
			Unresolvable:       result.UnresolvableCount,
		},
	}

	for _, g := range result.Domains {
		dw := jsonDomainWork{Domain: g.Domain}
		for _, ref := range g.Added {
			dw.Added = append(dw.Added, jsonOpRef(ref))
		}
		for _, ref := range g.Removed {
			dw.Removed = append(dw.Removed, jsonOpRef(ref))
		}
		for _, op := range g.Unresolvable {
			dw.Unresolvable = append(dw.Unresolvable, jsonUnresolvedOp(op))
		}
		for _, op := range g.ChangedStruct {
			dw.ChangedStruct = append(dw.ChangedStruct, toJSONChangedOp(op))
		}
		for _, op := range g.ChangedCosmetic {
			dw.ChangedCosmetic = append(dw.ChangedCosmetic, toJSONChangedOp(op))
		}
		report.Domains = append(report.Domains, dw)
	}

	return report
}

func toJSONChangedOp(op changedOp) jsonChangedOp {
	jo := jsonChangedOp{
		OperationID:   op.OperationID,
		Method:        op.Method,
		Path:          op.Path,
		TouchesStruct: op.TouchesStruct,
	}
	for _, c := range op.Changes {
		jo.Changes = append(jo.Changes, jsonFieldChange{
			Field:  c.JSONName,
			Kind:   string(c.Kind),
			Before: c.Before,
			After:  c.After,
		})
	}
	return jo
}
