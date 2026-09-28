// pkg/standard/generatedCode/response.go
package generatedCode

// NewProblemDetail creates a ProblemDetail with the required fields.
// type_: URI identifying the problem type (use Margo URIs for spec-defined errors)
// title: short human-readable summary
// status: HTTP status code
func NewProblemDetail(type_, title string, status int) ProblemDetail {
	return ProblemDetail{
		Type:   type_,
		Title:  title,
		Status: status,
	}
}

// WithDetail adds a human-readable explanation of this specific occurrence.
func (p ProblemDetail) WithDetail(detail string) ProblemDetail {
	p.Detail = &detail
	return p
}

// WithInstance adds a URI reference identifying the specific occurrence.
func (p ProblemDetail) WithInstance(instance string) ProblemDetail {
	p.Instance = &instance
	return p
}

// WithRetryable sets whether the client may retry the request.
func (p ProblemDetail) WithRetryable(retryable bool) ProblemDetail {
	p.Retryable = &retryable
	return p
}

// WithRetryAfterSeconds sets the advisory retry delay in seconds.
func (p ProblemDetail) WithRetryAfterSeconds(seconds int) ProblemDetail {
	p.RetryAfterSeconds = &seconds
	return p
}

// WithBackoffStrategy sets the recommended backoff strategy.
// Use the constants: None, Fixed, Exponential.
func (p ProblemDetail) WithBackoffStrategy(strategy ProblemDetailBackoffStrategy) ProblemDetail {
	p.BackoffStrategy = &strategy
	return p
}

// WithErrors adds field-level validation errors (for 422 responses).
func (p ProblemDetail) WithErrors(errors []struct {
	Field   *string `json:"field,omitempty"`
	Message *string `json:"message,omitempty"`
},
) ProblemDetail {
	p.Errors = &errors
	return p
}

// WithExtension adds a vendor-specific extension field.
// Key must not collide with standard fields; use your own URI namespace.
func (p ProblemDetail) WithExtension(key string, value interface{}) ProblemDetail {
	p.Set(key, value)
	return p
}
