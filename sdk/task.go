package sdk

import (
	"encoding/json"
)

type Task struct {
	ID           string          `json:"id"`
	ConnectorID  string          `json:"connector_id"`
	Action       ActionType      `json:"action"`
	Created      int64           `json:"created" desc:"unix timestamp, in seconds"`
	Started      int64           `json:"started" desc:"unix timestamp, in seconds"`
	Completed    int64           `json:"completed" desc:"unix timestamp, in seconds"`
	Archived     bool            `json:"archived"`
	Error        bool            `json:"error"`
	ErrorMessage string          `json:"error_message"`
	OriginalID   string          `json:"original_id"`
	Content      json.RawMessage `json:"content,omitempty"`
}

type ActionType string

const (
	ActionUpdateConfig ActionType = "update-config"
	ActionStop         ActionType = "stop"
	ActionStart        ActionType = "start"
	ActionRestore      ActionType = "restore"
)

type RestoreActionContent struct {
	ID string `json:"id" desc:"required"`
}

func (t Task) Status() string {
	switch {
	case t.Started == 0 && t.Archived:
		return "cancelled"
	case t.Started == 0:
		return "pending"
	case t.Started > 0 && t.Completed == 0:
		return "toack"
	case t.Error && !t.Archived:
		return "error"
	case t.Archived:
		return "archived"
	default:
		return "unknown"
	}
}
