package parser

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
)

const supportedStateVersion = 4

type tfState struct {
	Version   int               `json:"version"`
	Resources []tfStateResource `json:"resources"`
}

type tfStateResource struct {
	Module    string            `json:"module"`
	Mode      string            `json:"mode"`
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Instances []tfStateInstance `json:"instances"`
}

type tfStateInstance struct {
	IndexKey interface{} `json:"index_key"`
}

// LoadStateInstances reads a .tfstate file and returns instance keys
// for the given root-module resource address. Agent path only.
// Matches on module=="" and mode=="managed" too, so this can't
// collide with a same-named data source or a child-module resource.
func LoadStateInstances(statePath string, resourceType string, resourceName string) ([]string, *ParseError) {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, NewParseError(ErrStateFileUnreadable, statePath, 0, err.Error())
	}

	var state tfState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, NewParseError(ErrStateFileMalformed, statePath, 0, "invalid JSON: "+err.Error())
	}

	if state.Version != supportedStateVersion {
		return nil, NewParseError(ErrStateFileMalformed, statePath, 0,
			fmt.Sprintf("unsupported state version %d, want %d", state.Version, supportedStateVersion))
	}

	for _, r := range state.Resources {
		if r.Module == "" && r.Mode == "managed" && r.Type == resourceType && r.Name == resourceName {
			return formatIndexKeys(r.Instances), nil
		}
	}

	return nil, NewParseError(ErrResourceNotInState, statePath, 0,
		fmt.Sprintf("resource %s.%s not found in state", resourceType, resourceName))
}

func formatIndexKeys(instances []tfStateInstance) []string {
	out := make([]string, 0, len(instances))
	for _, inst := range instances {
		out = append(out, formatIndexKey(inst.IndexKey))
	}
	return out
}

// count index_keys decode as float64; for_each index_keys as string.
func formatIndexKey(key interface{}) string {
	switch v := key.(type) {
	case string:
		return v
	case float64:
		if v == math.Trunc(v) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}
