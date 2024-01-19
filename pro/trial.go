//go:build ee
// +build ee

package pro

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/exp/slog"
)

type TrialInfo struct {
	PrivKey []byte `json:"priv_key"`
	PubKey  []byte `json:"pub_key"`
	Secret  []byte `json:"secret"`
}

func addTrialLicenseHook() {
	logic.HookManagerCh <- models.HookDetails{
		Hook:     TrialLicenseHook,
		Interval: time.Hour,
	}
}

type TrialDates struct {
	TrialStartedAt time.Time `json:"trial_started_at"`
	TrialEndsAt    time.Time `json:"trial_ends_at"`
}

const trial_table_name = "trial"

const trial_data_key = "trialdata"

// store trial date
func initTrial() error {
	telData, err := logic.FetchTelemetryData()
	if err != nil {
		return err
	}
	if telData.Hosts > 0 || telData.Networks > 0 || telData.Users > 0 {
		return nil
	}
	err = database.CreateTable(trial_table_name)
	if err != nil {
		slog.Error("failed to create table", "table name", trial_table_name, "err", err.Error())
		return err
	}
	// setup encryption keys
	trafficPubKey, trafficPrivKey, err := box.GenerateKey(rand.Reader) // generate traffic keys
	if err != nil {
		return err
	}
	tPriv, err := ncutils.ConvertKeyToBytes(trafficPrivKey)
	if err != nil {
		return err
	}

	tPub, err := ncutils.ConvertKeyToBytes(trafficPubKey)
	if err != nil {
		return err
	}
	trialDates := TrialDates{
		TrialStartedAt: time.Now(),
		TrialEndsAt:    time.Now().Add(time.Hour * 24 * 30),
	}
	t := TrialInfo{
		PrivKey: tPriv,
		PubKey:  tPub,
	}
	tel, err := logic.FetchTelemetryRecord()
	if err != nil {
		return err
	}

	trialDatesData, err := json.Marshal(trialDates)
	if err != nil {
		return err
	}
	telePubKey, err := ncutils.ConvertBytesToKey(tel.TrafficKeyPub)
	if err != nil {
		return err
	}
	trialDatesSecret, err := ncutils.BoxEncrypt(trialDatesData, telePubKey, trafficPrivKey)
	if err != nil {
		return err
	}
	t.Secret = trialDatesSecret
	trialData, err := json.Marshal(t)
	if err != nil {
		return err
	}
	err = database.Insert(trial_data_key, string(trialData), trial_table_name)
	if err != nil {
		return err
	}
	return nil
}

func TrialLicenseHook() error {
	endDate, err := getTrialEndDate()
	if err != nil {
		logger.FatalLog0("failed to trial end date", err.Error())
	}
	if time.Now().After(endDate) {
		logger.FatalLog0("***IMPORTANT: Your Trial Has Ended, to continue using pro version, please visit https://app.netmaker.io/ and create on-prem tenant to obtain a license***\nIf you wish to downgrade to community version, please run this command `/root/nm-quick.sh -d`")

	}
	return nil
}

// get trial date
func getTrialEndDate() (time.Time, error) {
	record, err := database.FetchRecord(trial_table_name, trial_data_key)
	if err != nil {
		return time.Time{}, err
	}
	var trialInfo TrialInfo
	err = json.Unmarshal([]byte(record), &trialInfo)
	if err != nil {
		return time.Time{}, err
	}
	tel, err := logic.FetchTelemetryRecord()
	if err != nil {
		return time.Time{}, err
	}
	telePrivKey, err := ncutils.ConvertBytesToKey(tel.TrafficKeyPriv)
	if err != nil {
		return time.Time{}, err
	}
	trialPubKey, err := ncutils.ConvertBytesToKey(trialInfo.PubKey)
	if err != nil {
		return time.Time{}, err
	}
	// decrypt secret
	secretDecrypt, err := ncutils.BoxDecrypt(trialInfo.Secret, trialPubKey, telePrivKey)
	if err != nil {
		return time.Time{}, err
	}
	trialDates := TrialDates{}
	err = json.Unmarshal(secretDecrypt, &trialDates)
	if err != nil {
		return time.Time{}, err
	}
	if trialDates.TrialEndsAt.IsZero() {
		return time.Time{}, errors.New("invalid date")
	}
	return trialDates.TrialEndsAt, nil

}
