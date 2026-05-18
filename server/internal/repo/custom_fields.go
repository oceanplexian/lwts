package repo

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	CustomFieldTypeText   = "text"
	CustomFieldTypeSelect = "select"

	MaxCustomFields           = 50
	MaxCustomFieldOptions     = 50
	MaxCustomFieldNameLength  = 80
	MaxCustomFieldTextLength  = 2048
	MaxCustomFieldValueLength = 2048
)

var (
	customFieldIDRe    = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	customFieldColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	reservedFieldIDs   = map[string]bool{
		"assignee": true, "assignee_id": true, "attachments": true, "board": true, "board_id": true,
		"column": true, "column_id": true, "comments": true, "created": true, "created_at": true,
		"description": true, "due": true, "due_date": true, "epic": true, "epic_id": true,
		"id": true, "key": true, "points": true, "priority": true, "project": true,
		"project_key": true, "reporter": true, "reporter_id": true, "status": true,
		"tag": true, "title": true, "type": true, "updated": true, "updated_at": true,
		"version": true,
	}
)

type CustomFieldDefinition struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Type     string              `json:"type"`
	Required bool                `json:"required,omitempty"`
	Options  []CustomFieldOption `json:"options,omitempty"`
}

type CustomFieldOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Color string `json:"color,omitempty"`
}

type FieldValidationError struct {
	Fields map[string]string
}

func (e *FieldValidationError) Error() string {
	return "validation failed"
}

type CustomFieldValueConflict struct {
	CardID string            `json:"card_id"`
	Key    string            `json:"key"`
	Fields map[string]string `json:"fields"`
}

func IsFieldValidationError(err error) (map[string]string, bool) {
	if err == nil {
		return nil, false
	}
	if e, ok := err.(*FieldValidationError); ok {
		return e.Fields, true
	}
	return nil, false
}

func IsCustomFieldID(s string) bool {
	return customFieldIDRe.MatchString(s)
}

func ParseCustomFieldsJSON(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(values))
	for k, v := range values {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func MarshalCustomFieldsJSON(values map[string]string) (string, error) {
	if len(values) == 0 {
		return "{}", nil
	}
	out := make(map[string]string, len(values))
	for k, v := range values {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func NormalizeBoardSettings(raw string) (string, []CustomFieldDefinition, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "{}"
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return "", nil, fmt.Errorf("invalid settings JSON")
	}
	if settings == nil {
		settings = map[string]json.RawMessage{}
	}

	defs := []CustomFieldDefinition{}
	if rawDefs, ok := settings["custom_fields"]; ok {
		if string(rawDefs) == "null" {
			delete(settings, "custom_fields")
		} else {
			var parsed []CustomFieldDefinition
			if err := json.Unmarshal(rawDefs, &parsed); err != nil {
				return "", nil, &FieldValidationError{Fields: map[string]string{"custom_fields": "must be an array"}}
			}
			normalized, err := ValidateCustomFieldDefinitions(parsed)
			if err != nil {
				return "", nil, err
			}
			defs = normalized
			normalizedJSON, _ := json.Marshal(normalized)
			settings["custom_fields"] = normalizedJSON
		}
	}

	out, err := json.Marshal(settings)
	if err != nil {
		return "", nil, err
	}
	return string(out), defs, nil
}

func CustomFieldDefinitionsFromSettings(raw string) ([]CustomFieldDefinition, error) {
	_, defs, err := NormalizeBoardSettings(raw)
	return defs, err
}

func ValidateCustomFieldDefinitions(defs []CustomFieldDefinition) ([]CustomFieldDefinition, error) {
	fields := map[string]string{}
	if len(defs) > MaxCustomFields {
		fields["custom_fields"] = fmt.Sprintf("at most %d custom fields are allowed", MaxCustomFields)
		return nil, &FieldValidationError{Fields: fields}
	}

	seenIDs := map[string]bool{}
	seenNames := map[string]bool{}
	out := make([]CustomFieldDefinition, 0, len(defs))
	for i, def := range defs {
		prefix := fmt.Sprintf("custom_fields.%d", i)
		def.ID = strings.ToLower(strings.TrimSpace(def.ID))
		def.Name = strings.TrimSpace(def.Name)
		def.Type = normalizeCustomFieldType(def.Type)

		if def.ID == "" {
			fields[prefix+".id"] = "required"
		} else if !customFieldIDRe.MatchString(def.ID) {
			fields[prefix+".id"] = "must start with a lowercase letter and contain only lowercase letters, numbers, underscores, or hyphens"
		} else if reservedFieldIDs[def.ID] {
			fields[prefix+".id"] = "reserved field id"
		} else if seenIDs[def.ID] {
			fields[prefix+".id"] = "duplicate field id"
		}
		seenIDs[def.ID] = true

		nameKey := strings.ToLower(def.Name)
		if def.Name == "" {
			fields[prefix+".name"] = "required"
		} else if len(def.Name) > MaxCustomFieldNameLength {
			fields[prefix+".name"] = fmt.Sprintf("must be %d characters or fewer", MaxCustomFieldNameLength)
		} else if seenNames[nameKey] {
			fields[prefix+".name"] = "duplicate field name"
		}
		seenNames[nameKey] = true

		switch def.Type {
		case CustomFieldTypeText:
			def.Options = nil
		case CustomFieldTypeSelect:
			options, optionFields := validateCustomFieldOptions(prefix+".options", def.Options)
			for k, v := range optionFields {
				fields[k] = v
			}
			def.Options = options
		default:
			fields[prefix+".type"] = "must be text or select"
		}

		out = append(out, def)
	}

	if len(fields) > 0 {
		return nil, &FieldValidationError{Fields: fields}
	}
	return out, nil
}

func ApplyCustomFieldPatch(defs []CustomFieldDefinition, existing map[string]string, patch map[string]any, enforceRequired bool) (map[string]string, error) {
	out := make(map[string]string, len(existing)+len(patch))
	for k, v := range existing {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	for k, raw := range patch {
		key := strings.TrimSpace(k)
		if raw == nil {
			delete(out, key)
			continue
		}
		s, ok := raw.(string)
		if !ok {
			return nil, &FieldValidationError{Fields: map[string]string{"custom_fields." + key: "must be a string or null"}}
		}
		s = strings.TrimSpace(s)
		if s == "" {
			delete(out, key)
			continue
		}
		out[key] = s
	}
	if err := ValidateCustomFieldValues(defs, out, enforceRequired); err != nil {
		return nil, err
	}
	return out, nil
}

func ValidateCustomFieldValues(defs []CustomFieldDefinition, values map[string]string, enforceRequired bool) error {
	defByID := CustomFieldDefinitionMap(defs)
	fields := map[string]string{}

	for id, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		def, ok := defByID[id]
		if !ok {
			fields["custom_fields."+id] = "field is not configured for this board"
			continue
		}
		if len(value) > MaxCustomFieldValueLength {
			fields["custom_fields."+id] = fmt.Sprintf("must be %d characters or fewer", MaxCustomFieldValueLength)
			continue
		}
		switch def.Type {
		case CustomFieldTypeText:
			// Any non-empty string is valid.
		case CustomFieldTypeSelect:
			if !customFieldOptionExists(def, value) {
				fields["custom_fields."+id] = "must be one of the configured options"
			}
		default:
			fields["custom_fields."+id] = "unsupported field type"
		}
	}

	if enforceRequired {
		for _, def := range defs {
			if !def.Required {
				continue
			}
			if strings.TrimSpace(values[def.ID]) == "" {
				fields["custom_fields."+def.ID] = "required"
			}
		}
	}

	if len(fields) > 0 {
		return &FieldValidationError{Fields: fields}
	}
	return nil
}

func CustomFieldDefinitionMap(defs []CustomFieldDefinition) map[string]CustomFieldDefinition {
	out := make(map[string]CustomFieldDefinition, len(defs))
	for _, def := range defs {
		out[def.ID] = def
	}
	return out
}

func CustomFieldConflicts(defs []CustomFieldDefinition, cards []Card) []CustomFieldValueConflict {
	conflicts := []CustomFieldValueConflict{}
	for _, c := range cards {
		if err := ValidateCustomFieldValues(defs, c.CustomFields, false); err != nil {
			if fields, ok := IsFieldValidationError(err); ok {
				conflicts = append(conflicts, CustomFieldValueConflict{
					CardID: c.ID,
					Key:    c.Key,
					Fields: fields,
				})
			}
		}
	}
	return conflicts
}

func ReconcileCustomFieldValues(defs []CustomFieldDefinition, values map[string]string) (map[string]string, bool) {
	defByID := CustomFieldDefinitionMap(defs)
	out := make(map[string]string, len(values))
	changed := false

	for id, value := range values {
		if strings.TrimSpace(value) == "" {
			changed = true
			continue
		}
		def, ok := defByID[id]
		if !ok {
			changed = true
			continue
		}
		if len(value) > MaxCustomFieldValueLength {
			changed = true
			continue
		}
		switch def.Type {
		case CustomFieldTypeText:
			out[id] = value
		case CustomFieldTypeSelect:
			if customFieldOptionExists(def, value) {
				out[id] = value
			} else {
				changed = true
			}
		default:
			changed = true
		}
	}

	if len(out) != len(values) {
		changed = true
	}
	return out, changed
}

func normalizeCustomFieldType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "choice", "single_select", "single-select", "select":
		return CustomFieldTypeSelect
	case "text":
		return CustomFieldTypeText
	default:
		return strings.ToLower(strings.TrimSpace(t))
	}
}

func validateCustomFieldOptions(prefix string, options []CustomFieldOption) ([]CustomFieldOption, map[string]string) {
	fields := map[string]string{}
	if len(options) == 0 {
		fields[prefix] = "select fields require at least one option"
		return nil, fields
	}
	if len(options) > MaxCustomFieldOptions {
		fields[prefix] = fmt.Sprintf("at most %d options are allowed", MaxCustomFieldOptions)
		return nil, fields
	}

	seenIDs := map[string]bool{}
	seenLabels := map[string]bool{}
	out := make([]CustomFieldOption, 0, len(options))
	for i, opt := range options {
		itemPrefix := fmt.Sprintf("%s.%d", prefix, i)
		opt.ID = strings.ToLower(strings.TrimSpace(opt.ID))
		opt.Label = strings.TrimSpace(opt.Label)
		opt.Color = strings.TrimSpace(opt.Color)

		if opt.ID == "" {
			opt.ID = slugifyCustomFieldID(opt.Label, "option")
		}
		if opt.ID == "" {
			fields[itemPrefix+".id"] = "required"
		} else if !customFieldIDRe.MatchString(opt.ID) {
			fields[itemPrefix+".id"] = "must start with a lowercase letter and contain only lowercase letters, numbers, underscores, or hyphens"
		} else if seenIDs[opt.ID] {
			fields[itemPrefix+".id"] = "duplicate option id"
		}
		seenIDs[opt.ID] = true

		labelKey := strings.ToLower(opt.Label)
		if opt.Label == "" {
			fields[itemPrefix+".label"] = "required"
		} else if len(opt.Label) > MaxCustomFieldNameLength {
			fields[itemPrefix+".label"] = fmt.Sprintf("must be %d characters or fewer", MaxCustomFieldNameLength)
		} else if seenLabels[labelKey] {
			fields[itemPrefix+".label"] = "duplicate option label"
		}
		seenLabels[labelKey] = true

		if opt.Color != "" && !customFieldColorRe.MatchString(opt.Color) {
			fields[itemPrefix+".color"] = "must be a #RRGGBB color"
		}
		out = append(out, opt)
	}
	return out, fields
}

func customFieldOptionExists(def CustomFieldDefinition, value string) bool {
	for _, opt := range def.Options {
		if opt.ID == value {
			return true
		}
	}
	return false
}

func slugifyCustomFieldID(s, fallback string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = fallback
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = fallback + "-" + out
	}
	if len(out) > 64 {
		out = strings.TrimRight(out[:64], "-")
	}
	return out
}
