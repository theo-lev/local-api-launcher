package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseEnvVars(t *testing.T) {
	raw := "" +
		"# a comment\n" +
		"\n" +
		"  SPRING_PROFILES_ACTIVE = dev \n" + // trimmed key, value keeps inner spacing minus line trim
		"DB_URL=jdbc:postgresql://localhost:5432/app\n" +
		"NO_VALUE=\n" + // empty value is valid
		"loose line without equals\n" + // skipped
		"=novalue\n" + // empty key is skipped
		"TOKEN=\"abc=123\"" // verbatim value, only first '=' splits

	got := parseEnvVars(raw)
	want := []string{
		"SPRING_PROFILES_ACTIVE= dev", // key trimmed; value keeps its leading space
		"DB_URL=jdbc:postgresql://localhost:5432/app",
		"NO_VALUE=",
		"TOKEN=\"abc=123\"",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseEnvVars mismatch:\n got %#v\nwant %#v", got, want)
	}
}

// A config written before the upgrade stored pids as map[string]int; it must
// still load, mapping each pid into a ProcInfo.
func TestProcMapLegacyUnmarshal(t *testing.T) {
	var c config
	if err := json.Unmarshal([]byte(`{"pids":{"abc":1234}}`), &c); err != nil {
		t.Fatal(err)
	}
	if got := c.Pids["abc"]; got.Pid != 1234 || got.EnvID != "" {
		t.Fatalf("legacy pid not migrated, got %#v", got)
	}
}

func TestProcMapNewUnmarshal(t *testing.T) {
	var c config
	if err := json.Unmarshal([]byte(`{"pids":{"abc":{"pid":99,"envId":"e1","envName":"dev"}}}`), &c); err != nil {
		t.Fatal(err)
	}
	if got := c.Pids["abc"]; got.Pid != 99 || got.EnvName != "dev" {
		t.Fatalf("new pid shape not parsed, got %#v", got)
	}
}
