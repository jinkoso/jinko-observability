package logger

import "time"

// Field is a key/value pair attached to a single log entry. Values are
// mapped to Zap's typed fields at emit time; anything not in the explicit
// type-switch falls back to zap.Any.
type Field struct {
	Key   string
	Value interface{}
}

// String returns a Field for a string value.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int returns a Field for an int value.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 returns a Field for an int64 value.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 returns a Field for a float64 value.
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool returns a Field for a bool value.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Duration returns a Field for a time.Duration value.
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Time returns a Field for a time.Time value.
func Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

// Any returns a Field for an arbitrary value. Uses Zap's reflection-based
// encoding — avoid in hot paths.
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Error returns a Field under the reserved "error" key.
func Error(err error) Field {
	return Field{Key: "error", Value: err}
}
