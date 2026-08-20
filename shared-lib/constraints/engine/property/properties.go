package property

import (
	"encoding/json"
	"fmt"

	"github.com/go-openapi/jsonpointer"
	"github.com/margo/sandbox/shared-lib/constraints/common"
	clModels "github.com/margo/sandbox/standard/generatedCode/wfm/sbi"
)

type propertySelectorEngine struct {
	device *clModels.DeviceCapabilitiesManifest
}

func New(device *clModels.DeviceCapabilitiesManifest) common.PropertySelectorEngineIface {
	return &propertySelectorEngine{
		device: device,
	}
}

func (ps *propertySelectorEngine) Evaluate(s *clModels.Selector) (bool, string) {
	result, touched := false, false
	for _, me := range s.MatchExpressions {
		switch me.Operator {
		case clModels.In:
			ok, reason := ps.HandleIn(&me)
			result, touched, reason = common.BuildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.NotIn:
			ok, reason := ps.HandleNotIn(&me)
			result, touched, reason = common.BuildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.Exists:
			ok, reason := ps.HandleExists(&me)
			result, touched, reason = common.BuildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.DoesNotExist:
			ok, reason := ps.HandleDoesNotExists(&me)
			result, touched, reason = common.BuildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.Gt:
			ok, reason := ps.HandleGt(&me)
			result, touched, reason = common.BuildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.Lt:
			ok, reason := ps.HandleLt(&me)
			result, touched, reason = common.BuildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.ContainsAll:
			ok, reason := ps.HandleContainsAll(me)
			result, touched, reason = common.BuildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		case clModels.ContainsAny:
			ok, reason := ps.HandleContainsAny(me)
			result, touched, reason = common.BuildResult(ok, reason, result, touched)
			if reason != "" {
				return false, reason
			}
		default:
			return false, fmt.Sprintf("unknown or unsupported propertySelector operator - %s", me.Operator)
		}
	}
	return result, ""
}

// resolvePointer resolves an RFC 6901 JSON Pointer against the device's properties object.
// The pointer is relative to `properties` (e.g. "/vendor", "/cpus/0/cores").
// Returns the resolved value and true on success, or nil and false if the key does not exist.
//
// Uses github.com/go-openapi/jsonpointer for RFC 6901 compliant traversal, including
// correct ~0/~1 unescaping and array index support.
func (ps *propertySelectorEngine) resolvePointer(pointer string) (any, bool, error) {
	// RFC 6901: empty string refers to the whole document.
	if pointer == "" {
		return ps.device.Properties, true, nil
	}

	// Pointer must start with '/'.
	if len(pointer) == 0 || pointer[0] != '/' {
		return nil, false, fmt.Errorf("invalid JSON Pointer %q: must start with '/'", pointer)
	}

	// Marshal Properties to a generic map so jsonpointer can traverse it uniformly.
	// This round-trip also normalises all numeric types to float64, which is what
	// HandleIn and other handlers expect after JSON decoding.
	b, err := json.Marshal(ps.device.Properties)
	if err != nil {
		return nil, false, fmt.Errorf("resolvePointer: failed to marshal device properties: %w", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, false, fmt.Errorf("resolvePointer: failed to unmarshal device properties to map: %w", err)
	}

	// Parse the pointer string.
	jp, err := jsonpointer.New(pointer)
	if err != nil {
		return nil, false, fmt.Errorf("resolvePointer: invalid JSON Pointer %q: %w", pointer, err)
	}

	// Get resolves the pointer against the document.
	// It returns (value, kind, error); kind is the reflect.Kind of the resolved node.
	val, _, err := jp.Get(doc)
	if err != nil {
		// go-openapi/jsonpointer returns an error when a token is not found.
		// Treat this as "key absent" rather than a hard error, consistent with
		// the previous behaviour and the spec ("key not found → false, not an error").
		return nil, false, nil
	}

	return val, true, nil
}

// HandleIn implements the In operator for property selectors.
//
// Per the SUP spec:
//   - Scalar (string, number, bool): true when the resolved value equals any entry in me.Values.
//   - Array of strings/numbers:      true when the array contains at least one element that
//     equals any entry in me.Values.
//   - Array of objects:              must use ContainsAll/ContainsAny; In MUST return false.
//   - Any other type:                MUST return false.
func (ps *propertySelectorEngine) HandleIn(me *clModels.MatchExpression) (bool, string) {

	if reason, ok := validateValues(me); !ok {
		return false, reason
	}

	resolved, exists, err := ps.resolvePointer(me.Key)
	if err != nil {
		return false, fmt.Sprintf("property selector: failed to resolve key %q: %v", me.Key, err)
	}
	if !exists {
		return false, fmt.Sprintf("property selector: key %q not found in device properties", me.Key)
	}

	candidates := *me.Values

	// Spec: values MUST be the same data type, and booleans MUST have exactly one value.
	if len(candidates) > 0 {
		if _, isBool := candidates[0].(bool); isBool && len(candidates) != 1 {
			return false, "invalid match expression: values MUST contain exactly one boolean for NotIn"
		}
	}

	switch val := resolved.(type) {

	// --- scalar string ---
	case string:
		for _, c := range candidates {
			if s, ok := c.(string); ok && s == val {
				return true, ""
			}
		}
		return false, fmt.Sprintf("property %q value %q not found in match expression values", me.Key, val)

	// --- scalar number (JSON numbers unmarshal as float64) ---
	case float64:
		for _, c := range candidates {
			switch cv := c.(type) {
			case float64:
				if cv == val {
					return true, ""
				}
			case float32:
				if float64(cv) == val {
					return true, ""
				}
			}
		}
		return false, fmt.Sprintf("property %q value %v not found in match expression values", me.Key, val)

	// --- scalar bool ---
	case bool:
		for _, c := range candidates {
			if b, ok := c.(bool); ok && b == val {
				return true, ""
			}
		}
		return false, fmt.Sprintf("property %q value %v not found in match expression values", me.Key, val)

	// --- array (JSON arrays unmarshal as []any) ---
	case []any:
		if len(val) == 0 {
			return false, fmt.Sprintf("property %q is an empty array, nothing to match against", me.Key)
		}

		// Peek at the first non-nil element to determine the array element type.
		// Arrays of objects must use ContainsAll/ContainsAny per the spec.
		for _, elem := range val {
			switch elem.(type) {
			case map[string]any:
				return false, fmt.Sprintf(
					"property %q is an array of objects; use ContainsAll or ContainsAny instead of In",
					me.Key,
				)
			}
			break // only need to check the first non-nil element
		}

		// Array of scalars: true if at least one element matches any candidate value.
		for _, elem := range val {
			for _, c := range candidates {
				if common.ScalarEqual(elem, c) {
					return true, ""
				}
			}
		}
		return false, fmt.Sprintf(
			"no element of property %q matched any of the match expression values",
			me.Key,
		)

	default:
		return false, fmt.Sprintf(
			"property %q resolved to an unsupported type (%T); In requires string, number, bool, or array thereof",
			me.Key, resolved,
		)
	}
}

// HandleNotIn implements the NotIn operator for property selectors.
//
// Per the SUP spec:
//   - The key MUST exist (NotIn is only true when the key is present and no values match).
//   - Scalar (string, number, bool): true when the resolved value does not equal any entry in me.Values.
//   - Array of strings/numbers:      true when no element of the array equals any entry in me.Values.
//   - Array of objects:              MUST return false (use ContainsAll/ContainsAny instead).
//   - Any other type:                MUST return false.
func (ps *propertySelectorEngine) HandleNotIn(me *clModels.MatchExpression) (bool, string) {
	if reason, ok := validateValues(me); !ok {
		return false, reason
	}

	resolved, exists, err := ps.resolvePointer(me.Key)
	if err != nil {
		return false, fmt.Sprintf("property selector: failed to resolve key %q: %v", me.Key, err)
	}
	if !exists {
		// Spec: NotIn is true only when the key exists and none of its values match.
		return false, fmt.Sprintf("property selector: key %q not found in device properties", me.Key)
	}

	candidates := *me.Values

	// Spec: values MUST be the same data type, and booleans MUST have exactly one value.
	if len(candidates) > 0 {
		if _, isBool := candidates[0].(bool); isBool && len(candidates) != 1 {
			return false, "invalid match expression: values MUST contain exactly one boolean for NotIn"
		}
	}

	switch val := resolved.(type) {

	// --- scalar string ---
	case string:
		for _, c := range candidates {
			if s, ok := c.(string); ok && s == val {
				return false, fmt.Sprintf("property %q value %q is present in match expression values", me.Key, val)
			}
		}
		return true, ""

	// --- scalar number (JSON numbers unmarshal as float64) ---
	case float64:
		for _, c := range candidates {
			switch cv := c.(type) {
			case float64:
				if cv == val {
					return false, fmt.Sprintf("property %q value %v is present in match expression values", me.Key, val)
				}
			case float32:
				if float64(cv) == val {
					return false, fmt.Sprintf("property %q value %v is present in match expression values", me.Key, val)
				}
			}
		}
		return true, ""

	// --- scalar bool ---
	case bool:
		for _, c := range candidates {
			if b, ok := c.(bool); ok && b == val {
				return false, fmt.Sprintf("property %q value %v is present in match expression values", me.Key, val)
			}
		}
		return true, ""

	// --- array (JSON arrays unmarshal as []any) ---
	case []any:
		if len(val) == 0 {
			return false, fmt.Sprintf("property %q is an empty array, nothing to match against", me.Key)
		}

		// Arrays of objects must use ContainsAll/ContainsAny per the spec.
		elem := val[0]
		if _, isObj := elem.(map[string]any); isObj {
			return false, fmt.Sprintf(
				"property %q is an array of objects; use ContainsAll or ContainsAny instead of NotIn",
				me.Key,
			)
		}

		// Array of scalars: true only if NO element matches any candidate value.
		for _, elem := range val {
			for _, c := range candidates {
				if common.ScalarEqual(elem, c) {
					return false, fmt.Sprintf(
						"property %q contains a value that is present in match expression values",
						me.Key,
					)
				}
			}
		}
		return true, ""

	default:
		return false, fmt.Sprintf(
			"property %q resolved to an unsupported type (%T); NotIn requires string, number, bool, or array thereof",
			me.Key, resolved,
		)
	}
}

// HandleExists implements the Exists operator for property selectors.
//
// Per the SUP spec:
//   - Exists: true when the referenced key is present in device properties.
//   - values MUST be omitted; if present, the expression MUST evaluate to false.
//
// The key (me.Key) MUST be a JSON Pointer (RFC 6901) relative to the
// device's properties object (e.g. "/vendor", "/cpus/0/architecture").
// resolvePointer handles all RFC 6901 traversal including ~0/~1 unescaping
// and array index support.
func (ps *propertySelectorEngine) HandleExists(me *clModels.MatchExpression) (bool, string) {
	// Spec: values MUST be omitted for Exists. Reject early to prevent
	// silent misuse where a caller passes values expecting them to be evaluated.
	if me.Values != nil && len(*me.Values) >= 0 {
		return false, "invalid match expression: values MUST be omitted for Exists"
	}

	// Resolve the JSON Pointer against the device's properties object.
	// resolvePointer returns (value, exists, error):
	//   - exists=false with err=nil means the key is simply absent (not an error).
	//   - err!=nil means the pointer itself is malformed or marshalling failed.
	_, exists, err := ps.resolvePointer(me.Key)
	if err != nil {
		return false, fmt.Sprintf("property selector: failed to resolve key %q: %v", me.Key, err)
	}

	// Key is absent: Exists must evaluate to false.
	if !exists {
		return false, fmt.Sprintf("property selector: key %q not found in device properties", me.Key)
	}

	// Key is present: Exists evaluates to true regardless of the value at that key.
	return true, ""
}

// HandleDoesNotExists implements the DoesNotExist operator for property selectors.
//
// Per the SUP spec:
//   - DoesNotExist: true when the referenced key is absent from device properties.
//   - values MUST be omitted; if present, the expression MUST evaluate to false.
//
// This is the logical complement of HandleExists. A key that resolves to nil
// or any value (including zero values) is still considered "present". Only a
// key that is entirely absent from the properties object satisfies DoesNotExist.
//
// Note: a hard resolution error (e.g. malformed pointer syntax) is surfaced
// explicitly rather than silently returning true, which would be a false positive
// and could incorrectly allow a device to pass a constraint it should not.
func (ps *propertySelectorEngine) HandleDoesNotExists(me *clModels.MatchExpression) (bool, string) {
	// Spec: values MUST be omitted for DoesNotExist. Reject early to prevent
	// silent misuse where a caller passes values expecting them to be evaluated.
	if me.Values != nil && len(*me.Values) >= 0 {
		return false, "invalid match expression: values MUST be omitted for DoesNotExist"
	}

	// Resolve the JSON Pointer against the device's properties object.
	// resolvePointer returns (value, exists, error):
	//   - exists=false with err=nil means the key is simply absent — this is
	//     the success condition for DoesNotExist.
	//   - err!=nil means the pointer is malformed or marshalling failed; surface
	//     this as an explicit error rather than treating it as "absent".
	_, exists, err := ps.resolvePointer(me.Key)
	if err != nil {
		return false, fmt.Sprintf("property selector: failed to resolve key %q: %v", me.Key, err)
	}

	// Key is present: DoesNotExist must evaluate to false.
	if exists {
		return false, fmt.Sprintf("property selector: key %q is present in device properties", me.Key)
	}

	// Key is absent: DoesNotExist evaluates to true.
	return true, ""
}

// HandleGt implements the Gt operator for property selectors.
//
// Per the SUP spec:
//   - Gt: true when the referenced property value is greater than values[0].
//   - values MUST be present, contain exactly one entry, and be parsable as a number.
//   - The resolved property value MUST be numeric; any other type MUST return false.
//
// The key (me.Key) MUST be a JSON Pointer (RFC 6901) relative to the device's
// properties object (e.g. "/cpus/0/cores"). After JSON round-trip through
// resolvePointer, all numbers are normalised to float64.
func (ps *propertySelectorEngine) HandleGt(me *clModels.MatchExpression) (bool, string) {
	// Validate that values contains exactly one number before touching the device.
	threshold, reason, ok := validateNumericValues(me)
	if !ok {
		return false, reason
	}

	// Resolve the JSON Pointer against the device's properties object.
	// resolvePointer returns (value, exists, error):
	//   - exists=false with err=nil means the key is simply absent.
	//   - err!=nil means the pointer is malformed or marshalling failed.
	resolved, exists, err := ps.resolvePointer(me.Key)
	if err != nil {
		return false, fmt.Sprintf("property selector: failed to resolve key %q: %v", me.Key, err)
	}
	if !exists {
		return false, fmt.Sprintf("property selector: key %q not found in device properties", me.Key)
	}

	// The resolved value MUST be numeric. After the JSON round-trip inside
	// resolvePointer all numbers arrive as float64; float32 is also accepted
	// defensively for values that bypass the round-trip.
	val, ok := common.ToFloat64(resolved)
	if !ok {
		return false, fmt.Sprintf(
			"property %q resolved to a non-numeric type (%T); Gt requires a number",
			me.Key, resolved,
		)
	}

	// Spec: Gt is true when the device property value is strictly greater than
	// the single threshold value supplied in values[0].
	if val > threshold {
		return true, ""
	}
	return false, fmt.Sprintf(
		"property %q value %v is not greater than %v",
		me.Key, val, threshold,
	)
}

// HandleLt implements the Lt operator for property selectors.
//
// Per the SUP spec:
//   - Lt: true when the referenced property value is less than values[0].
//   - values MUST be present, contain exactly one entry, and be parsable as a number.
//   - The resolved property value MUST be numeric; any other type MUST return false.
//
// This is the strict complement of HandleGt. The same numeric coercion and
// validation rules apply; only the final comparison direction differs.
func (ps *propertySelectorEngine) HandleLt(me *clModels.MatchExpression) (bool, string) {
	// Validate that values contains exactly one number before touching the device.
	threshold, reason, ok := validateNumericValues(me)
	if !ok {
		return false, reason
	}

	// Resolve the JSON Pointer against the device's properties object.
	// resolvePointer returns (value, exists, error):
	//   - exists=false with err=nil means the key is simply absent.
	//   - err!=nil means the pointer is malformed or marshalling failed.
	resolved, exists, err := ps.resolvePointer(me.Key)
	if err != nil {
		return false, fmt.Sprintf("property selector: failed to resolve key %q: %v", me.Key, err)
	}
	if !exists {
		return false, fmt.Sprintf("property selector: key %q not found in device properties", me.Key)
	}

	// The resolved value MUST be numeric. After the JSON round-trip inside
	// resolvePointer all numbers arrive as float64; float32 is also accepted
	// defensively for values that bypass the round-trip.
	val, ok := common.ToFloat64(resolved)
	if !ok {
		return false, fmt.Sprintf(
			"property %q resolved to a non-numeric type (%T); Lt requires a number",
			me.Key, resolved,
		)
	}

	// Spec: Lt is true when the device property value is strictly less than
	// the single threshold value supplied in values[0].
	if val < threshold {
		return true, ""
	}
	return false, fmt.Sprintf(
		"property %q value %v is not less than %v",
		me.Key, val, threshold,
	)
}

// HandleContainsAll implements the ContainsAll operator for property selectors.
//
// Per the SUP spec:
//   - The key MUST resolve to an array of objects.
//   - itemSelector MUST be present and contain one or more matchExpressions.
//   - values MUST be omitted.
//   - True when AT LEAST ONE array element satisfies ALL itemSelector.matchExpressions
//     (AND logic).
//   - Keys within itemSelector.matchExpressions are JSON Pointers relative to each
//     array element. We make them absolute by prepending me.Key + "/" + index,
//     so "/type" under "/peripherals" becomes "/peripherals/0/type".
//     This means resolvePointer always receives a complete RFC 6901 pointer
//     rooted at properties, and recursion works naturally.
func (ps *propertySelectorEngine) HandleContainsAll(me clModels.MatchExpression) (bool, string) {
	// Spec: values MUST be omitted for ContainsAll.
	if me.Values != nil && len(*me.Values) >= 0 {
		return false, "invalid match expression: values MUST be omitted for ContainsAll"
	}

	// Spec: itemSelector MUST be present with at least one matchExpression.
	if me.ItemSelector == nil || len(me.ItemSelector.MatchExpressions) == 0 {
		return false, fmt.Sprintf(
			"invalid match expression: itemSelector with at least one matchExpression is required for ContainsAll (key %q)",
			me.Key,
		)
	}

	// Resolve the parent key to confirm it exists and is an array.
	resolved, exists, err := ps.resolvePointer(me.Key)
	if err != nil {
		return false, fmt.Sprintf("property selector: failed to resolve key %q: %v", me.Key, err)
	}
	if !exists {
		return false, fmt.Sprintf("property selector: key %q not found in device properties", me.Key)
	}

	arr, ok := resolved.([]any)
	if !ok {
		return false, fmt.Sprintf(
			"property %q resolved to a non-array type (%T); ContainsAll requires an array of objects",
			me.Key, resolved,
		)
	}
	if len(arr) == 0 {
		return false, fmt.Sprintf("property %q is an empty array, nothing to match against", me.Key)
	}

	// Validate first element is an object — scalar arrays must use In/NotIn.
	if _, isObj := arr[0].(map[string]any); !isObj {
		return false, fmt.Sprintf(
			"property %q is an array of scalars; ContainsAll requires an array of objects",
			me.Key,
		)
	}

	// For each element at index i, rewrite every itemSelector matchExpression key
	// from a relative pointer (e.g. "/type") to an absolute pointer rooted at
	// properties (e.g. "/peripherals/0/type") by prepending me.Key + "/" + i.
	//
	// Then call ps.Evaluate with the rewritten selector — Evaluate ANDs all
	// matchExpressions, satisfying the ContainsAll AND requirement.
	// If any element passes Evaluate, ContainsAll is true (at-least-one semantics).
	//
	// Recursion: if a child expression is itself ContainsAll/ContainsAny, its
	// me.Key will already be absolute (e.g. "/peripherals/0/subArray"), and its
	// own children will be further prefixed with that absolute key + index.
	for i := range arr {
		// Build a rewritten selector with absolute keys for this element index.
		rewritten := clModels.Selector{
			MatchExpressions: make([]clModels.MatchExpression, len(me.ItemSelector.MatchExpressions)),
		}
		for j, child := range me.ItemSelector.MatchExpressions {
			rewritten.MatchExpressions[j] = child // copy
			rewritten.MatchExpressions[j].Key = fmt.Sprintf("%s/%d%s", me.Key, i, child.Key)
		}

		// Evaluate ANDs all matchExpressions — exactly the AND semantics ContainsAll needs.
		match, _ := ps.Evaluate(&rewritten)
		if match {
			return true, ""
		}
	}

	return false, fmt.Sprintf(
		"no element of property %q satisfied all itemSelector.matchExpressions",
		me.Key,
	)
}

// HandleContainsAny implements the ContainsAny operator for property selectors.
//
// Per the SUP spec:
//   - The key MUST resolve to an array of objects.
//   - itemSelector MUST be present and contain one or more matchExpressions.
//   - values MUST be omitted.
//   - True when AT LEAST ONE array element satisfies ANY itemSelector.matchExpression
//     (OR logic across expressions within itemSelector).
//   - Keys within itemSelector.matchExpressions are JSON Pointers relative to each
//     array element. We make them absolute by prepending me.Key + "/" + index,
//     so "/type" under "/peripherals" becomes "/peripherals/0/type".
//     This means resolvePointer always receives a complete RFC 6901 pointer
//     rooted at properties, and recursion works naturally.
//
// Contrast with HandleContainsAll:
//   - ContainsAll: one element must satisfy ALL expressions (AND).
//   - ContainsAny: one element must satisfy ANY expression  (OR).
//
// Because Evaluate() always ANDs its matchExpressions, it cannot be reused here
// for OR semantics. Instead, each rewritten child expression is dispatched
// individually via evaluateSingleExpression, and we short-circuit on the first
// true result for a given element.
func (ps *propertySelectorEngine) HandleContainsAny(me clModels.MatchExpression) (bool, string) {
	// Spec: values MUST be omitted for ContainsAny.
	if me.Values != nil && len(*me.Values) >= 0 {
		return false, "invalid match expression: values MUST be omitted for ContainsAny"
	}

	// Spec: itemSelector MUST be present with at least one matchExpression.
	if me.ItemSelector == nil || len(me.ItemSelector.MatchExpressions) == 0 {
		return false, fmt.Sprintf(
			"invalid match expression: itemSelector with at least one matchExpression is required for ContainsAny (key %q)",
			me.Key,
		)
	}

	// Resolve the parent key to confirm it exists and is an array.
	resolved, exists, err := ps.resolvePointer(me.Key)
	if err != nil {
		return false, fmt.Sprintf("property selector: failed to resolve key %q: %v", me.Key, err)
	}
	if !exists {
		return false, fmt.Sprintf("property selector: key %q not found in device properties", me.Key)
	}

	arr, ok := resolved.([]any)
	if !ok {
		return false, fmt.Sprintf(
			"property %q resolved to a non-array type (%T); ContainsAny requires an array of objects",
			me.Key, resolved,
		)
	}
	if len(arr) == 0 {
		return false, fmt.Sprintf("property %q is an empty array, nothing to match against", me.Key)
	}

	// Validate first element is an object — scalar arrays must use In/NotIn.
	if _, isObj := arr[0].(map[string]any); !isObj {
		return false, fmt.Sprintf(
			"property %q is an array of scalars; ContainsAny requires an array of objects",
			me.Key,
		)
	}

	// For each element at index i, rewrite every itemSelector matchExpression key
	// from a relative pointer (e.g. "/type") to an absolute pointer rooted at
	// properties (e.g. "/peripherals/0/type") by prepending me.Key + "/" + i.
	//
	// OR logic: for a given element, if ANY single rewritten expression evaluates
	// to true, that element satisfies ContainsAny and we return true immediately.
	//
	// Recursion: if a child expression is itself ContainsAll/ContainsAny, its
	// me.Key will already be absolute (e.g. "/peripherals/0/subArray"), and its
	// own children will be further prefixed with that absolute key + index.
	for i := range arr {
		for _, child := range me.ItemSelector.MatchExpressions {
			// Rewrite the child key to be absolute for this element index.
			rewrittenChild := child
			rewrittenChild.Key = fmt.Sprintf("%s/%d%s", me.Key, i, child.Key)

			// Evaluate this single expression — OR logic means one true is enough.
			match, _ := ps.evaluateSingleExpression(&rewrittenChild)
			if match {
				return true, ""
			}
		}
	}

	return false, fmt.Sprintf(
		"no element of property %q satisfied any itemSelector.matchExpression",
		me.Key,
	)
}

// evaluateSingleExpression dispatches a single MatchExpression to the appropriate
// HandleXXX method. It mirrors the switch in Evaluate() but operates on one
// expression at a time, returning (bool, string) without the touched/result
// accumulation that Evaluate() performs.
//
// This is needed by HandleContainsAny to apply OR logic across itemSelector
// expressions, since Evaluate() always ANDs its matchExpressions and cannot
// be reused for OR semantics.
func (ps *propertySelectorEngine) evaluateSingleExpression(me *clModels.MatchExpression) (bool, string) {
	switch me.Operator {
	case clModels.In:
		return ps.HandleIn(me)
	case clModels.NotIn:
		return ps.HandleNotIn(me)
	case clModels.Exists:
		return ps.HandleExists(me)
	case clModels.DoesNotExist:
		return ps.HandleDoesNotExists(me)
	case clModels.Gt:
		return ps.HandleGt(me)
	case clModels.Lt:
		return ps.HandleLt(me)
	case clModels.ContainsAll:
		return ps.HandleContainsAll(*me)
	case clModels.ContainsAny:
		return ps.HandleContainsAny(*me)
	default:
		return false, fmt.Sprintf("unknown or unsupported propertySelector operator - %s", me.Operator)
	}
}

// validateValues checks that me.Values conforms to the spec rules for In/NotIn:
//   - values MUST contain one or more strings or numbers, or exactly one boolean
//   - values MUST be the same data type when indicating more than one
func validateValues(me *clModels.MatchExpression) (string, bool) {
	if me.Values == nil || len(*me.Values) == 0 {
		return "invalid match expression: values is required for " + string(me.Operator), false
	}

	candidates := *me.Values

	// Determine the type of the first element
	var dominantType string
	switch candidates[0].(type) {
	case string:
		dominantType = "string"
	case float64, float32:
		dominantType = "number"
	case bool:
		dominantType = "bool"
	default:
		return fmt.Sprintf("invalid match expression: unsupported value type %T in values", candidates[0]), false
	}

	// Boolean: only one value allowed
	if dominantType == "bool" && len(candidates) != 1 {
		return "invalid match expression: values MUST contain exactly one boolean for " + string(me.Operator), false
	}

	// All elements must be the same type
	for i, c := range candidates {
		var elemType string
		switch c.(type) {
		case string:
			elemType = "string"
		case float64, float32:
			elemType = "number"
		case bool:
			elemType = "bool"
		default:
			return fmt.Sprintf("invalid match expression: unsupported value type %T at values[%d]", c, i), false
		}
		if elemType != dominantType {
			return fmt.Sprintf(
				"invalid match expression: values MUST be the same data type, got %s at values[0] but %s at values[%d]",
				dominantType, elemType, i,
			), false
		}
	}

	return "", true
}

// validateNumericValues checks that me.Values conforms to the spec rules for Gt/Lt:
//   - values MUST be present
//   - values MUST contain exactly one value
//   - that value MUST be parsable as a number (float64 or float32)
//
// Returns the single numeric value as float64 and true on success,
// or a reason string and false on failure.
func validateNumericValues(me *clModels.MatchExpression) (float64, string, bool) {
	if me.Values == nil || len(*me.Values) == 0 {
		return 0, "invalid match expression: values is required for " + string(me.Operator), false
	}

	candidates := *me.Values

	// Spec: values MUST only contain a single number for Gt/Lt.
	if len(candidates) != 1 {
		return 0, fmt.Sprintf(
			"invalid match expression: values MUST contain exactly one number for %s, got %d",
			me.Operator, len(candidates),
		), false
	}

	// Spec: the single value MUST be parsable as a number.
	threshold, ok := common.ToFloat64(candidates[0])
	if !ok {
		return 0, fmt.Sprintf(
			"invalid match expression: values[0] MUST be a number for %s, got %T",
			me.Operator, candidates[0],
		), false
	}

	return threshold, "", true
}
