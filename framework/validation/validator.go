// Package validation provides struct-tag-driven input validation for OniWorks.
//
// Rules are declared as `validate:"required,min=3,max=255,email"` struct tags.
// The validator is allocation-light: rule lists are cached per type.
package validation

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// Errors holds validation failures keyed by field name.
type Errors map[string][]string

// Error implements the error interface; it prints all field errors.
func (e Errors) Error() string {
	if len(e) == 0 {
		return "validation: no errors"
	}
	var b strings.Builder
	for field, msgs := range e {
		b.WriteString(field + ": " + strings.Join(msgs, ", ") + "; ")
	}
	return strings.TrimSuffix(b.String(), "; ")
}

// Any reports whether there are any validation errors.
func (e Errors) Any() bool { return len(e) > 0 }

// Add appends a message for a field.
func (e Errors) Add(field, msg string) { e[field] = append(e[field], msg) }

// ─────────────────────────── Validator ────────────────────────────

// Validator validates structs against `validate` struct tags.
type Validator struct {
	cache    sync.Map // map[reflect.Type][]fieldRule
	customs  map[string]RuleFunc
	customMu sync.RWMutex
}

// RuleFunc is a custom validation rule. It returns an error message on failure, or "".
type RuleFunc func(value any, param string) string

// New creates a Validator. The zero value is not usable; always use New().
func New() *Validator {
	return &Validator{customs: make(map[string]RuleFunc)}
}

// Register adds a custom validation rule.
//
//	v.Register("phone", func(val any, _ string) string {
//	    s, _ := val.(string)
//	    if !phoneRe.MatchString(s) { return "must be a valid phone number" }
//	    return ""
//	})
func (v *Validator) Register(name string, fn RuleFunc) {
	v.customMu.Lock()
	v.customs[name] = fn
	v.customMu.Unlock()
}

// Validate validates s (a struct pointer) against its `validate` tags.
// Returns nil if all fields pass. Returns Errors (implementing error) on failure.
func (v *Validator) Validate(s any) error {
	errs := make(Errors)
	v.validateStruct(reflect.ValueOf(s), errs, "")
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// ValidateMap validates a map[string]any against a rules map.
//
//	errs := v.ValidateMap(data, validation.Rules{
//	    "email": "required,email",
//	    "age":   "required,min=18",
//	})
func (v *Validator) ValidateMap(data map[string]any, rules Rules) Errors {
	errs := make(Errors)
	for field, ruleStr := range rules {
		val := data[field]
		parsed := parseRuleString(ruleStr)
		for _, r := range parsed {
			if msg := v.applyRule(r.name, val, r.param); msg != "" {
				errs.Add(field, msg)
			}
		}
	}
	return errs
}

// Rules is a map of field name → rule string for use with ValidateMap.
type Rules map[string]string

func (v *Validator) validateStruct(rv reflect.Value, errs Errors, prefix string) {
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return
	}

	rt := rv.Type()
	rules := v.structRules(rt)

	for _, fr := range rules {
		fv := rv.Field(fr.index)
		fieldName := prefix + fr.name
		for _, r := range fr.rules {
			if msg := v.applyRule(r.name, fv.Interface(), r.param); msg != "" {
				errs.Add(fieldName, msg)
			}
		}
		// Recurse into nested structs
		if fv.Kind() == reflect.Struct || (fv.Kind() == reflect.Ptr && !fv.IsNil()) {
			v.validateStruct(fv, errs, fieldName+".")
		}
	}
}

// ─────────────────────────── rule cache ───────────────────────────

type fieldRule struct {
	index int
	name  string
	rules []parsedRule
}

type parsedRule struct {
	name  string
	param string
}

func (v *Validator) structRules(rt reflect.Type) []fieldRule {
	if cached, ok := v.cache.Load(rt); ok {
		return cached.([]fieldRule)
	}
	rules := buildFieldRules(rt)
	v.cache.Store(rt, rules)
	return rules
}

func buildFieldRules(rt reflect.Type) []fieldRule {
	var result []fieldRule
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := f.Tag.Get("validate")
		if tag == "" || tag == "-" {
			continue
		}
		name := f.Tag.Get("json")
		if name == "" || name == "-" {
			name = strings.ToLower(f.Name)
		}
		name = strings.Split(name, ",")[0]

		result = append(result, fieldRule{
			index: i,
			name:  name,
			rules: parseRuleString(tag),
		})
	}
	return result
}

func parseRuleString(s string) []parsedRule {
	parts := strings.Split(s, ",")
	rules := make([]parsedRule, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if idx := strings.Index(p, "="); idx >= 0 {
			rules = append(rules, parsedRule{name: p[:idx], param: p[idx+1:]})
		} else {
			rules = append(rules, parsedRule{name: p})
		}
	}
	return rules
}

// ─────────────────────────── built-in rules ───────────────────────

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var urlRe = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
var alphanumRe = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

func (v *Validator) applyRule(rule string, val any, param string) string {
	// Check custom rules first
	v.customMu.RLock()
	fn, ok := v.customs[rule]
	v.customMu.RUnlock()
	if ok {
		return fn(val, param)
	}

	str := fmt.Sprintf("%v", val)
	var isZero bool
	if val == nil {
		isZero = true
	} else if t := reflect.TypeOf(val); t != nil {
		isZero = reflect.DeepEqual(val, reflect.Zero(t).Interface())
	}

	switch rule {
	case "required":
		if val == nil || str == "" || isZero {
			return "is required"
		}
	case "email":
		if str != "" && !emailRe.MatchString(str) {
			return "must be a valid email address"
		}
	case "url":
		if str != "" && !urlRe.MatchString(str) {
			return "must be a valid URL"
		}
	case "alpha":
		if str != "" && !regexp.MustCompile(`^[a-zA-Z]+$`).MatchString(str) {
			return "must contain only letters"
		}
	case "alphanum":
		if str != "" && !alphanumRe.MatchString(str) {
			return "must contain only letters and numbers"
		}
	case "numeric":
		if str != "" {
			if _, err := strconv.ParseFloat(str, 64); err != nil {
				return "must be a number"
			}
		}
	case "min":
		n, _ := strconv.Atoi(param)
		if s, ok := val.(string); ok {
			if utf8.RuneCountInString(s) < n {
				return fmt.Sprintf("must be at least %d characters", n)
			}
		} else if i, err := toFloat(val); err == nil {
			if i < float64(n) {
				return fmt.Sprintf("must be at least %d", n)
			}
		}
	case "max":
		n, _ := strconv.Atoi(param)
		if s, ok := val.(string); ok {
			if utf8.RuneCountInString(s) > n {
				return fmt.Sprintf("must be at most %d characters", n)
			}
		} else if i, err := toFloat(val); err == nil {
			if i > float64(n) {
				return fmt.Sprintf("must be at most %d", n)
			}
		}
	case "len":
		n, _ := strconv.Atoi(param)
		if s, ok := val.(string); ok {
			if utf8.RuneCountInString(s) != n {
				return fmt.Sprintf("must be exactly %d characters", n)
			}
		}
	case "oneof":
		opts := strings.Split(param, " ")
		found := false
		for _, o := range opts {
			if str == o {
				found = true
				break
			}
		}
		if !found {
			return fmt.Sprintf("must be one of: %s", strings.Join(opts, ", "))
		}
	case "gt":
		n, _ := strconv.ParseFloat(param, 64)
		if i, err := toFloat(val); err == nil && i <= n {
			return fmt.Sprintf("must be greater than %s", param)
		}
	case "gte":
		n, _ := strconv.ParseFloat(param, 64)
		if i, err := toFloat(val); err == nil && i < n {
			return fmt.Sprintf("must be greater than or equal to %s", param)
		}
	case "lt":
		n, _ := strconv.ParseFloat(param, 64)
		if i, err := toFloat(val); err == nil && i >= n {
			return fmt.Sprintf("must be less than %s", param)
		}
	case "lte":
		n, _ := strconv.ParseFloat(param, 64)
		if i, err := toFloat(val); err == nil && i > n {
			return fmt.Sprintf("must be less than or equal to %s", param)
		}
	case "regex":
		re, err := regexp.Compile(param)
		if err == nil && str != "" && !re.MatchString(str) {
			return "format is invalid"
		}
	case "uuid":
		uuidRe := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
		if str != "" && !uuidRe.MatchString(str) {
			return "must be a valid UUID"
		}
	}
	return ""
}

func toFloat(v any) (float64, error) {
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case float32:
		return float64(n), nil
	case float64:
		return n, nil
	case string:
		return strconv.ParseFloat(n, 64)
	}
	return 0, fmt.Errorf("not a number")
}
