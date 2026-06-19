package clickhousex

import (
	"reflect"
	"strings"

	"github.com/shopspring/decimal"
)

// Rows exposes ClickHouse query rows without leaking the concrete driver type.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
	Err() error
	ColumnTypes() []ColumnType
}

// ColumnType describes a returned ClickHouse column.
type ColumnType struct {
	Name     string
	Type     string
	Nullable bool
}

type rowsWrapper struct {
	rows driverRows
}

func (r *rowsWrapper) Next() bool {
	return r.rows.Next()
}

func (r *rowsWrapper) Scan(dest ...any) error {
	const op = "clickhousex.Rows.Scan"
	columns := r.rows.ColumnTypes()
	if len(columns) > 0 && len(dest) != len(columns) {
		return NewError(ErrorKindColumnCountMismatch, op, "destination count must match column count", false)
	}
	for i, target := range dest {
		var column ColumnType
		if i < len(columns) {
			column = columns[i]
		}
		if err := validateScanDestination(column, target); err != nil {
			return err
		}
	}
	if err := r.rows.Scan(dest...); err != nil {
		return operationError(ErrorKindQuery, op, err)
	}
	return nil
}

func (r *rowsWrapper) Close() error {
	if err := r.rows.Close(); err != nil {
		return operationError(ErrorKindQuery, "clickhousex.Rows.Close", err)
	}
	return nil
}

func (r *rowsWrapper) Err() error {
	if err := r.rows.Err(); err != nil {
		return operationError(ErrorKindQuery, "clickhousex.Rows.Err", err)
	}
	return nil
}

func (r *rowsWrapper) ColumnTypes() []ColumnType {
	return append([]ColumnType(nil), r.rows.ColumnTypes()...)
}

func validateScanDestination(column ColumnType, dest any) error {
	const op = "clickhousex.Rows.Scan"
	if dest == nil {
		return NewError(ErrorKindTypeMismatch, op, "destination must be a non-nil pointer", false)
	}
	destType := reflect.TypeOf(dest)
	if destType.Kind() != reflect.Pointer || reflect.ValueOf(dest).IsNil() {
		return NewError(ErrorKindTypeMismatch, op, "destination must be a non-nil pointer", false)
	}
	if isAnyPointer(destType) {
		return nil
	}
	if column.Nullable && !isPointerToPointer(destType) {
		return NewError(ErrorKindTypeMismatch, op, "nullable columns must scan into a pointer destination", false)
	}
	if isDecimalType(column.Type) && !isDecimalDestination(destType) {
		return NewError(ErrorKindTypeMismatch, op, "decimal columns must scan into decimal.Decimal", false)
	}
	return nil
}

func isAnyPointer(destType reflect.Type) bool {
	return destType.Kind() == reflect.Pointer && destType.Elem().Kind() == reflect.Interface && destType.Elem().NumMethod() == 0
}

func isPointerToPointer(destType reflect.Type) bool {
	return destType.Kind() == reflect.Pointer && destType.Elem().Kind() == reflect.Pointer
}

func isDecimalDestination(destType reflect.Type) bool {
	decimalType := reflect.TypeOf(decimal.Decimal{})
	if destType.Kind() != reflect.Pointer {
		return false
	}
	elem := destType.Elem()
	if elem == decimalType {
		return true
	}
	return elem.Kind() == reflect.Pointer && elem.Elem() == decimalType
}

func isDecimalType(databaseType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(databaseType))
	for strings.HasPrefix(normalized, "nullable(") || strings.HasPrefix(normalized, "lowcardinality(") {
		open := strings.Index(normalized, "(")
		if open < 0 || !strings.HasSuffix(normalized, ")") {
			break
		}
		normalized = normalized[open+1 : len(normalized)-1]
	}
	return strings.HasPrefix(normalized, "decimal")
}
