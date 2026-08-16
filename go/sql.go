package natyv

import (
	"encoding/json"
	"errors"

	"github.com/extism/go-pdk"
)

//go:wasmimport extism:host/user sqlite_exec
func sqliteExecHost(uint64) uint64

//go:wasmimport extism:host/user sqlite_query
func sqliteQueryHost(uint64) uint64

type sqlRequest struct {
	Sql    string   `json:"sql"`
	Params []string `json:"params"`
}

type ExecResult struct {
	RowsAffected int64
	LastInsertID int64
}

func SqlExec(sql string, params []string) (ExecResult, error) {
	body, err := json.Marshal(sqlRequest{Sql: sql, Params: params})
	if err != nil {
		return ExecResult{}, err
	}
	var resp struct {
		RowsAffected int64  `json:"rows_affected"`
		LastInsertId int64  `json:"last_insert_id"`
		Error        string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(sqliteExecHost(pdk.ResultBytes(body))), &resp); err != nil {
		return ExecResult{}, err
	}
	if resp.Error != "" {
		return ExecResult{}, errors.New(resp.Error)
	}
	return ExecResult{RowsAffected: resp.RowsAffected, LastInsertID: resp.LastInsertId}, nil
}

func SqlQuery(sql string, params []string) ([]map[string]string, error) {
	body, err := json.Marshal(sqlRequest{Sql: sql, Params: params})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Rows  []map[string]string `json:"rows"`
		Error string               `json:"error,omitempty"`
	}
	if err := json.Unmarshal(pdk.ParamBytes(sqliteQueryHost(pdk.ResultBytes(body))), &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	return resp.Rows, nil
}
