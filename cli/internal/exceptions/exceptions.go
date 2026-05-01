package exceptions

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	upperIDPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]*$`)
)

var (
	severities = []string{"low", "medium", "high", "critical"}
	statuses   = []string{"active", "expired", "revoked", "superseded"}
)

type exceptionsManifest struct {
	Exceptions []exception `yaml:"exceptions"`
}

type exception struct {
	ID          string `yaml:"id"`
	Summary     string `yaml:"summary"`
	Owner       string `yaml:"owner"`
	Approver    string `yaml:"approver"`
	Severity    string `yaml:"severity"`
	Status      string `yaml:"status"`
	CreatedAt   string `yaml:"created_at"`
	ExpiresAt   string `yaml:"expires_at"`
	Reason      string `yaml:"reason"`
	Scope       string `yaml:"scope"`
	DecisionRef string `yaml:"decision_ref"`
}

type CheckResult struct {
	Valid      bool             `json:"valid"`
	Exceptions []ExceptionCheck `json:"exceptions"`
}

type ExceptionCheck struct {
	ID     string       `json:"id"`
	Valid  bool         `json:"valid"`
	Errors []CheckError `json:"errors,omitempty"`
}

type CheckError struct {
	ExceptionID string `json:"exception_id"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

func CheckExceptions(manifestPath string, now time.Time) (CheckResult, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return CheckResult{}, fmt.Errorf("reading manifest: %w", err)
	}
	return CheckExceptionsFromBytes(data, now)
}

func CheckExceptionsFromBytes(data []byte, now time.Time) (CheckResult, error) {
	var manifest exceptionsManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return CheckResult{}, fmt.Errorf("parsing manifest: %w", err)
	}

	result := CheckResult{Valid: true}
	for _, exc := range manifest.Exceptions {
		ec := checkException(exc, now)
		if !ec.Valid {
			result.Valid = false
		}
		result.Exceptions = append(result.Exceptions, ec)
	}
	return result, nil
}

func checkException(exc exception, now time.Time) ExceptionCheck {
	ec := ExceptionCheck{
		ID:    exc.ID,
		Valid: true,
	}

	checkID(&ec, exc)
	checkSummary(&ec, exc)
	checkOwner(&ec, exc)
	checkApprover(&ec, exc)
	checkSeverity(&ec, exc)
	checkStatus(&ec, exc)
	checkReason(&ec, exc)
	checkExpiration(&ec, exc, now)

	ec.Valid = len(ec.Errors) == 0
	return ec
}

func checkID(ec *ExceptionCheck, exc exception) {
	if strings.TrimSpace(exc.ID) == "" {
		addError(ec, exc.ID, "MISSING_EXCEPTION_ID", "exception id is required")
		return
	}
	if !upperIDPattern.MatchString(exc.ID) {
		addError(ec, exc.ID, "INVALID_EXCEPTION_ID",
			fmt.Sprintf("id %q must match %s", exc.ID, upperIDPattern.String()))
	}
}

func checkSummary(ec *ExceptionCheck, exc exception) {
	if strings.TrimSpace(exc.Summary) == "" {
		addError(ec, exc.ID, "MISSING_SUMMARY", "summary is required")
	}
}

func checkOwner(ec *ExceptionCheck, exc exception) {
	if strings.TrimSpace(exc.Owner) == "" {
		addError(ec, exc.ID, "NO_OWNER", "owner is required")
	}
}

func checkApprover(ec *ExceptionCheck, exc exception) {
	if strings.TrimSpace(exc.Approver) == "" {
		addError(ec, exc.ID, "NO_APPROVER", "approver is required")
	}
}

func checkSeverity(ec *ExceptionCheck, exc exception) {
	if strings.TrimSpace(exc.Severity) == "" {
		addError(ec, exc.ID, "MISSING_SEVERITY", "severity is required")
		return
	}
	found := false
	for _, s := range severities {
		if s == exc.Severity {
			found = true
			break
		}
	}
	if !found {
		addError(ec, exc.ID, "INVALID_SEVERITY",
			fmt.Sprintf("severity %q must be one of: %s", exc.Severity, strings.Join(severities, ", ")))
	}
}

func checkStatus(ec *ExceptionCheck, exc exception) {
	if strings.TrimSpace(exc.Status) == "" {
		addError(ec, exc.ID, "MISSING_STATUS", "status is required")
		return
	}
	found := false
	for _, s := range statuses {
		if s == exc.Status {
			found = true
			break
		}
	}
	if !found {
		addError(ec, exc.ID, "INVALID_STATUS",
			fmt.Sprintf("status %q must be one of: %s", exc.Status, strings.Join(statuses, ", ")))
	}
}

func checkReason(ec *ExceptionCheck, exc exception) {
	if strings.TrimSpace(exc.Reason) == "" {
		addError(ec, exc.ID, "MISSING_REASON", "reason is required")
	}
}

func checkExpiration(ec *ExceptionCheck, exc exception, now time.Time) {
	if strings.TrimSpace(exc.ExpiresAt) == "" {
		addError(ec, exc.ID, "MISSING_EXPIRATION", "expires_at is required")
		return
	}
	expiresAt, err := time.Parse("2006-01-02", exc.ExpiresAt)
	if err != nil {
		addError(ec, exc.ID, "INVALID_EXPIRATION",
			fmt.Sprintf("expires_at %q must be a valid date (YYYY-MM-DD)", exc.ExpiresAt))
		return
	}
	if exc.Status == "active" && now.After(expiresAt) {
		addError(ec, exc.ID, "EXCEPTION_EXPIRED",
			fmt.Sprintf("exception expired on %s and is still active", exc.ExpiresAt))
	}
}

func addError(ec *ExceptionCheck, exceptionID string, code string, message string) {
	ec.Errors = append(ec.Errors, CheckError{
		ExceptionID: exceptionID,
		Code:        code,
		Message:     message,
	})
}
