package events

type TaskEvent struct {
	TaskID string `json:"task_id" validate:"required"`
	Error  string `json:"error"`
}
