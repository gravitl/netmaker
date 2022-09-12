package ee

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
)

// AddLicenseHooks - adds the validation and cache clear hooks
func AddLicenseHooks() {
	logic.AddHook(ValidateLicense)
	logic.AddHook(ClearLicenseCache)
}

// ValidateLicense - the initial license check for netmaker server
// checks if a license is valid + limits are not exceeded
// if license is free_tier and limits exceeds, then server should terminate
// if license is not valid, server should terminate
func ValidateLicense() error {
	licenseKeyValue := servercfg.GetLicenseKey()
	netmakerAccountID := servercfg.GetNetmakerAccountID()
	logger.Log(0, "proceeding with Netmaker license validation...")
	if len(licenseKeyValue) == 0 || len(netmakerAccountID) == 0 {
		logger.FatalLog(errValidation.Error())
	}

	apiPublicKey, err := getLicensePublicKey(licenseKeyValue)
	if err != nil {
		logger.FatalLog(errValidation.Error())
	}

	tempPubKey, tempPrivKey, err := pro.FetchApiServerKeys()
	if err != nil {
		logger.FatalLog(errValidation.Error())
	}

	licenseSecret := LicenseSecret{
		UserID: netmakerAccountID,
		Limits: getCurrentServerLimit(),
	}

	secretData, err := json.Marshal(&licenseSecret)
	if err != nil {
		logger.FatalLog(errValidation.Error())
	}

	encryptedData, err := ncutils.BoxEncrypt(secretData, apiPublicKey, tempPrivKey)
	if err != nil {
		logger.FatalLog(errValidation.Error())
	}

	validationResponse, err := validateLicenseKey(encryptedData, tempPubKey)
	if err != nil || len(validationResponse) == 0 {
		logger.FatalLog(errValidation.Error())
	}

	var licenseResponse ValidatedLicense
	if err = json.Unmarshal(validationResponse, &licenseResponse); err != nil {
		logger.FatalLog(errValidation.Error())
	}

	respData, err := ncutils.BoxDecrypt(base64decode(licenseResponse.EncryptedLicense), apiPublicKey, tempPrivKey)
	if err != nil {
		logger.FatalLog(errValidation.Error())
	}

	license := LicenseKey{}
	if err = json.Unmarshal(respData, &license); err != nil {
		logger.FatalLog(errValidation.Error())
	}

	Limits.Networks = math.MaxInt
	Limits.FreeTier = license.FreeTier == "yes"
	Limits.Clients = license.LimitClients
	Limits.Nodes = license.LimitNodes
	Limits.Servers = license.LimitServers
	Limits.Users = license.LimitUsers
	if Limits.FreeTier {
		Limits.Networks = 3
	}

	logger.Log(0, "License validation succeeded!")
	return nil
}

func getLicensePublicKey(licensePubKeyEncoded string) (*[32]byte, error) {
	decodedPubKey := base64decode(licensePubKeyEncoded)
	return ncutils.ConvertBytesToKey(decodedPubKey)
}

func validateLicenseKey(encryptedData []byte, publicKey *[32]byte) ([]byte, error) {

	publicKeyBytes, err := ncutils.ConvertKeyToBytes(publicKey)
	if err != nil {
		return nil, err
	}

	msg := ValidateLicenseRequest{
		NmServerPubKey: base64encode(publicKeyBytes),
		EncryptedPart:  base64encode(encryptedData),
	}

	requestBody, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, api_endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}
	reqParams := req.URL.Query()
	reqParams.Add("licensevalue", servercfg.GetLicenseKey())
	req.URL.RawQuery = reqParams.Encode()
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	client := &http.Client{}
	var body []byte
	validateResponse, err := client.Do(req)
	if err != nil { // check cache
		body, err = getCachedResponse()
		if err != nil {
			return nil, err
		}
		logger.Log(3, "proceeding with cached response, Netmaker API may be down")
	} else {
		defer validateResponse.Body.Close()
		if validateResponse.StatusCode != 200 {
			return nil, fmt.Errorf("could not validate license")
		} // if you received a 200 cache the response locally

		body, err = ioutil.ReadAll(validateResponse.Body)
		if err != nil {
			return nil, err
		}
		cacheResponse(body)
	}

	return body, err
}

func cacheResponse(response []byte) error {
	var lrc = licenseResponseCache{
		Body: response,
	}

	record, err := json.Marshal(&lrc)
	if err != nil {
		return err
	}

	return database.Insert(license_cache_key, string(record), database.CACHE_TABLE_NAME)
}

func getCachedResponse() ([]byte, error) {
	var lrc licenseResponseCache
	record, err := database.FetchRecord(database.CACHE_TABLE_NAME, license_cache_key)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(record), &lrc); err != nil {
		return nil, err
	}
	return lrc.Body, nil
}

// ClearLicenseCache - clears the cached validate response
func ClearLicenseCache() error {
	return database.DeleteRecord(database.CACHE_TABLE_NAME, license_cache_key)
}

// AddServerIDIfNotPresent - add's current server ID to DB if not present
func AddServerIDIfNotPresent() error {
	currentNodeID := servercfg.GetNodeID()
	currentServerIDs := serverIDs{}

	record, err := database.FetchRecord(database.SERVERCONF_TABLE_NAME, server_id_key)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	} else if err == nil {
		if err = json.Unmarshal([]byte(record), &currentServerIDs); err != nil {
			return err
		}
	}

	if !logic.StringSliceContains(currentServerIDs.ServerIDs, currentNodeID) {
		currentServerIDs.ServerIDs = append(currentServerIDs.ServerIDs, currentNodeID)
		data, err := json.Marshal(&currentServerIDs)
		if err != nil {
			return err
		}
		return database.Insert(server_id_key, string(data), database.SERVERCONF_TABLE_NAME)
	}

	return nil
}

func getServerCount() int {
	if record, err := database.FetchRecord(database.SERVERCONF_TABLE_NAME, server_id_key); err == nil {
		currentServerIDs := serverIDs{}
		if err = json.Unmarshal([]byte(record), &currentServerIDs); err == nil {
			return len(currentServerIDs.ServerIDs)
		}
	}
	return 1
}
