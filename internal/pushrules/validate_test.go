package pushrules

import (
	"strings"
	"testing"
)

func TestValidateRuleNegatives(t *testing.T) {
	tsts := []struct {
		Name          string
		Kind          Kind
		Rule          Rule
		WantErrString string
	}{
		{Name: "emptyRuleID", Kind: OverrideKind, Rule: Rule{}, WantErrString: "invalid rule ID"},
		{Name: "invalidKind", Kind: Kind("something else"), Rule: Rule{}, WantErrString: "invalid rule kind"},
		{Name: "ruleIDBackslash", Kind: OverrideKind, Rule: Rule{RuleID: "#foo\\:example.com"}, WantErrString: "invalid rule ID"},
		{Name: "noActions", Kind: OverrideKind, Rule: Rule{}, WantErrString: "missing actions"},
		{Name: "invalidAction", Kind: OverrideKind, Rule: Rule{Actions: []*Action{{}}}, WantErrString: "invalid rule action kind"},
		{Name: "invalidCondition", Kind: OverrideKind, Rule: Rule{Conditions: []*Condition{{}}}, WantErrString: "invalid rule condition kind"},
		{Name: "overrideNoCondition", Kind: OverrideKind, Rule: Rule{}, WantErrString: "missing rule conditions"},
		{Name: "underrideNoCondition", Kind: UnderrideKind, Rule: Rule{}, WantErrString: "missing rule conditions"},
		{Name: "contentNoPattern", Kind: ContentKind, Rule: Rule{}, WantErrString: "missing content rule pattern"},
		{Name: "contentEmptyPattern", Kind: ContentKind, Rule: Rule{Pattern: pointer("")}, WantErrString: "missing content rule pattern"},
	}
	for _, tst := range tsts {
		t.Run(tst.Name, func(t *testing.T) {
			errs := ValidateRule(tst.Kind, &tst.Rule)
			var foundErr error
			for _, err := range errs {
				t.Logf("Got error %#v", err)
				if strings.Contains(err.Error(), tst.WantErrString) {
					foundErr = err
				}
			}
			if foundErr == nil {
				t.Errorf("errs: got %#v, want containing %q", errs, tst.WantErrString)
			}
		})
	}
}

func TestValidateRulePositives(t *testing.T) {
	tsts := []struct {
		Name            string
		Kind            Kind
		Rule            Rule
		WantNoErrString string
	}{
		{"invalidKind", OverrideKind, Rule{}, "invalid rule kind"},
		{"invalidActionNoActions", OverrideKind, Rule{}, "invalid rule action kind"},
		{"invalidConditionNoConditions", OverrideKind, Rule{}, "invalid rule condition kind"},
		{"contentNoCondition", ContentKind, Rule{}, "missing rule conditions"},
		{"roomNoCondition", RoomKind, Rule{}, "missing rule conditions"},
		{"senderNoCondition", SenderKind, Rule{}, "missing rule conditions"},
		{"overrideNoPattern", OverrideKind, Rule{}, "missing content rule pattern"},
		{"overrideEmptyConditions", OverrideKind, Rule{Conditions: []*Condition{}}, "missing rule conditions"},
	}
	for _, tst := range tsts {
		t.Run(tst.Name, func(t *testing.T) {
			errs := ValidateRule(tst.Kind, &tst.Rule)
			for _, err := range errs {
				t.Logf("Got error %#v", err)
				if strings.Contains(err.Error(), tst.WantNoErrString) {
					t.Errorf("errs: got %#v, want none containing %q", errs, tst.WantNoErrString)
				}
			}
		})
	}
}

func TestValidateActionNegatives(t *testing.T) {
	tsts := []struct {
		Name          string
		Action        Action
		WantErrString string
	}{
		{"emptyKind", Action{}, "invalid rule action kind"},
		{"invalidKind", Action{Kind: ActionKind("something else")}, "invalid rule action kind"},
	}
	for _, tst := range tsts {
		t.Run(tst.Name, func(t *testing.T) {
			errs := validateAction(&tst.Action)
			var foundErr error
			for _, err := range errs {
				t.Logf("Got error %#v", err)
				if strings.Contains(err.Error(), tst.WantErrString) {
					foundErr = err
				}
			}
			if foundErr == nil {
				t.Errorf("errs: got %#v, want containing %q", errs, tst.WantErrString)
			}
		})
	}
}

func TestValidateActionPositives(t *testing.T) {
	tsts := []struct {
		Name            string
		Action          Action
		WantNoErrString string
	}{
		{"invalidKind", Action{Kind: NotifyAction}, "invalid rule action kind"},
	}
	for _, tst := range tsts {
		t.Run(tst.Name, func(t *testing.T) {
			errs := validateAction(&tst.Action)
			for _, err := range errs {
				t.Logf("Got error %#v", err)
				if strings.Contains(err.Error(), tst.WantNoErrString) {
					t.Errorf("errs: got %#v, want none containing %q", errs, tst.WantNoErrString)
				}
			}
		})
	}
}

func TestValidateConditionNegatives(t *testing.T) {
	tsts := []struct {
		Name          string
		Condition     Condition
		WantErrString string
	}{
		{"emptyKind", Condition{}, "invalid rule condition kind"},
		{"invalidKind", Condition{Kind: ConditionKind("something else")}, "invalid rule condition kind"},
	}
	for _, tst := range tsts {
		t.Run(tst.Name, func(t *testing.T) {
			errs := validateCondition(&tst.Condition)
			var foundErr error
			for _, err := range errs {
				t.Logf("Got error %#v", err)
				if strings.Contains(err.Error(), tst.WantErrString) {
					foundErr = err
				}
			}
			if foundErr == nil {
				t.Errorf("errs: got %#v, want containing %q", errs, tst.WantErrString)
			}
		})
	}
}

func TestValidateConditionPositives(t *testing.T) {
	tsts := []struct {
		Name            string
		Condition       Condition
		WantNoErrString string
	}{
		{"invalidKind", Condition{Kind: EventMatchCondition}, "invalid rule condition kind"},
	}
	for _, tst := range tsts {
		t.Run(tst.Name, func(t *testing.T) {
			errs := validateCondition(&tst.Condition)
			for _, err := range errs {
				t.Logf("Got error %#v", err)
				if strings.Contains(err.Error(), tst.WantNoErrString) {
					t.Errorf("errs: got %#v, want none containing %q", errs, tst.WantNoErrString)
				}
			}
		})
	}
}
