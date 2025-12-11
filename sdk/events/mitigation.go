package events

import (
	"context"
	"time"
)

type EventMitigationHandler interface {
	NotifyFileMitigation(ctx context.Context, action MitigationAction, elementID string, reason MitigationReason, info FileInfos) (err error)
	NotifyEmailMitigation(ctx context.Context, action MitigationAction, elementID string, reason MitigationReason, info EmailInfos) (err error)
	NotifyURLMitigation(ctx context.Context, action MitigationAction, elementID string, reason MitigationReason, info URLInfos) (err error)
}

type MitigationEvent struct {
	Action   MitigationAction   `json:"type" validate:"required,oneof=quarantine block remove log"`
	InfoType MitigationInfoType `json:"info_type"`
	Time     int64              `json:"time"`

	ElementID string           `json:"element_id" validate:"required"`
	Reason    MitigationReason `json:"reason"`
	Info      any              `json:"info"`
}

// Define mitigation action
type MitigationAction string

const (
	// threat has been stored in quarantine
	ActionQuarantine MitigationAction = "quarantine"
	// threat has been blocked without quarantine
	ActionBlock MitigationAction = "block"
	// threat has been removed without quarantine
	ActionRemove MitigationAction = "remove"
	// a log message has been produced to warn about threat
	ActionLog MitigationAction = "log"
)

// Define what triggered the mitigation
type MitigationReason string

const (
	// Used if malware triggers mitigation
	ReasonMalware MitigationReason = "malware"
	// Used if phishing triggers mitigation
	ReasonPhishing MitigationReason = "phishing"
	// Used if analysis error triggers mitigation
	ReasonError MitigationReason = "error"
	// Used if inability to analyze triggers mitigation
	ReasonInvalid MitigationReason = "invalid"
	// Used if inability to analyze due to too big size triggers mitigation
	ReasonTooBig MitigationReason = "toobig"
	// Used if file's type triggers mitigation
	ReasonFileType MitigationReason = "filetype"
	// Used if file's path triggers mitigation
	ReasonFilePath MitigationReason = "filepath"
)

type MitigationInfoType string

const (
	InfoTypeFile  MitigationInfoType = "file"
	InfoTypeEmail MitigationInfoType = "email"
	InfoTypeURL   MitigationInfoType = "url"
)

type CommonDetails struct {
	Malwares           []string `json:"malwares"`
	GmalwareURLs       []string `json:"gmalware_urls"` // ex: expert analysis url
	QuarantineLocation string   `json:"quarantine_location"`
}

type FileInfos struct {
	CommonDetails
	File     string `json:"file"` // filePath + fileName
	Filetype string `json:"filetype"`
	Size     int64  `json:"size"`
}

type EmailInfos struct {
	CommonDetails
	Subject    string   `json:"subject"`
	Sender     string   `json:"sender"`
	Recipients []string `json:"recipients"`
}

type URLInfos struct {
	CommonDetails
	Method        string `json:"method"`
	URL           string `json:"url"`
	ContentLength int64  `json:"content_length"`
	ContentType   string `json:"content_type"`
}

func (h *Handler) NotifyFileMitigation(ctx context.Context, action MitigationAction, elementID string, reason MitigationReason, info FileInfos) (err error) {
	err = h.notifier.Notify(ctx, MitigationEvent{
		Action:   action,
		InfoType: InfoTypeFile,
		Time:     time.Now().Unix(),

		ElementID: elementID,
		Reason:    reason,
		Info:      info,
	})
	if err != nil {
		return
	}
	return
}

func (h *Handler) NotifyEmailMitigation(ctx context.Context, action MitigationAction, elementID string, reason MitigationReason, info EmailInfos) (err error) {
	err = h.notifier.Notify(ctx, MitigationEvent{
		Action:   action,
		InfoType: InfoTypeEmail,
		Time:     time.Now().Unix(),

		ElementID: elementID,
		Reason:    reason,
		Info:      info,
	})
	if err != nil {
		return
	}
	return
}

func (h *Handler) NotifyURLMitigation(ctx context.Context, action MitigationAction, elementID string, reason MitigationReason, info URLInfos) (err error) {
	err = h.notifier.Notify(ctx, MitigationEvent{
		Action:   action,
		InfoType: InfoTypeURL,
		Time:     time.Now().Unix(),

		ElementID: elementID,
		Reason:    reason,
		Info:      info,
	})
	if err != nil {
		return
	}
	return
}
