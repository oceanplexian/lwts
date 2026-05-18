package repo

import "testing"

func TestCustomFieldDefinitionsNormalizeValidConfig(t *testing.T) {
	defs, err := ValidateCustomFieldDefinitions([]CustomFieldDefinition{
		{ID: "customer", Name: "Customer", Type: "text", Required: true},
		{ID: "severity", Name: "Severity", Type: "choice", Options: []CustomFieldOption{
			{Label: "SEV 1", Color: "#f44336"},
			{ID: "sev2", Label: "SEV 2"},
		}},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if defs[1].Type != CustomFieldTypeSelect {
		t.Fatalf("type normalized to %q, want select", defs[1].Type)
	}
	if defs[1].Options[0].ID != "sev-1" {
		t.Fatalf("option id = %q, want sev-1", defs[1].Options[0].ID)
	}
}

func TestCustomFieldDefinitionsRejectInvalidConfig(t *testing.T) {
	cases := []struct {
		name string
		defs []CustomFieldDefinition
	}{
		{
			name: "reserved id",
			defs: []CustomFieldDefinition{{ID: "priority", Name: "Priority 2", Type: "text"}},
		},
		{
			name: "duplicate id",
			defs: []CustomFieldDefinition{
				{ID: "customer", Name: "Customer", Type: "text"},
				{ID: "customer", Name: "Client", Type: "text"},
			},
		},
		{
			name: "select without options",
			defs: []CustomFieldDefinition{{ID: "severity", Name: "Severity", Type: "select"}},
		},
		{
			name: "duplicate option",
			defs: []CustomFieldDefinition{{ID: "severity", Name: "Severity", Type: "select", Options: []CustomFieldOption{
				{ID: "sev1", Label: "SEV 1"},
				{ID: "sev1", Label: "SEV One"},
			}}},
		},
		{
			name: "invalid color",
			defs: []CustomFieldDefinition{{ID: "severity", Name: "Severity", Type: "select", Options: []CustomFieldOption{
				{ID: "sev1", Label: "SEV 1", Color: "red"},
			}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateCustomFieldDefinitions(tc.defs); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCustomFieldValuesEnforceRequiredAndChoices(t *testing.T) {
	defs := []CustomFieldDefinition{
		{ID: "customer", Name: "Customer", Type: CustomFieldTypeText, Required: true},
		{ID: "severity", Name: "Severity", Type: CustomFieldTypeSelect, Options: []CustomFieldOption{
			{ID: "sev1", Label: "SEV 1"},
			{ID: "sev2", Label: "SEV 2"},
		}},
	}

	_, err := ApplyCustomFieldPatch(defs, nil, map[string]any{"severity": "sev1"}, true)
	if err == nil {
		t.Fatal("expected required customer error")
	}
	fields, ok := IsFieldValidationError(err)
	if !ok || fields["custom_fields.customer"] != "required" {
		t.Fatalf("fields = %#v", fields)
	}

	_, err = ApplyCustomFieldPatch(defs, nil, map[string]any{"customer": "Acme", "severity": "sev3"}, true)
	if err == nil {
		t.Fatal("expected invalid select option error")
	}

	values, err := ApplyCustomFieldPatch(defs, nil, map[string]any{"customer": "Acme", "severity": "sev2"}, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if values["customer"] != "Acme" || values["severity"] != "sev2" {
		t.Fatalf("values = %#v", values)
	}
}

func TestCustomFieldPatchCanClearOptionalValues(t *testing.T) {
	defs := []CustomFieldDefinition{{ID: "customer", Name: "Customer", Type: CustomFieldTypeText}}
	values, err := ApplyCustomFieldPatch(defs, map[string]string{"customer": "Acme"}, map[string]any{"customer": ""}, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, ok := values["customer"]; ok {
		t.Fatalf("customer should be cleared: %#v", values)
	}
}

func TestCustomFieldConflictsIgnoreMissingRequiredOnExistingCards(t *testing.T) {
	defs := []CustomFieldDefinition{{ID: "customer", Name: "Customer", Type: CustomFieldTypeText, Required: true}}
	cards := []Card{{ID: "card-1", Key: "T-1", CustomFields: map[string]string{}}}

	if conflicts := CustomFieldConflicts(defs, cards); len(conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", conflicts)
	}
}

func TestReconcileCustomFieldValuesDropsInvalidExistingValues(t *testing.T) {
	defs := []CustomFieldDefinition{
		{ID: "customer", Name: "Customer", Type: CustomFieldTypeText},
		{ID: "severity", Name: "Severity", Type: CustomFieldTypeSelect, Options: []CustomFieldOption{{ID: "sev2", Label: "SEV 2"}}},
	}

	values, changed := ReconcileCustomFieldValues(defs, map[string]string{
		"customer": "Acme",
		"severity": "sev1",
		"removed":  "legacy",
	})
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if values["customer"] != "Acme" {
		t.Fatalf("customer = %#v", values["customer"])
	}
	if _, ok := values["severity"]; ok {
		t.Fatalf("severity should be dropped: %#v", values)
	}
	if _, ok := values["removed"]; ok {
		t.Fatalf("removed should be dropped: %#v", values)
	}
}
