package main

import (
	"fmt"
	"strconv"

	"github.com/xwb1989/sqlparser"
)

// AnalysisResult holds the outcome of SQL static analysis.
type AnalysisResult struct {
	StatementType string // SELECT, INSERT, UPDATE, DELETE, etc.
	IsReadOnly    bool
	LimitValue    int // 0 means no limit or not applicable
}

// Analyze performs 4-layer SQL static analysis.
// Returns an error if the SQL violates any security policy.
func Analyze(sql string, maxRows int, readOnly bool) (*AnalysisResult, error) {
	// Layer 1: Parse (injection detection built in)
	stmt, err := sqlparser.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("SQL parsing failed: %w", err)
	}

	// Determine statement type and properties
	stmtType, isRead, limitVal, err := classify(stmt)
	if err != nil {
		return nil, err
	}

	// Layer 2: Single-statement check (implicit from Parse, but double-check)
	// Parse already ensures single statement — but we verify classification succeeded.

	// Layer 3: Dangerous operation check
	if err := checkDangerous(stmt, stmtType); err != nil {
		return nil, err
	}

	// Layer 4: LIMIT check for SELECT
	if stmtType == "SELECT" {
		if limitVal == 0 {
			return nil, fmt.Errorf("SELECT must include a LIMIT clause")
		}
		if limitVal > maxRows {
			return nil, fmt.Errorf("LIMIT %d exceeds maximum %d rows for this instance", limitVal, maxRows)
		}
	}

	// Read-only enforcement
	if readOnly && !isRead {
		return nil, fmt.Errorf("write operation %q rejected: instance is read-only", stmtType)
	}

	return &AnalysisResult{
		StatementType: stmtType,
		IsReadOnly:    isRead,
		LimitValue:    limitVal,
	}, nil
}

// classify determines statement type, whether it's read-only, and the LIMIT value.
func classify(stmt sqlparser.Statement) (stmtType string, isRead bool, limitVal int, err error) {
	switch s := stmt.(type) {
	case *sqlparser.Select:
		if s.Limit != nil {
			val, pe := strconv.Atoi(sqlparser.String(s.Limit.Rowcount))
			if pe != nil {
				return "", false, 0, fmt.Errorf("cannot parse LIMIT value: %w", pe)
			}
			limitVal = val
		}
		return "SELECT", true, limitVal, nil

	case *sqlparser.Show:
		return "SHOW", true, 0, nil

	case *sqlparser.DBDDL:
		if s.Action == "create" {
			return "CREATE DATABASE", false, 0, nil
		}
		return "ALTER DATABASE", false, 0, nil

	case *sqlparser.DDL:
		action := s.Action
		if action == "create" {
			return "CREATE TABLE", false, 0, nil
		}
		if action == "alter" {
			return "ALTER TABLE", false, 0, nil
		}
		if action == "drop" {
			return "DROP TABLE", false, 0, nil
		}
		if action == "rename" {
			return "RENAME TABLE", false, 0, nil
		}
		if action == "truncate" {
			return "TRUNCATE TABLE", false, 0, nil
		}
		return fmt.Sprintf("DDL(%s)", action), false, 0, nil

	case *sqlparser.Insert:
		return "INSERT", false, 0, nil

	case *sqlparser.Update:
		return "UPDATE", false, 0, nil

	case *sqlparser.Delete:
		return "DELETE", false, 0, nil

	case *sqlparser.Set:
		return "SET", false, 0, nil

	case *sqlparser.Use:
		return "USE", false, 0, nil

	default:
		// Catch-all for statements we don't explicitly handle
		return fmt.Sprintf("%T", s), false, 0, nil
	}
}

// checkDangerous rejects known dangerous SQL operations.
func checkDangerous(stmt sqlparser.Statement, stmtType string) error {
	switch s := stmt.(type) {
	case *sqlparser.DDL:
		if s.Action == "drop" {
			return fmt.Errorf("DROP TABLE is not allowed for safety")
		}
		if s.Action == "truncate" {
			return fmt.Errorf("TRUNCATE TABLE is not allowed for safety")
		}
		if s.Action == "rename" {
			return fmt.Errorf("RENAME TABLE is not allowed for safety")
		}
	case *sqlparser.DBDDL:
		if s.Action == "drop" {
			return fmt.Errorf("DROP DATABASE is not allowed for safety")
		}
	}

	switch stmtType {
	case "USE":
		return fmt.Errorf("USE statement is not allowed (cross-database access prevented)")
	case "SET":
		return fmt.Errorf("SET statement is not allowed")
	}

	// Also block statements we couldn't classify (unknown types)
	switch t := stmt.(type) {
	case *sqlparser.OtherRead, *sqlparser.OtherAdmin:
		// Unknown node types — reject for safety
		return fmt.Errorf("unsupported SQL statement type %T", t)
	}

	return nil
}
