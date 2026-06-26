package actionlint

import "strings"

// RuleParallelSteps is a rule to check references between parallel steps. A 'wait' or 'cancel' step
// must refer to the ID of a preceding step that runs in the background (with 'background: true').
// https://github.blog/changelog/2026-06-25-actions-steps-can-now-be-run-in-parallel/
type RuleParallelSteps struct {
	RuleBase
}

// NewRuleParallelSteps creates a new RuleParallelSteps instance.
func NewRuleParallelSteps() *RuleParallelSteps {
	return &RuleParallelSteps{
		RuleBase: RuleBase{
			name: "parallel-steps",
			desc: "Checks that \"wait\" and \"cancel\" steps refer to IDs of preceding \"background\" steps",
		},
	}
}

// VisitJobPre is callback when visiting Job node before visiting its children.
func (rule *RuleParallelSteps) VisitJobPre(n *Job) error {
	// Collect the IDs of 'background: true' steps while walking the steps in order. A 'wait' or
	// 'cancel' step may only refer to a background step that precedes it, so checking each reference
	// against the IDs seen so far validates existence and ordering at once. Step IDs are case
	// insensitive, hence lower-cased.
	background := map[string]struct{}{}
	rule.checkSteps(n.Steps, background)
	return nil
}

func (rule *RuleParallelSteps) checkSteps(steps []*Step, background map[string]struct{}) {
	for _, s := range steps {
		switch e := s.Exec.(type) {
		case *ExecWait:
			for _, name := range e.Names {
				rule.checkRef(name, background)
			}
		case *ExecCancel:
			rule.checkRef(e.Name, background)
		case *ExecParallel:
			// Steps grouped in 'parallel:' run after the steps before the group, so they may refer to
			// background steps seen so far.
			rule.checkSteps(e.Steps, background)
		}

		if id := backgroundStepID(s); id != "" {
			background[id] = struct{}{}
		}
	}
}

func (rule *RuleParallelSteps) checkRef(ref *String, background map[string]struct{}) {
	if ref == nil || ref.Value == "" || ref.ContainsExpression() {
		// Empty values come from an already-reported parse error; expression step IDs can't be
		// resolved statically. Skip both.
		return
	}
	if _, ok := background[strings.ToLower(ref.Value)]; !ok {
		rule.Errorf(
			ref.Pos,
			"%q is not the ID of a preceding background step. \"wait\" and \"cancel\" steps can only refer to an earlier step that has \"background: true\"",
			ref.Value,
		)
	}
}

// backgroundStepID returns the lower-cased ID of the given step if it is (or may be) a background
// step that can be referred to by a later 'wait' or 'cancel' step. It returns an empty string when
// the step has no static ID or is statically not a background step.
func backgroundStepID(s *Step) string {
	if s.ID == nil || s.ID.ContainsExpression() {
		return ""
	}
	// When 'background' is an expression, whether the step runs in the background can't be known
	// statically, so it's treated as a possible background step to avoid false positives.
	if s.Background == nil || (s.Background.Expression == nil && !s.Background.Value) {
		return ""
	}
	return strings.ToLower(s.ID.Value)
}
