package sdk

import (
	"encoding/json"

	"github.com/glimps-re/connector-integration/sdk/validation"
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

func (ActionType) Values() []ActionType {
	return []ActionType{ActionUpdateConfig, ActionStop, ActionStart, ActionRestore}
}

// TaskActionTag is the validator tag validating an ActionType.
const TaskActionTag = "task_action"

func (ActionType) Validation() validation.EnumValidation {
	return validation.NewEnumValidation(ActionType("").Values())
}

type RestoreActionContent struct {
	ID string `json:"id" desc:"required"`
}

type TaskStatus string

const (
	StatusCancelled TaskStatus = "cancelled"
	StatusPending   TaskStatus = "pending"
	StatusToAck     TaskStatus = "toack"
	StatusError     TaskStatus = "error"
	StatusArchived  TaskStatus = "archived"
	StatusUnknown   TaskStatus = "unknown"
)

func (TaskStatus) Values() []TaskStatus {
	return []TaskStatus{StatusCancelled, StatusPending, StatusToAck, StatusError, StatusArchived, StatusUnknown}
}

// FilterableValues lists the statuses usable as a search filter. 'unknown' is
// excluded: it is a fallback in Status() and has no corresponding search filter.
func (TaskStatus) FilterableValues() []TaskStatus {
	return []TaskStatus{StatusCancelled, StatusPending, StatusToAck, StatusError, StatusArchived}
}

// TaskStatusTag is the validator tag validating a TaskStatus search filter.
const TaskStatusTag = "task_status"

func (TaskStatus) Validation() validation.EnumValidation {
	return validation.NewEnumValidation(TaskStatus("").FilterableValues())
}

func (t Task) Status() string {
	switch {
	case t.Started == 0 && t.Archived:
		return string(StatusCancelled)
	case t.Started == 0:
		return string(StatusPending)
	case t.Started > 0 && t.Completed == 0:
		return string(StatusToAck)
	case t.Error && !t.Archived:
		return string(StatusError)
	case t.Archived:
		return string(StatusArchived)
	default:
		return string(StatusUnknown)
	}
}
