package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

type certificateDigestFields struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"sessionID"`
	IssuedAt       time.Time `json:"issuedAt"`
	Reviewer       string    `json:"reviewer"`
	SessionVersion uint64    `json:"sessionVersion"`
	EventHeadHash  string    `json:"eventHeadHash"`
}

func CertificateDigest(certificate ReadinessCertificate) string {
	stable := certificateDigestFields{ID: certificate.ID, SessionID: certificate.SessionID, IssuedAt: certificate.IssuedAt.UTC(), Reviewer: certificate.Reviewer, SessionVersion: certificate.SessionVersion, EventHeadHash: certificate.EventHeadHash}
	encoded, _ := json.Marshal(stable)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func VerifyCertificate(certificate ReadinessCertificate) bool {
	return certificate.Digest != "" && certificate.Digest == CertificateDigest(certificate)
}

func (a *Aggregate) IssueCertificate(id, eventHeadHash string, now time.Time) (Event, error) {
	if err := a.ensureStatus(SessionReview); err != nil {
		return Event{}, err
	}
	id = strings.TrimSpace(id)
	eventHeadHash = strings.TrimSpace(eventHeadHash)
	if id == "" || eventHeadHash == "" {
		return Event{}, ruleError("INVALID_CERTIFICATE", "证书 ID 和事件链头哈希不能为空")
	}
	if len(a.FailedCueIDs()) > 0 {
		return Event{}, ruleError("FAILED_CUES_REMAIN", "仍有动作未通过，不能签发证书")
	}
	reviewer, approved := a.LatestApprovedReviewer()
	if !approved {
		return Event{}, ruleError("APPROVAL_REQUIRED", "证书签发前必须完成通过复核")
	}
	if a.Certificate != nil {
		return Event{}, ruleError("CERTIFICATE_EXISTS", "演出就绪证书已经签发")
	}
	certificate := ReadinessCertificate{ID: id, SessionID: a.Session.ID, IssuedAt: now.UTC(), Reviewer: reviewer, SessionVersion: a.Session.Version + 1, EventHeadHash: eventHeadHash}
	certificate.Digest = CertificateDigest(certificate)
	return a.emit(EventCertificateIssued, now, CertificateIssued{Certificate: certificate})
}
