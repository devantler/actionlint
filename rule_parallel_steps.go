package actionlint

import "strings"

// RuleParallelSteps is a rule to check references between parallel steps. A 'wait' or 'cancel' step
// must refer to the ID of a preceding step that runs in the background (with 'background: true').
// https://github.blog/changelog/2026-06-25-actions-steps-can-now-be-run-in-parallel/
type RuleParallelSteps struct {
	RuleBase
	// background holds the IDs (lower-cased; step IDs are case insensitive) of the 'background: true'
	// steps seen so far in the current job. Steps are visited in order, including steps nested in
	// 'parallel:' groups, so a 'wait'/'cancel' reference found in this set both exists and precedes
	// the reference.
	background map[string]struct{}
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
	rule.background = map[string]struct{}{}
	return nil
}

// VisitJobPost is callback when visiting Job node after visiting its children.
func (rule *RuleParallelSteps) VisitJobPost(n *Job) error {
	rule.background = nil
	return nil
}

// VisitStep is callback when visiting Step node.
func (rule *RuleParallelSteps) VisitStep(n *Step) error {
	switch e := n.Exec.(type) {
	case *ExecWait:
		for _, name := range e.Names {
			rule.checkRef(name)
		}
	case *ExecCancel:
		rule.checkRef(e.Name)
	}

	if n.ID != nil && !n.ID.ContainsExpression() && isBackgroundStep(n) {
		rule.background[strings.ToLower(n.ID.Value)] = struct{}{}
	}

	return nil
}

func (rule *RuleParallelSteps) checkRef(ref *String) {
	if ref == nil || ref.Value == "" || ref.ContainsExpression() {
		// Empty values come from an already-reported parse error; expression step IDs can't be
		// resolved statically. Skip both.
		return
	}
	if _, ok := rule.background[strings.ToLower(ref.Value)]; !ok {
		rule.Errorf(
			ref.Pos,
			"%q is not the ID of a preceding background step. \"wait\" and \"cancel\" steps can only refer to an earlier step that has \"background: true\"",
			ref.Value,
		)
	}
}

// isBackgroundStep reports whether the step runs (or may run) in the background. When 'background' is
// an expression, whether the step runs in the background can't be known statically, so it's treated
// as a possible background step to avoid false positives.
func isBackgroundStep(s *Step) bool {
	return s.Background != nil && (s.Background.Expression != nil || s.Background.Value)
}
